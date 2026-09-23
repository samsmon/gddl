package chunked

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"gdrive-downloader/pkg/logger"
)

// ProgressCallback reports real-time download statistics.
type ProgressCallback func(downloadedBytes, totalBytes, speed int64, etaSeconds int64, percentage float64)

// ChunkSegment represents a byte range [Start, End] of a file.
type ChunkSegment struct {
	Index   int          `json:"index"`
	Start   int64        `json:"start"`
	End     int64        `json:"end"`
	Current atomic.Int64 `json:"current"`
}

// ChunkSegmentSnapshot is used for clean JSON serialization of segment progress.
type ChunkSegmentSnapshot struct {
	Index   int   `json:"index"`
	Start   int64 `json:"start"`
	End     int64 `json:"end"`
	Current int64 `json:"current"`
}

// ChunkState represents the persistent state of a segmented download on disk.
type ChunkState struct {
	TotalSize int64                  `json:"total_size"`
	Segments  []ChunkSegmentSnapshot `json:"segments"`
}

// Downloader manages multi-stream segmented downloads with parallel HTTP Range requests.
type Downloader struct {
	client *http.Client
}

// NewDownloader creates a new chunked Downloader with multi-socket HTTP/1.1 transport.
// Forcing HTTP/1.1 avoids HTTP/2 stream multiplexing onto a single TCP socket,
// allowing each parallel chunk stream to bypass provider per-connection rate limits (e.g. 10MB/s per TCP socket).
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
		// Disabling HTTP/2 is CRITICAL for multi-chunk throughput acceleration:
		// HTTP/2 multiplexes all requests onto a single TCP socket, which causes Google/CDNs
		// to rate-limit all chunk streams collectively to ~10MB/s.
		// By forcing HTTP/1.1 with high MaxIdleConnsPerHost, each chunk gets its own independent TCP connection.
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
		client: client,
	}
}

// ChunkMetaPath returns the file path of the chunk state metadata.
func ChunkMetaPath(destPath string) string {
	return destPath + ".gddl-chunks"
}

// HasChunkMeta returns true if an active chunk state file exists for destPath.
func HasChunkMeta(destPath string) bool {
	fi, err := os.Stat(ChunkMetaPath(destPath))
	return err == nil && fi.Size() > 0
}

// RemoveChunkMeta removes any chunk state file for destPath.
func RemoveChunkMeta(destPath string) {
	_ = os.Remove(ChunkMetaPath(destPath))
}

func loadChunkState(destPath string, expectedTotalSize int64) (*ChunkState, error) {
	metaFile := ChunkMetaPath(destPath)
	data, err := os.ReadFile(metaFile)
	if err != nil {
		return nil, err
	}
	var state ChunkState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	if state.TotalSize != expectedTotalSize || len(state.Segments) == 0 {
		return nil, errors.New("mismatched chunk metadata")
	}
	return &state, nil
}

