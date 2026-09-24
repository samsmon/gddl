package chunked

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"gdrive-downloader/pkg/logger"
)

func (d *Downloader) downloadSegmentWorker(
	ctx context.Context,
	s *ChunkSegment,
	out *os.File,
	downloadURL string,
	headers http.Header,
	totalDownloaded *atomic.Int64,
	setErr func(error),
	hasActiveErr func() bool,
) {
	// IDM-style staggered socket startup:
	// Introduce a small delay (80ms per stream index) so parallel TCP handshakes
	// do not trigger Cloudflare / CDN burst rate limiting
	if s.Index > 0 {
		select {
		case <-ctx.Done():
			setErr(ctx.Err())
			return
		case <-time.After(time.Duration(s.Index*80) * time.Millisecond):
		}
	}

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
			return
		}

		startEpoch := d.getEpoch()
		reqCtx, cancelReq := context.WithCancel(ctx)
		attemptDone := make(chan struct{})
		if startEpoch > 0 {
			go func(expectedEpoch uint64) {
				ticker := time.NewTicker(400 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-attemptDone:
						return
					case <-ctx.Done():
						return
					case <-ticker.C:
						if d.getEpoch() != expectedEpoch {
							cancelReq()
							return
						}
					}
				}
			}(startEpoch)
		}

		req, err := http.NewRequestWithContext(reqCtx, "GET", downloadURL, nil)
		if err != nil {
			close(attemptDone)
			cancelReq()
			setErr(err)
			return
		}

		for k, v := range headers {
			for _, val := range v {
				req.Header.Add(k, val)
			}
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", currentPos, s.End))

		resp, err := d.client.Do(req)
		if err != nil {
			close(attemptDone)
			cancelReq()
			if errors.Is(ctx.Err(), context.Canceled) {
				setErr(ctx.Err())
				return
			}
			if startEpoch > 0 && d.getEpoch() != startEpoch {
				logger.Infof("Chunked", "Chunk %d reconnecting via rotated WARP/Proxy IP at byte %d...", s.Index, s.Current.Load())
				attempt--
				backoff = time.Second
				continue
			}
			if attempt == maxRetries {
				setErr(fmt.Errorf("chunk %d failed after %d attempts: %w", s.Index, maxRetries, err))
				return
			}
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			close(attemptDone)
			cancelReq()
			d.notifyRateLimit(fmt.Sprintf("Chunk %d received HTTP 429 Too Many Requests", s.Index))

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

		if resp.StatusCode == http.StatusOK && currentPos > 0 {
			resp.Body.Close()
			close(attemptDone)
			cancelReq()
			setErr(fmt.Errorf("server does not support HTTP Range requests (returned 200 OK for range starting at byte %d)", currentPos))
			return
		}

		if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			close(attemptDone)
			cancelReq()
			if attempt == maxRetries {
				setErr(fmt.Errorf("chunk %d received unexpected HTTP %d (%s)", s.Index, resp.StatusCode, resp.Status))
				return
			}
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		buf := make([]byte, 128*1024)
		chunkReadErr := false
		proxyRotated := false

		for {
			select {
			case <-ctx.Done():
				resp.Body.Close()
				close(attemptDone)
				cancelReq()
				setErr(ctx.Err())
				return
			default:
			}

			if startEpoch > 0 && d.getEpoch() != startEpoch {
				resp.Body.Close()
				proxyRotated = true
				break
			}

			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				writePos := s.Current.Load()
				_, writeErr := out.WriteAt(buf[:n], writePos)
				if writeErr != nil {
					resp.Body.Close()
					close(attemptDone)
					cancelReq()
					setErr(fmt.Errorf("disk write failed at offset %d: %w", writePos, writeErr))
					return
				}
				s.Current.Add(int64(n))
				totalDownloaded.Add(int64(n))
			}

			if readErr != nil {
				resp.Body.Close()
				if startEpoch > 0 && d.getEpoch() != startEpoch && ctx.Err() == nil {
					proxyRotated = true
					break
				}
				if readErr == io.EOF {
					break
				}
				chunkReadErr = true
				break
			}
		}

		close(attemptDone)
		cancelReq()

		if proxyRotated && ctx.Err() == nil {
			logger.Infof("Chunked", "Chunk %d switching to newly rotated WARP/Proxy IP at byte %d...", s.Index, s.Current.Load())
			attempt--
			backoff = time.Second
			continue
		}

		if !chunkReadErr && s.Current.Load() > s.End {
			return
		}

		if attempt < maxRetries {
			time.Sleep(backoff)
			backoff *= 2
		}
	}

	if s.Current.Load() <= s.End && !hasActiveErr() {
		setErr(fmt.Errorf("chunk %d incomplete (got %d of %d bytes)", s.Index, s.Current.Load()-s.Start, s.End-s.Start+1))
	}
}
