package discord

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gdrive-downloader/pkg/chunked"
	"gdrive-downloader/pkg/logger"
)

type ProgressCallback = chunked.ProgressCallback

type progressReader struct {
	ctx            context.Context
	reader         io.Reader
	totalBytes     int64
	downloaded     int64
	lastDownloaded int64
	lastReport     time.Time
	onProgress     ProgressCallback
	startEpoch     uint64
	epochProvider  func() uint64
}

var errProxyRotated = fmt.Errorf("proxy_rotated")

func (pr *progressReader) Read(p []byte) (int, error) {
	if err := pr.ctx.Err(); err != nil {
		return 0, err
	}
	if pr.startEpoch > 0 && pr.epochProvider != nil && pr.epochProvider() != pr.startEpoch {
		return 0, errProxyRotated
	}

	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.downloaded += int64(n)
		now := time.Now()
		elapsed := now.Sub(pr.lastReport).Seconds()

		if elapsed >= 0.15 {
			speed := int64(float64(pr.downloaded-pr.lastDownloaded) / elapsed)
			pr.lastDownloaded = pr.downloaded
			pr.lastReport = now

			var percentage float64
			var eta int64
			if pr.totalBytes > 0 {
				percentage = float64(pr.downloaded) / float64(pr.totalBytes) * 100
				if speed > 0 {
					eta = (pr.totalBytes - pr.downloaded) / speed
				}
			}

			if pr.onProgress != nil {
				pr.onProgress(pr.downloaded, pr.totalBytes, speed, eta, percentage)
			}
		}
	}

	return n, err
}

type Downloader struct {
	client            *http.Client
	transport         *http.Transport
	chunkedDownloader *chunked.Downloader
	chunksPerDownload int
	epochProvider     func() uint64
	onRateLimit       func(reason string)
	mu                sync.RWMutex
}

func NewDownloader() *Downloader {
	transport := &http.Transport{
		// Enforce HTTP/1.1 to prevent Cloudflare/Discord HTTP/2 RST_STREAM INTERNAL_ERROR
		TLSNextProto:        make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
		DisableCompression: true,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		ReadBufferSize:      1024 * 1024, // 1MB buffer for gigabit / fast throughput
		WriteBufferSize:     1024 * 1024,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   0,
	}

	return &Downloader{
		client:            client,
		transport:         transport,
		chunkedDownloader: chunked.NewDownloader(client),
		chunksPerDownload: 4,
	}
}

// BindProxyController connects this downloader and its chunked downloader to the WARP/Proxy controller.
func (d *Downloader) BindProxyController(registerTransport func(*http.Transport), epochProvider func() uint64, onRateLimit func(string)) {
	d.mu.Lock()
	d.epochProvider = epochProvider
	d.onRateLimit = onRateLimit
	d.mu.Unlock()

	if registerTransport != nil {
		registerTransport(d.transport)
		registerTransport(d.chunkedDownloader.Transport())
	}
	d.chunkedDownloader.SetEpochProvider(epochProvider)
	d.chunkedDownloader.SetRateLimitCallback(onRateLimit)
}

func (d *Downloader) getEpoch() uint64 {
	d.mu.RLock()
	fn := d.epochProvider
	d.mu.RUnlock()
	if fn != nil {
		return fn()
	}
	return 0
}

func (d *Downloader) notifyRateLimit(reason string) {
	d.mu.RLock()
	fn := d.onRateLimit
	d.mu.RUnlock()
	if fn != nil {
		go fn(reason)
	}
}

// SetChunksPerDownload configures parallel download streams for large Discord attachments.
func (d *Downloader) SetChunksPerDownload(n int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if n < 1 {
		n = 1
	}
	if n > 16 {
		n = 16
	}
	d.chunksPerDownload = n
}

// GetChunksPerDownload returns the configured parallel streams for Discord downloads.
func (d *Downloader) GetChunksPerDownload() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.chunksPerDownload <= 0 {
		return 4
	}
	return d.chunksPerDownload
}

