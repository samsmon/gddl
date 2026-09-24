package chunked

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gdrive-downloader/pkg/logger"
)

// ProgressCallback reports real-time download statistics.
type ProgressCallback func(downloadedBytes, totalBytes, speed int64, etaSeconds int64, percentage float64)

// Downloader manages multi-stream segmented downloads with parallel HTTP Range requests.
type Downloader struct {
	mu            sync.RWMutex
	client        *http.Client
	transport     *http.Transport
	epochProvider func() uint64
	onRateLimit   func(reason string)
}

// NewDownloader creates a new chunked Downloader with multi-socket HTTP/1.1 transport.
func NewDownloader(baseClient *http.Client) *Downloader {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dialer.DialContext(ctx, "tcp4", addr)
			if err != nil {
				return dialer.DialContext(ctx, network, addr)
			}
			return conn, nil
		},
		TLSNextProto:          make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
		DisableCompression:   true,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ReadBufferSize:        1024 * 1024,
		WriteBufferSize:       1024 * 1024,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   0,
	}
	if baseClient != nil && baseClient.Jar != nil {
		client.Jar = baseClient.Jar
	}

	return &Downloader{
		client:    client,
		transport: transport,
	}
}

// Transport returns the underlying HTTP transport for dynamic proxy wiring.
func (d *Downloader) Transport() *http.Transport {
	return d.transport
}

// SetEpochProvider registers a function that returns the current WARP/Proxy rotation epoch.
func (d *Downloader) SetEpochProvider(fn func() uint64) {
	d.mu.Lock()
	d.epochProvider = fn
	d.mu.Unlock()
}

// SetRateLimitCallback registers a callback invoked when HTTP 429 is encountered.
func (d *Downloader) SetRateLimitCallback(fn func(reason string)) {
	d.mu.Lock()
	d.onRateLimit = fn
	d.mu.Unlock()
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

// DownloadSegmented downloads a file in parallel chunks using HTTP Range requests with
// automatic self-healing, staggered socket connection (IDM-style), and state resumption.
func (d *Downloader) DownloadSegmented(
	ctx context.Context,
	destPath string,
	downloadURL string,
	totalSize int64,
	headers http.Header,
	numChunks int,
	onProgress ProgressCallback,
) (int64, error) {
	maxHealAttempts := 3
	var lastErr error
	var finalDownloaded int64

	for healAttempt := 1; healAttempt <= maxHealAttempts; healAttempt++ {
		if ctx.Err() != nil {
			return finalDownloaded, ctx.Err()
		}

		finalDownloaded, lastErr = d.downloadSegmentedOnce(
			ctx,
			destPath,
			downloadURL,
			totalSize,
			headers,
			numChunks,
			onProgress,
		)

		if lastErr == nil {
			return finalDownloaded, nil
		}

		if ctx.Err() != nil || errors.Is(lastErr, context.Canceled) {
			return finalDownloaded, ctx.Err()
		}

		if strings.Contains(lastErr.Error(), "does not support HTTP Range requests") {
			return finalDownloaded, lastErr
		}

		if strings.Contains(lastErr.Error(), "expired") || strings.Contains(lastErr.Error(), "HTTP 401") || strings.Contains(lastErr.Error(), "HTTP 403") {
			return finalDownloaded, lastErr
		}

		if healAttempt == 1 {
			logger.Warnf("Chunked", "Segment transfer error on '%s': %v. Auto-healing (1/2): retrying incomplete chunks...", filepath.Base(destPath), lastErr)
			time.Sleep(1 * time.Second)
		} else if healAttempt == 2 {
			logger.Warnf("Chunked", "Segment transfer error persisted on '%s': %v. Auto-healing (2/2): clearing corrupt chunk state and restarting cleanly from byte 0...", filepath.Base(destPath), lastErr)
			RemoveChunkMeta(destPath)
			_ = os.Remove(destPath)
			time.Sleep(1500 * time.Millisecond)
		}
	}

	return finalDownloaded, lastErr
}

func (d *Downloader) downloadSegmentedOnce(
	ctx context.Context,
	destPath string,
	downloadURL string,
	totalSize int64,
	headers http.Header,
	numChunks int,
	onProgress ProgressCallback,
) (int64, error) {
	if numChunks <= 1 || totalSize <= 0 {
		return 0, errors.New("invalid chunk count or total size for segmented download")
	}
	if numChunks > 16 {
		numChunks = 16
	}

	segments, initialDownloaded := initOrResumeSegments(destPath, totalSize, numChunks)

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return initialDownloaded, fmt.Errorf("failed creating output file: %w", err)
	}
	defer out.Close()

	if err := out.Truncate(totalSize); err != nil {
		logger.Warnf("Chunked", "Could not pre-allocate file size for %s: %v", destPath, err)
	}

	_ = saveChunkState(destPath, totalSize, segments)

	var totalDownloaded atomic.Int64
	totalDownloaded.Store(initialDownloaded)

	var activeErr error
	var errOnce sync.Once
	setErr := func(e error) {
		errOnce.Do(func() {
			activeErr = e
		})
	}
	hasActiveErr := func() bool {
		return activeErr != nil
	}

	progressCtx, cancelProgress := context.WithCancel(ctx)
	defer cancelProgress()

	go runProgressMonitor(progressCtx, destPath, totalSize, initialDownloaded, &totalDownloaded, segments, onProgress)

	var wg sync.WaitGroup
	wg.Add(len(segments))
	for _, seg := range segments {
		go func(s *ChunkSegment) {
			defer wg.Done()
			d.downloadSegmentWorker(ctx, s, out, downloadURL, headers, &totalDownloaded, setErr, hasActiveErr)
		}(seg)
	}

	wg.Wait()
	cancelProgress()

	finalDownloaded := totalDownloaded.Load()
	if activeErr != nil {
		_ = saveChunkState(destPath, totalSize, segments)
		out.Close()
		return finalDownloaded, activeErr
	}

	_ = out.Sync()
	out.Close()
	RemoveChunkMeta(destPath)

	if onProgress != nil {
		onProgress(totalSize, totalSize, 0, 0, 100.0)
	}
	return finalDownloaded, nil
}
