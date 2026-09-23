package gdrive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"gdrive-downloader/pkg/logger"
)

// ChunkSegment represents a byte range [Start, End] of a file.
type ChunkSegment struct {
	Index   int
	Start   int64
	End     int64
	Current int64
}

// ChunkedDownloader manages multi-stream segmented downloads with parallel HTTP Range requests.
type ChunkedDownloader struct {
	client *http.Client
}

// NewChunkedDownloader creates a new ChunkedDownloader with the provided client.
func NewChunkedDownloader(client *http.Client) *ChunkedDownloader {
	return &ChunkedDownloader{
		client: client,
	}
}

// DownloadSegmented downloads a file in parallel chunks using HTTP Range requests.
func (cd *ChunkedDownloader) DownloadSegmented(
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

	// Clamp chunks to sensible bounds
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

	// Open or create destination file
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed creating output file: %w", err)
	}
	defer out.Close()

	// Pre-allocate full file size on disk for contiguous allocation & instant WriteAt
	if err := out.Truncate(totalSize); err != nil {
		logger.Warnf("Download", "Could not pre-allocate file size for %s: %v", destPath, err)
	}

	// Divide into segments
	chunkSize := totalSize / int64(numChunks)
	segments := make([]*ChunkSegment, numChunks)
	for i := 0; i < numChunks; i++ {
		start := int64(i) * chunkSize
		end := int64(i+1)*chunkSize - 1
		if i == numChunks-1 {
			end = totalSize - 1
		}
		segments[i] = &ChunkSegment{
			Index:   i,
			Start:   start,
			End:     end,
			Current: start,
		}
	}

	var totalDownloaded atomic.Int64
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
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()

		var lastDownloaded int64 = 0
		var lastTime = time.Now()

		for {
			select {
			case <-progressCtx.Done():
				return
			case now := <-ticker.C:
				current := totalDownloaded.Load()
				elapsed := now.Sub(lastTime).Seconds()
				if elapsed <= 0 {
					elapsed = 0.25
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
			}
		}
	}()

	var wg sync.WaitGroup
	wg.Add(numChunks)

	for _, seg := range segments {
		go func(s *ChunkSegment) {
			defer wg.Done()

			maxRetries := 3
			for attempt := 1; attempt <= maxRetries; attempt++ {
				select {
				case <-ctx.Done():
					setErr(ctx.Err())
					return
				default:
				}

				if s.Current > s.End {
					// Segment already complete
					return
				}

				req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
				if err != nil {
					setErr(err)
					return
				}

				// Copy headers (Auth, Cookies, User-Agent)
				for k, v := range headers {
					for _, val := range v {
						req.Header.Add(k, val)
					}
				}
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", s.Current, s.End))

				resp, err := cd.client.Do(req)
				if err != nil {
					if errors.Is(ctx.Err(), context.Canceled) {
						setErr(ctx.Err())
						return
					}
					if attempt == maxRetries {
						setErr(fmt.Errorf("chunk %d failed after %d attempts: %w", s.Index, maxRetries, err))
						return
					}
					time.Sleep(time.Duration(attempt*500) * time.Millisecond)
					continue
				}

				if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
					resp.Body.Close()
					if attempt == maxRetries {
						setErr(fmt.Errorf("chunk %d received HTTP %d", s.Index, resp.StatusCode))
						return
					}
					time.Sleep(time.Duration(attempt*500) * time.Millisecond)
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
						_, writeErr := out.WriteAt(buf[:n], s.Current)
						if writeErr != nil {
							resp.Body.Close()
							setErr(fmt.Errorf("disk write failed at offset %d: %w", s.Current, writeErr))
							return
						}
						s.Current += int64(n)
						totalDownloaded.Add(int64(n))
					}

					if readErr != nil {
						resp.Body.Close()
						if readErr == io.EOF {
							// Stream reached end
							break
						}
						// Disconnect during read -> retry remaining slice
						chunkReadErr = true
						break
					}
				}

				if !chunkReadErr && s.Current > s.End {
					// Successfully finished this chunk
					return
				}

				if attempt < maxRetries {
					time.Sleep(time.Duration(attempt*500) * time.Millisecond)
				}
			}

			if s.Current <= s.End && activeErr == nil {
				setErr(fmt.Errorf("chunk %d incomplete (got %d of %d bytes)", s.Index, s.Current-s.Start, s.End-s.Start+1))
			}
		}(seg)
	}

	wg.Wait()
	cancelProgress()

	finalDownloaded := totalDownloaded.Load()

	if activeErr != nil {
		out.Close()
		_ = os.Remove(destPath)
		return finalDownloaded, activeErr
	}

	// Final progress update
	if onProgress != nil {
		onProgress(totalSize, totalSize, 0, 0, 100.0)
	}

	return finalDownloaded, nil
}
