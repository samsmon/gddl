package chunked

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"gdrive-downloader/pkg/logger"
)

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

func initOrResumeSegments(destPath string, totalSize int64, numChunks int) ([]*ChunkSegment, int64) {
	minChunkSize := int64(1024 * 1024)
	if totalSize/int64(numChunks) < minChunkSize {
		numChunks = int(totalSize / minChunkSize)
		if numChunks < 2 {
			numChunks = 2
		}
	}

	var segments []*ChunkSegment
	var initialDownloaded int64 = 0

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
		return segments, initialDownloaded
	}

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
	return segments, initialDownloaded
}

func runProgressMonitor(
	progressCtx context.Context,
	destPath string,
	totalSize int64,
	initialDownloaded int64,
	totalDownloaded *atomic.Int64,
	segments []*ChunkSegment,
	onProgress ProgressCallback,
) {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	lastDownloaded := initialDownloaded
	lastTime := time.Now()
	lastSave := time.Now()

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

			var eta int64
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
			if now.Sub(lastSave) >= time.Second {
				_ = saveChunkState(destPath, totalSize, segments)
				lastSave = now
			}
		}
	}
}
