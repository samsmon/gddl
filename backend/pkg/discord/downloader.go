package discord

import (
	"context"
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
	"time"

	"gdrive-downloader/pkg/logger"
)

type ProgressCallback func(downloadedBytes, totalBytes, speed int64, etaSeconds int64, percentage float64)

type progressReader struct {
	ctx            context.Context
	reader         io.Reader
	totalBytes     int64
	downloaded     int64
	lastDownloaded int64
	lastReport     time.Time
	onProgress     ProgressCallback
}

func (pr *progressReader) Read(p []byte) (int, error) {
	if err := pr.ctx.Err(); err != nil {
		return 0, err
	}

	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.downloaded += int64(n)
		now := time.Now()
		elapsed := now.Sub(pr.lastReport).Seconds()

		if elapsed >= 0.3 {
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
	client *http.Client
}

func NewDownloader() *Downloader {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		ReadBufferSize:      1024 * 1024, // 1MB buffer for gigabit / fast throughput
		WriteBufferSize:     1024 * 1024,
	}

	return &Downloader{
		client: &http.Client{
			Transport: transport,
			Timeout:   0,
		},
	}
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

func applyBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://discord.com/")
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="130", "Google Chrome";v="130", "Not?A_Brand";v="99"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
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

		// Open target file
		var out *os.File
		var totalBytes int64

		if resp.StatusCode == http.StatusPartialContent && existingBytes > 0 {
			out, err = os.OpenFile(partPath, os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				resp.Body.Close()
				return filename, existingBytes, fmt.Errorf("failed to open part file for append: %w", err)
			}
			totalBytes = existingBytes + resp.ContentLength
		} else {
			// Start fresh
			existingBytes = 0
			out, err = os.Create(partPath)
			if err != nil {
				resp.Body.Close()
				return filename, 0, fmt.Errorf("failed to create part file: %w", err)
			}
			totalBytes = resp.ContentLength
		}

		pr := &progressReader{
			ctx:            ctx,
			reader:         resp.Body,
			totalBytes:     totalBytes,
			downloaded:     existingBytes,
			lastDownloaded: existingBytes,
			lastReport:     time.Now(),
			onProgress:     onProgress,
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
			logger.Warnf("Discord", "Network error during transfer of '%s': %v. Retrying in %v...", filename, copyErr, backoff)
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		// Download successfully finished!
		if err := os.Rename(partPath, destPath); err != nil {
			// On Windows, if destination exists, remove first
			_ = os.Remove(destPath)
			if err := os.Rename(partPath, destPath); err != nil {
				return filename, existingBytes + written, fmt.Errorf("failed to finalize downloaded file: %w", err)
			}
		}

		if onProgress != nil {
			onProgress(totalBytes, totalBytes, 0, 0, 100)
		}

		return filename, totalBytes, nil
	}

	return filename, existingBytes, fmt.Errorf("failed to download from Discord CDN after %d attempts", maxAttempts)
}