// IsDiscordURL checks whether the given URL is a Discord CDN attachment URL
func IsDiscordURL(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	h := strings.ToLower(u.Host)
	return h == "cdn.discordapp.com" || h == "media.discordapp.net"
}

// ExtractDiscordFileInfo extracts the clean filename, expiry time, and whether it's expired
func ExtractDiscordFileInfo(rawURL string) (filename string, expiry time.Time, isExpired bool, err error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", time.Time{}, false, fmt.Errorf("invalid URL: %w", err)
	}

	// Discord attachment path format: /attachments/<channel_id>/<attachment_id>/<filename>
	p := u.Path
	filename = path.Base(p)
	if unescaped, err := url.PathUnescape(filename); err == nil && unescaped != "" {
		filename = unescaped
	}

	// Check expiration parameter `ex`
	q := u.Query()
	exHex := q.Get("ex")
	if exHex != "" {
		sec, parseErr := strconv.ParseInt(exHex, 16, 64)
		if parseErr == nil && sec > 0 {
			expiry = time.Unix(sec, 0)
			if time.Now().After(expiry) {
				isExpired = true
			}
		}
	}

	return filename, expiry, isExpired, nil
}

// ExtractDiscordIDs extracts the channel ID, attachment ID, and clean filename from a Discord CDN URL.
func ExtractDiscordIDs(rawURL string) (channelID string, attachmentID string, filename string, err error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", "", "", fmt.Errorf("invalid URL: %w", err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// Format is attachments/<channel_id>/<attachment_id>/<filename>
	for i, part := range parts {
		if part == "attachments" && i+2 < len(parts) {
			channelID = parts[i+1]
			attachmentID = parts[i+2]
			if i+3 < len(parts) {
				if dec, err := url.PathUnescape(parts[i+3]); err == nil && dec != "" {
					filename = dec
				} else {
					filename = parts[i+3]
				}
			}
			return channelID, attachmentID, filename, nil
		}
	}
	fn := path.Base(u.Path)
	if dec, err := url.PathUnescape(fn); err == nil && dec != "" {
		filename = dec
	} else {
		filename = fn
	}
	return "", "", filename, nil
}

func applyBrowserHeaders(req *http.Request) {
	applyBrowserHeadersToHeader(req.Header)
}

func applyBrowserHeadersToHeader(h http.Header) {
	h.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	h.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	h.Set("Accept-Language", "en-US,en;q=0.9")
	h.Set("Referer", "https://discord.com/")
	h.Set("Sec-Ch-Ua", `"Chromium";v="130", "Google Chrome";v="130", "Not?A_Brand";v="99"`)
	h.Set("Sec-Ch-Ua-Mobile", "?0")
	h.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	h.Set("Sec-Fetch-Dest", "document")
	h.Set("Sec-Fetch-Mode", "navigate")
	h.Set("Sec-Fetch-Site", "cross-site")
	h.Set("Sec-Fetch-User", "?1")
	h.Set("Upgrade-Insecure-Requests", "1")
}

// GetFileInfo inspects the file using a HEAD request
func (d *Downloader) GetFileInfo(ctx context.Context, rawURL string) (string, int64, error) {
	cleanName, expiry, isExpired, err := ExtractDiscordFileInfo(rawURL)
	if err != nil {
		return "", 0, err
	}
	if isExpired {
		return cleanName, 0, fmt.Errorf("Discord CDN attachment link has expired (expired at %s). Please copy a fresh link from Discord", expiry.Format("2006-01-02 15:04:05"))
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", rawURL, nil)
	if err != nil {
		return cleanName, 0, err
	}
	applyBrowserHeaders(req)

	resp, err := d.client.Do(req)
	if err != nil {
		return cleanName, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return cleanName, 0, fmt.Errorf("rate limited by Discord CDN (HTTP 429)")
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return cleanName, 0, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
	}

	size := resp.ContentLength

	// Check Content-Disposition for potential better name
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if fn := params["filename*"]; fn != "" {
				// Format might be UTF-8''encoded_name
				if strings.HasPrefix(strings.ToLower(fn), "utf-8''") {
					encoded := fn[7:]
					if dec, err := url.PathUnescape(encoded); err == nil && dec != "" {
						cleanName = dec
					}
				} else {
					cleanName = fn
				}
			} else if fn := params["filename"]; fn != "" {
				cleanName = fn
			}
		}
	}

	return cleanName, size, nil
}

