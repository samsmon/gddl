package discord

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gdrive-downloader/pkg/chunked"
	"gdrive-downloader/pkg/logger"
)

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
		TLSNextProto:        make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
		DisableCompression: true,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		ReadBufferSize:      1024 * 1024,
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

	remoteName, remoteSize, headErr := d.GetFileInfo(ctx, rawURL)
	if headErr == nil && remoteName != "" && (len(desiredFilename) == 0 || desiredFilename[0] == "") {
		filename = remoteName
		destPath = filepath.Join(targetFolder, filename)
		partPath = destPath + ".part"
	}

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

	if chunks := d.GetChunksPerDownload(); chunks > 1 && remoteSize >= 10*1024*1024 {
		logger.Infof("Discord", "Downloading '%s' (%d bytes) with %d parallel HTTP/1.1 chunk streams (IDM-style)", filename, remoteSize, chunks)
		headers := http.Header{}
		applyBrowserHeadersToHeader(headers)
		copied, dlErr := d.chunkedDownloader.DownloadSegmented(ctx, partPath, rawURL, remoteSize, headers, chunks, onProgress)
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

	var existingBytes int64
	if fi, err := os.Stat(partPath); err == nil {
		existingBytes = fi.Size()
	}

	return d.downloadSingleStream(ctx, rawURL, filename, partPath, destPath, existingBytes, onProgress)
}
