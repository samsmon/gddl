package queue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gdrive-downloader/pkg/gdrive"
	"gdrive-downloader/pkg/logger"
)

func (m *Manager) downloadFolderZip(ctx context.Context, item *DownloadItem) {
	item.mu.RLock()
	files := item.folderFiles
	folderID := item.FileID
	targetFolder := item.TargetFolder
	folderTitle := item.FolderTitle
	item.mu.RUnlock()

	logger.Infof("FolderZip", "Starting folder download '%s' (ID: %s) in ZIP mode", folderTitle, folderID)

	if len(files) == 0 {
		info, err := m.downloader.FetchFolderInfo(ctx, folderID)
		if err != nil {
			logger.Errorf("FolderZip", "Failed to load folder contents for %s: %v", folderID, err)
			item.mu.Lock()
			item.Status = StatusFailed
			item.Error = fmt.Sprintf("Failed to load folder contents: %v", err)
			item.Speed = 0
			item.ETASeconds = 0
			item.cancelFunc = nil
			item.mu.Unlock()
			m.triggerBroadcast()
			m.saveToDisk()
			return
		}
		files = info.Files
		if folderTitle == "" {
			folderTitle = info.Title
		}
		childFiles := make([]ChildFileItem, len(files))
		for i, f := range files {
			childFiles[i] = ChildFileItem{ID: f.ID, Filename: f.Filename, Status: "queued"}
		}
		item.mu.Lock()
		item.folderFiles = files
		item.FolderFiles = childFiles
		item.TotalFiles = len(files)
		item.FolderTitle = folderTitle
		item.mu.Unlock()
	}

	stagingDir := filepath.Join(targetFolder, fmt.Sprintf(".tmp_gdrive_%s", item.ID))
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		logger.Errorf("FolderZip", "Failed to create staging directory %s: %v", stagingDir, err)
		item.mu.Lock()
		item.Status = StatusFailed
		item.Error = fmt.Sprintf("Failed to create staging directory: %v", err)
		item.cancelFunc = nil
		item.mu.Unlock()
		m.triggerBroadcast()
		m.saveToDisk()
		return
	}

	// 1. Initial scan: verify which files are ALREADY downloaded in stagingDir
	item.mu.Lock()
	var completedCount int
	var completedBytes int64
	for i, f := range files {
		childPath := filepath.Join(stagingDir, f.Filename)
		if fi, err := os.Stat(childPath); err == nil && fi.Size() > 0 {
			if i < len(item.FolderFiles) && item.FolderFiles[i].Status == "completed" {
				completedCount++
				completedBytes += fi.Size()
			}
		} else if i < len(item.FolderFiles) && item.FolderFiles[i].Status != "completed" {
			item.FolderFiles[i].Status = "queued"
		}
	}
	item.CompletedFiles = completedCount
	item.DownloadedBytes = completedBytes
	if item.TotalFiles > 0 {
		item.Percentage = float64(completedCount) / float64(item.TotalFiles) * 100.0
	}
	item.mu.Unlock()
	m.triggerBroadcast()

	logger.Infof("FolderZip", "Folder '%s': %d of %d files already downloaded in staging directory", folderTitle, completedCount, len(files))

	// 2. Download missing child files into staging directory
	for i, f := range files {
		if ctx.Err() != nil {
			break
		}

		childPath := filepath.Join(stagingDir, f.Filename)
		item.mu.RLock()
		alreadyDone := i < len(item.FolderFiles) && item.FolderFiles[i].Status == "completed"
		item.mu.RUnlock()

		if alreadyDone {
			if fi, err := os.Stat(childPath); err == nil && fi.Size() > 0 {
				logger.Infof("FolderZip", "[%d/%d] Skipping '%s' (already downloaded)", i+1, len(files), f.Filename)
				continue
			}
		}

		item.mu.Lock()
		item.CurrentFile = f.Filename
		if i < len(item.FolderFiles) {
			item.FolderFiles[i].Status = "downloading"
		}
		item.mu.Unlock()
		m.triggerBroadcast()

		logger.Infof("FolderZip", "[%d/%d] Downloading '%s' (ID: %s)", i+1, len(files), f.Filename, f.ID)

		var downloadedForFile, written int64
		var dlErr error

		for attempt := 1; attempt <= 2; attempt++ {
			if ctx.Err() != nil {
				break
			}
			_, written, dlErr = m.downloader.Download(
				ctx,
				f.ID,
				stagingDir,
				func(dlBytes, totBytes, spd, eta int64, pct float64) {
					downloadedForFile = dlBytes
					item.mu.Lock()
					item.Speed = spd
					item.ETASeconds = eta
					if item.TotalFiles > 0 {
						item.Percentage = (float64(item.CompletedFiles)*100.0 + pct) / float64(item.TotalFiles)
					}
					if i < len(item.FolderFiles) && totBytes > 0 {
						item.FolderFiles[i].Size = totBytes
					}
					item.mu.Unlock()
				},
				f.Filename,
			)
			if dlErr == nil || ctx.Err() != nil || strings.Contains(dlErr.Error(), "context canceled") {
				break
			}
			if attempt < 2 {
				logger.Warnf("FolderZip", "[%d/%d] Attempt %d failed for '%s': %v. Retrying in 1.5s...", i+1, len(files), attempt, f.Filename, dlErr)
				select {
				case <-ctx.Done():
				case <-time.After(1500 * time.Millisecond):
				}
			}
		}

		if dlErr != nil {
			item.mu.Lock()
			item.cancelFunc = nil
			isCancel := ctx.Err() != nil || strings.Contains(dlErr.Error(), "context canceled")
			if isCancel {
				logger.Warnf("FolderZip", "Download paused at file [%d/%d] '%s'. Progress preserved in staging.", i+1, len(files), f.Filename)
				if i < len(item.FolderFiles) {
					item.FolderFiles[i].Status = "paused"
				}
				if item.Status != StatusPaused {
					item.Status = StatusPaused
				}
			} else {
				logger.Errorf("FolderZip", "[%d/%d] Permanent failure downloading '%s': %v", i+1, len(files), f.Filename, dlErr)
				if i < len(item.FolderFiles) {
					item.FolderFiles[i].Status = "failed"
				}
				item.Status = StatusFailed
				item.Error = fmt.Sprintf("Failed to download '%s': %v", f.Filename, dlErr)
			}
			item.Speed = 0
			item.ETASeconds = 0
			item.mu.Unlock()
			m.triggerBroadcast()
			m.saveToDisk()
			return
		}

		item.mu.Lock()
		item.CompletedFiles++
		item.DownloadedBytes += downloadedForFile
		if i < len(item.FolderFiles) {
			item.FolderFiles[i].Status = "completed"
			if written > 0 {
				item.FolderFiles[i].Size = written
			}
		}
		if item.TotalFiles > 0 {
			item.Percentage = float64(item.CompletedFiles) / float64(item.TotalFiles) * 100.0
		}
		item.mu.Unlock()
		m.triggerBroadcast()
		logger.Successf("FolderZip", "[%d/%d] Finished '%s' (%d bytes)", i+1, len(files), f.Filename, written)
	}

	if ctx.Err() != nil {
		item.mu.Lock()
		item.cancelFunc = nil
		if item.Status != StatusPaused {
			item.Status = StatusPaused
		}
		item.Speed = 0
		item.ETASeconds = 0
		item.mu.Unlock()
		m.triggerBroadcast()
		m.saveToDisk()
		return
	}

	// 3. Local compression into .zip
	item.mu.Lock()
	item.Status = StatusCompressing
	item.Speed = 0
	item.ETASeconds = 0
	item.CompressionProgress = 0
	item.CurrentFile = "Starting ZIP compression..."
	item.mu.Unlock()
	m.triggerBroadcast()

	zipFilename := folderTitle + ".zip"
	if zipFilename == ".zip" {
		zipFilename = item.Filename
	}
	zipDstPath := filepath.Join(targetFolder, zipFilename)
	logger.Infof("Compress", "Compressing %d files into '%s'...", len(files), zipDstPath)

	zipErr := gdrive.CompressFolderToZip(ctx, stagingDir, zipDstPath, func(current, total int, curFile string) {
		item.mu.Lock()
		item.CurrentFile = curFile
		item.CompressionProgress = float64(current) / float64(total) * 100.0
		item.Percentage = item.CompressionProgress
		item.mu.Unlock()
		m.triggerBroadcast()
	})
	_ = os.RemoveAll(stagingDir)

	item.mu.Lock()
	item.cancelFunc = nil
	if zipErr != nil {
		if ctx.Err() != nil || strings.Contains(zipErr.Error(), "context canceled") {
			if item.Status != StatusPaused {
				item.Status = StatusCancelled
			}
			logger.Warnf("Compress", "Compression paused/cancelled for '%s'", zipFilename)
		} else {
			item.Status = StatusFailed
			item.Error = fmt.Sprintf("Compression failed: %v", zipErr)
			logger.Errorf("Compress", "Compression failed for '%s': %v", zipFilename, zipErr)
		}
		item.Speed = 0
		item.ETASeconds = 0
		item.mu.Unlock()
		m.triggerBroadcast()
		m.saveToDisk()
		return
	}

	if fi, err := os.Stat(zipDstPath); err == nil {
		item.TotalBytes = fi.Size()
		item.DownloadedBytes = fi.Size()
		logger.Successf("Compress", "ZIP archive '%s' created successfully (%d bytes)", zipFilename, fi.Size())
	}

	item.Status = StatusCompleted
	item.Filename = zipFilename
	item.Percentage = 100
	item.CompressionProgress = 100
	item.Speed = 0
	item.ETASeconds = 0
	item.CurrentFile = ""
	item.mu.Unlock()
	m.triggerBroadcast()
	m.saveToDisk()
	logger.Successf("Queue", "Completed folder ZIP transfer '%s'", zipFilename)
}
