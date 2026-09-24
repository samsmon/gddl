package discord

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gdrive-downloader/pkg/logger"
)

// downloadSingleStream executes single-stream HTTP range download with backoff and WARP rotation recovery.
func (d *Downloader) downloadSingleStream(
	ctx context.Context,
	rawURL, filename, partPath, destPath string,
	existingBytes int64,
	onProgress ProgressCallback,
) (string, int64, error) {
	backoff := time.Second
	maxAttempts := 5

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return filename, existingBytes, ctx.Err()
		}

		req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
		if err != nil {
			return filename, existingBytes, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")

		if existingBytes > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingBytes))
		}

		resp, err := d.client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return filename, existingBytes, ctx.Err()
			}
			logger.Warnf("Discord", "Connection error for '%s': %v. Retrying in %v (attempt %d/%d)...", filename, err, backoff, attempt, maxAttempts)
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			d.notifyRateLimit("Discord CDN rate limit (HTTP 429) encountered")
			retryAfter := backoff
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if s, err := strconv.Atoi(ra); err == nil {
					retryAfter = time.Duration(s) * time.Second
				}
			}
			jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
			logger.Warnf("Discord", "HTTP 429 Too Many Requests for '%s'. Backing off for %v (attempt %d/%d)...", filename, retryAfter+jitter, attempt, maxAttempts)
			time.Sleep(retryAfter + jitter)
			backoff *= 2
			continue
		}

		if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			resp.Body.Close()
			var remoteTotal int64 = 0
			cr := resp.Header.Get("Content-Range")
			if cr != "" {
				parts := strings.Split(cr, "/")
				if len(parts) == 2 {
					if val, parseErr := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); parseErr == nil {
						remoteTotal = val
					}
				}
			}
			if remoteTotal == 0 {
				if val, parseErr := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); parseErr == nil {
					remoteTotal = val
				}
			}
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

		var out *os.File
		var totalBytes int64
		if resp.StatusCode == http.StatusPartialContent && existingBytes > 0 {
			totalBytes = existingBytes + resp.ContentLength
			out, err = os.OpenFile(partPath, os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				resp.Body.Close()
				return filename, existingBytes, fmt.Errorf("failed to open part file for append: %w", err)
			}
		} else {
			existingBytes = 0
			totalBytes = resp.ContentLength
			out, err = os.Create(partPath)
			if err != nil {
				resp.Body.Close()
				return filename, 0, fmt.Errorf("failed to create part file: %w", err)
			}
		}

		readDone := make(chan struct{})
		go func(body io.Closer) {
			select {
			case <-readDone:
			case <-ctx.Done():
				_ = body.Close()
			}
		}(resp.Body)

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
		written, copyErr := io.CopyBuffer(out, pr, make([]byte, 1024*1024))
		close(readDone)
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