func saveChunkState(destPath string, totalSize int64, segments []*ChunkSegment) error {
	metaFile := ChunkMetaPath(destPath)
	tmpFile := metaFile + ".tmp"

	snaps := make([]ChunkSegmentSnapshot, len(segments))
	for i, s := range segments {
		snaps[i] = ChunkSegmentSnapshot{
			Index:   s.Index,
			Start:   s.Start,
			End:     s.End,
			Current: s.Current.Load(),
		}
	}

	state := ChunkState{
		TotalSize: totalSize,
		Segments:  snaps,
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	_ = os.Remove(metaFile)
	return os.Rename(tmpFile, metaFile)
}

// DownloadSegmented downloads a file in parallel chunks using HTTP Range requests.
// Supports resuming from prior .gddl-chunks state, or partitioning remaining bytes
// if destPath already contains partial contiguous data.
func (d *Downloader) DownloadSegmented(
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

	// Calculate chunk size. Minimum 1MB per chunk.
	minChunkSize := int64(1024 * 1024)
	if totalSize/int64(numChunks) < minChunkSize {
		numChunks = int(totalSize / minChunkSize)
		if numChunks < 2 {
			numChunks = 2
		}
	}

	var segments []*ChunkSegment
	var initialDownloaded int64 = 0

	// 1. Try loading existing .gddl-chunks metadata for multi-stream resume
	savedState, loadErr := loadChunkState(destPath, totalSize)
	if loadErr == nil && savedState != nil && len(savedState.Segments) > 0 {
		segments = make([]*ChunkSegment, len(savedState.Segments))
		baseOffset := savedState.Segments[0].Start
		initialDownloaded = baseOffset

		for i, snap := range savedState.Segments {
			s := &ChunkSegment{
				Index: snap.Index,
				Start: snap.Start,
				End:   snap.End,
			}
			cur := snap.Current
			if cur < snap.Start {
				cur = snap.Start
			}
			if cur > snap.End+1 {
				cur = snap.End + 1
			}
			s.Current.Store(cur)
			initialDownloaded += (cur - snap.Start)
			segments[i] = s
		}
		logger.Infof("Chunked", "Resuming parallel multi-chunk download for '%s' (%d streams, %d/%d bytes already downloaded)",
			filepath.Base(destPath), len(segments), initialDownloaded, totalSize)
	} else {
		// 2. Check if destination file has contiguous partial bytes (e.g. from prior single-stream download)
		var startOffset int64 = 0
		if fi, statErr := os.Stat(destPath); statErr == nil && fi.Size() > 0 && fi.Size() < totalSize {
			startOffset = fi.Size()
			initialDownloaded = startOffset
			logger.Infof("Chunked", "Found %d partial bytes on disk for '%s'; partitioning remaining %d bytes into %d chunks",
				startOffset, filepath.Base(destPath), totalSize-startOffset, numChunks)
		}

		remaining := totalSize - startOffset
		if remaining/int64(numChunks) < minChunkSize {
			numChunks = int(remaining / minChunkSize)
			if numChunks < 2 {
				numChunks = 2
			}
		}

		chunkSize := remaining / int64(numChunks)
		segments = make([]*ChunkSegment, numChunks)
		for i := 0; i < numChunks; i++ {
			start := startOffset + int64(i)*chunkSize
			end := startOffset + int64(i+1)*chunkSize - 1
			if i == numChunks-1 {
				end = totalSize - 1
			}
			s := &ChunkSegment{
				Index: i,
				Start: start,
				End:   end,
			}
			s.Current.Store(start)
			segments[i] = s
		}
	}

	// Open or create destination file
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return initialDownloaded, fmt.Errorf("failed creating output file: %w", err)
	}
	defer out.Close()

	// Pre-allocate / extend file size on disk for instant concurrent WriteAt
	if err := out.Truncate(totalSize); err != nil {
		logger.Warnf("Chunked", "Could not pre-allocate file size for %s: %v", destPath, err)
	}

	// Save initial chunk state
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

	// Progress monitor loop
	progressCtx, cancelProgress := context.WithCancel(ctx)
	defer cancelProgress()

	go func() {
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()

		var lastDownloaded = initialDownloaded
		var lastTime = time.Now()
		var lastSave = time.Now()

		for {
			select {
			case <-progressCtx.Done():
				return
			case now := <-ticker.C:
				current := totalDownloaded.Load()
				elapsed := now.Sub(lastTime).Seconds()
				if elapsed <= 0 {
					elapsed = 0.15
				}

				speed := int64(float64(current-lastDownloaded) / elapsed)
				if speed < 0 {
					speed = 0
				}
				lastDownloaded = current
				lastTime = now

				var eta int64 = 0
				if speed > 0 && totalSize > current {
					eta = (totalSize - current) / speed
				}

				percentage := float64(current) / float64(totalSize) * 100.0
				if percentage > 100.0 {
					percentage = 100.0
				}

				if onProgress != nil {
					onProgress(current, totalSize, speed, eta, percentage)
				}

				// Periodically persist chunk state every 1 second
				if now.Sub(lastSave) >= time.Second {
					_ = saveChunkState(destPath, totalSize, segments)
					lastSave = now
				}
			}
		}
	}()

	var wg sync.WaitGroup
	wg.Add(len(segments))

	for _, seg := range segments {
		go func(s *ChunkSegment) {
			defer wg.Done()

			maxRetries := 5
			backoff := time.Second

			for attempt := 1; attempt <= maxRetries; attempt++ {
				select {
				case <-ctx.Done():
					setErr(ctx.Err())
					return
				default:
				}

				currentPos := s.Current.Load()
				if currentPos > s.End {
					// Segment already 100% complete
					return
				}

				req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
				if err != nil {
					setErr(err)
					return
				}

				// Copy headers (Auth, Cookies, User-Agent, Referer)
				for k, v := range headers {
					for _, val := range v {
						req.Header.Add(k, val)
					}
				}
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", currentPos, s.End))

				resp, err := d.client.Do(req)
				if err != nil {
					if errors.Is(ctx.Err(), context.Canceled) {
						setErr(ctx.Err())
						return
					}
					if attempt == maxRetries {
						setErr(fmt.Errorf("chunk %d failed after %d attempts: %w", s.Index, maxRetries, err))
						return
					}
					time.Sleep(backoff)
					backoff *= 2
					continue
				}

				// Handle HTTP 429 Rate Limit with backoff
				if resp.StatusCode == http.StatusTooManyRequests {
					resp.Body.Close()
					retryAfterSec := 5
					if ra := resp.Header.Get("Retry-After"); ra != "" {
						if sec, err := strconv.Atoi(ra); err == nil && sec > 0 {
							retryAfterSec = sec
						}
					}
					waitDuration := time.Duration(retryAfterSec)*time.Second + time.Duration(rand.Intn(1000))*time.Millisecond
					logger.Warnf("Chunked", "Chunk %d hit HTTP 429. Waiting %v before retry %d/%d...", s.Index, waitDuration, attempt, maxRetries)

					select {
					case <-ctx.Done():
						setErr(ctx.Err())
						return
					case <-time.After(waitDuration):
					}
					continue
				}

				// Check if server returned 200 OK for Range request with start > 0 (server doesn't support Range)
				if resp.StatusCode == http.StatusOK && currentPos > 0 {
					resp.Body.Close()
					setErr(fmt.Errorf("server does not support HTTP Range requests (returned 200 OK for range starting at byte %d)", currentPos))
					return
				}

				if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
					resp.Body.Close()
					if attempt == maxRetries {
						setErr(fmt.Errorf("chunk %d received unexpected HTTP %d (%s)", s.Index, resp.StatusCode, resp.Status))
						return
					}
					time.Sleep(backoff)
					backoff *= 2
					continue
				}

				buf := make([]byte, 128*1024) // 128KB read buffer per stream
				chunkReadErr := false

				for {
					select {
					case <-ctx.Done():
						resp.Body.Close()
						setErr(ctx.Err())
						return
					default:
					}

					n, readErr := resp.Body.Read(buf)
					if n > 0 {
						writePos := s.Current.Load()
						_, writeErr := out.WriteAt(buf[:n], writePos)
						if writeErr != nil {
							resp.Body.Close()
							setErr(fmt.Errorf("disk write failed at offset %d: %w", writePos, writeErr))
							return
						}
						s.Current.Add(int64(n))
						totalDownloaded.Add(int64(n))
					}

					if readErr != nil {
						resp.Body.Close()
						if readErr == io.EOF {
							break
						}
						chunkReadErr = true
						break
					}
				}

				if !chunkReadErr && s.Current.Load() > s.End {
					// Successfully finished this chunk
					return
				}

				if attempt < maxRetries {
					time.Sleep(backoff)
					backoff *= 2
				}
			}

			if s.Current.Load() <= s.End && activeErr == nil {
				setErr(fmt.Errorf("chunk %d incomplete (got %d of %d bytes)", s.Index, s.Current.Load()-s.Start, s.End-s.Start+1))
			}
		}(seg)
	}

	wg.Wait()
	cancelProgress()

	finalDownloaded := totalDownloaded.Load()

	if activeErr != nil {
		// Persist chunk state on pause or error - DO NOT DELETE destPath!
		_ = saveChunkState(destPath, totalSize, segments)
		out.Close()
		return finalDownloaded, activeErr
	}

	// Flush and close file
	_ = out.Sync()
	out.Close()

	// Download finished completely - remove metadata file
	RemoveChunkMeta(destPath)

	// Final progress update
	if onProgress != nil {
		onProgress(totalSize, totalSize, 0, 0, 100.0)
	}

	return finalDownloaded, nil
}
