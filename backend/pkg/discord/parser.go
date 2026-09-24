package discord

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"gdrive-downloader/pkg/chunked"
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

	p := u.Path
	filename = path.Base(p)
	if unescaped, err := url.PathUnescape(filename); err == nil && unescaped != "" {
		filename = unescaped
	}

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
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if fn := params["filename*"]; fn != "" {
				if strings.HasPrefix(strings.ToLower(fn), "utf-8''") {
					if dec, err := url.PathUnescape(fn[7:]); err == nil && dec != "" {
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

// finalizeDownloadedFile safely moves the .part file to destPath.
func finalizeDownloadedFile(partPath, destPath string, expectedSize int64) error {
	if _, err := os.Stat(partPath); os.IsNotExist(err) {
		if destFi, statErr := os.Stat(destPath); statErr == nil && (expectedSize <= 0 || destFi.Size() == expectedSize) {
			return nil
		}
		return fmt.Errorf("part file missing and destination file not finalized: %w", err)
	}
	if err := os.Rename(partPath, destPath); err == nil {
		return nil
	}
	_ = os.Remove(destPath)
	if err := os.Rename(partPath, destPath); err == nil {
		return nil
	}
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