// Download downloads a Discord CDN attachment URL with HTTP Range resumption and exponential backoff retry for HTTP 429
func (d *Downloader) Download(
	ctx context.Context,
	rawURL string,
	targetFolder string,
	onProgress ProgressCallback,
	desiredFilename ...string,
) (filename string, written int64, err error) {
	parsedName, expiry, isExpired, err := ExtractDiscordFileInfo(rawURL)
	if err != nil {
		return "", 0, err
	}
	if isExpired {
		return parsedName, 0, fmt.Errorf("Discord CDN attachment link has expired (expired at %s). Please generate or copy a fresh link from Discord", expiry.Format("2006-01-02 15:04:05"))
	}

	filename = parsedName
	if len(desiredFilename) > 0 && desiredFilename[0] != "" {
		filename = desiredFilename[0]
	}

	if err := os.MkdirAll(targetFolder, 0755); err != nil {
		return filename, 0, fmt.Errorf("failed to create target directory: %w", err)
	}

	destPath := filepath.Join(targetFolder, filename)
	partPath := destPath + ".part"

	// Fast probe for remote size and accurate Content-Disposition filename
	remoteName, remoteSize, headErr := d.GetFileInfo(ctx, rawURL)
	if headErr == nil && remoteName != "" && (len(desiredFilename) == 0 || desiredFilename[0] == "") {
		filename = remoteName
		destPath = filepath.Join(targetFolder, filename)
		partPath = destPath + ".part"
	}

	// 1. Check if destination file already exists and is fully downloaded
	if remoteSize > 0 {
		if fi, err := os.Stat(destPath); err == nil && fi.Size() == remoteSize {
			logger.Infof("Discord", "File '%s' already fully downloaded (%d bytes)", filename, remoteSize)
			if onProgress != nil {
				onProgress(remoteSize, remoteSize, 0, 0, 100)
			}
			return filename, remoteSize, nil
		}
		if fi, err := os.Stat(partPath); err == nil && fi.Size() == remoteSize {
			logger.Infof("Discord", "File '%s' .part is already complete (%d bytes), finalizing...", filename, remoteSize)
			if err := finalizeDownloadedFile(partPath, destPath, remoteSize); err == nil {
				if onProgress != nil {
					onProgress(remoteSize, remoteSize, 0, 0, 100)
				}
				return filename, remoteSize, nil
			}
		}
	}

	chunks := d.GetChunksPerDownload()
	useChunks := chunks > 1 && remoteSize >= 10*1024*1024

	// 2. IDM-style parallel multi-socket download for large attachments
	if useChunks {
		logger.Infof("Discord", "Downloading '%s' (%d bytes) with %d parallel HTTP/1.1 chunk streams (IDM-style)", filename, remoteSize, chunks)
		headers := http.Header{}
		applyBrowserHeadersToHeader(headers)

		copied, dlErr := d.chunkedDownloader.DownloadSegmented(
			ctx,
			partPath,
			rawURL,
			remoteSize,
			headers,
			chunks,
			onProgress,
		)

		if dlErr == nil {
			if err := finalizeDownloadedFile(partPath, destPath, remoteSize); err != nil {
				return filename, copied, fmt.Errorf("failed to finalize downloaded file: %w", err)
			}
			if onProgress != nil {
				onProgress(remoteSize, remoteSize, 0, 0, 100)
			}
			logger.Successf("Discord", "Saved '%s' successfully (%d bytes) via %d streams", filename, remoteSize, chunks)
			return filename, remoteSize, nil
		}

		if ctx.Err() != nil || strings.Contains(dlErr.Error(), "context canceled") {
			return filename, copied, ctx.Err()
		}

		if strings.Contains(dlErr.Error(), "expired") || strings.Contains(dlErr.Error(), "HTTP 401") || strings.Contains(dlErr.Error(), "HTTP 403") {
			return filename, copied, dlErr
		}

		logger.Warnf("Discord", "Chunked download for '%s' encountered error: %v. Falling back to single-stream download with resume...", filename, dlErr)
		chunked.RemoveChunkMeta(partPath)
	}

	var existingBytes int64 = 0
	if fi, err := os.Stat(partPath); err == nil {
		existingBytes = fi.Size()
	}

	// Retry loop for rate limits or temporary connection hiccups
	maxAttempts := 5
	backoff := 2 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return filename, existingBytes, ctx.Err()
		}

		req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
		if err != nil {
			return filename, existingBytes, err
		}
		applyBrowserHeaders(req)

		if existingBytes > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingBytes))
		}

		// Random jitter delay between 100ms - 300ms before connecting
		time.Sleep(time.Duration(100+rand.Intn(200)) * time.Millisecond)

		resp, err := d.client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return filename, existingBytes, ctx.Err()
			}
			logger.Warnf("Discord", "Attempt %d/%d request failed for '%s': %v. Retrying in %v...", attempt, maxAttempts, filename, err, backoff)
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		// Check rate limit HTTP 429
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			d.notifyRateLimit(fmt.Sprintf("Discord CDN HTTP 429 for '%s'", filename))
			retryAfterSec := 5
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if s, err := strconv.Atoi(ra); err == nil && s > 0 {
					retryAfterSec = s
				}
			}
			waitDuration := time.Duration(retryAfterSec)*time.Second + time.Duration(rand.Intn(1000))*time.Millisecond
			logger.Warnf("Discord", "Rate limit HTTP 429 encountered for '%s'. Waiting %v before retry %d/%d...", filename, waitDuration, attempt, maxAttempts)
			
			select {
			case <-ctx.Done():
				return filename, existingBytes, ctx.Err()
			case <-time.After(waitDuration):
			}
			continue
		}

		// Check if expired during attempt
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()
			return filename, existingBytes, fmt.Errorf("access denied or Discord attachment signature expired (HTTP %d)", resp.StatusCode)
		}

		// Handle HTTP 416: Requested Range Not Satisfiable
		if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			resp.Body.Close()

			contentRange := resp.Header.Get("Content-Range")
			var remoteTotal int64 = -1
			if idx := strings.LastIndex(contentRange, "/"); idx != -1 {
				if val, parseErr := strconv.ParseInt(strings.TrimSpace(contentRange[idx+1:]), 10, 64); parseErr == nil {
					remoteTotal = val
				}
			}

			// If local .part is exactly equal to remoteTotal and remoteTotal > 0, the file was already fully downloaded
			if remoteTotal > 0 && existingBytes == remoteTotal {
				logger.Infof("Discord", "File '%s' already fully downloaded (%d bytes), finalizing...", filename, existingBytes)
				if err := finalizeDownloadedFile(partPath, destPath, remoteTotal); err != nil {
					return filename, existingBytes, fmt.Errorf("failed to finalize downloaded file: %w", err)
				}
				if onProgress != nil {
					onProgress(remoteTotal, remoteTotal, 0, 0, 100)
				}
				return filename, remoteTotal, nil
			}

			// Local part is corrupted or out of range. Delete and restart from byte 0.
			logger.Warnf("Discord", "HTTP 416 Range mismatch for '%s' (local .part size: %d, remote: %d). Resetting invalid .part and restarting from 0...", filename, existingBytes, remoteTotal)
			_ = os.Remove(partPath)
			existingBytes = 0
			time.Sleep(1 * time.Second)
			continue
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			if resp.StatusCode >= 500 && attempt < maxAttempts {
				logger.Warnf("Discord", "HTTP %d from Discord CDN for '%s'. Retrying in %v...", resp.StatusCode, filename, backoff)
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return filename, existingBytes, fmt.Errorf("unexpected status %s (HTTP %d)", resp.Status, resp.StatusCode)
		}

		// Determine total size
		var out *os.File
		var totalBytes int64

		if resp.StatusCode == http.StatusPartialContent && existingBytes > 0 {
			totalBytes = existingBytes + resp.ContentLength
		} else {
			existingBytes = 0
			totalBytes = resp.ContentLength
		}

		if resp.StatusCode == http.StatusPartialContent && existingBytes > 0 {
			out, err = os.OpenFile(partPath, os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				resp.Body.Close()
				return filename, existingBytes, fmt.Errorf("failed to open part file for append: %w", err)
			}
		} else {
			out, err = os.Create(partPath)
			if err != nil {
				resp.Body.Close()
				return filename, 0, fmt.Errorf("failed to create part file: %w", err)
			}
		}

		pr := &progressReader{
			ctx:            ctx,
			reader:         resp.Body,
			totalBytes:     totalBytes,
			downloaded:     existingBytes,
			lastDownloaded: existingBytes,
			lastReport:     time.Now(),
			onProgress:     onProgress,
			startEpoch:     d.getEpoch(),
			epochProvider:  d.getEpoch,
		}

		buf := make([]byte, 1024*1024)
		written, copyErr := io.CopyBuffer(out, pr, buf)
		out.Close()
		resp.Body.Close()

		if copyErr != nil {
			if fi, statErr := os.Stat(partPath); statErr == nil {
				existingBytes = fi.Size()
			}
			if ctx.Err() != nil {
				return filename, existingBytes, ctx.Err()
			}
			if copyErr == errProxyRotated {
				logger.Infof("Discord", "Switching download '%s' to newly rotated WARP/Proxy IP at byte %d...", filename, existingBytes)
				attempt--
				backoff = time.Second
				continue
			}
			logger.Warnf("Discord", "Network error during transfer of '%s': %v. Retrying in %v...", filename, copyErr, backoff)
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		// Download successfully finished!
		if err := finalizeDownloadedFile(partPath, destPath, totalBytes); err != nil {
			return filename, existingBytes + written, fmt.Errorf("failed to finalize downloaded file: %w", err)
		}

		if onProgress != nil {
			onProgress(totalBytes, totalBytes, 0, 0, 100)
		}

		return filename, totalBytes, nil
	}

	return filename, existingBytes, fmt.Errorf("failed to download from Discord CDN after %d attempts", maxAttempts)
}

// finalizeDownloadedFile safely moves the .part file to destPath, handling existing destinations,
// already finalized files, and cross-filesystem copy fallbacks (e.g. Docker/Linux mount points).
func finalizeDownloadedFile(partPath, destPath string, expectedSize int64) error {
	// If partPath does not exist, check if destPath already exists with expected size
	if _, err := os.Stat(partPath); os.IsNotExist(err) {
		if destFi, statErr := os.Stat(destPath); statErr == nil && (expectedSize <= 0 || destFi.Size() == expectedSize) {
			return nil
		}
		return fmt.Errorf("part file missing and destination file not finalized: %w", err)
	}

	// First attempt: direct rename
	if err := os.Rename(partPath, destPath); err == nil {
		return nil
	}

	// Destination might exist (especially on Windows) - remove and retry rename
	_ = os.Remove(destPath)
	if err := os.Rename(partPath, destPath); err == nil {
		return nil
	}

	// Fallback for cross-device links (EXDEV) or filesystem mount issues (e.g. /mnt/hdd-backup): copy + remove
	in, err := os.Open(partPath)
	if err != nil {
		return fmt.Errorf("failed to open part file for fallback copy: %w", err)
	}
	defer in.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file for fallback copy: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		_ = os.Remove(destPath)
		return fmt.Errorf("failed during fallback file copy: %w", err)
	}

	_ = in.Close()
	_ = out.Close()
	_ = os.Remove(partPath)
	return nil
}
