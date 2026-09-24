package queue

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gdrive-downloader/pkg/logger"
)

func (m *Manager) SetItemTargetFolder(id string, newFolder string) error {
	newFolder = strings.TrimSpace(newFolder)
	if newFolder == "" {
		return fmt.Errorf("target folder cannot be empty")
	}

	m.mu.RLock()
	item, exists := m.items[id]
	m.mu.RUnlock()
	if !exists {
		return fmt.Errorf("item not found: %s", id)
	}

	item.mu.Lock()
	if item.Status == StatusMoving {
		item.mu.Unlock()
		return fmt.Errorf("file is already being moved")
	}
	if item.Status == StatusDownloading || item.Status == StatusCompressing {
		item.mu.Unlock()
		return fmt.Errorf("cannot move file while downloading or compressing; please pause download first")
	}

	oldFolder := item.TargetFolder
	itemFilename := item.Filename
	if item.IsFolder && !item.ZipMode && item.FolderTitle != "" {
		itemFilename = item.FolderTitle
	}
	prevStatus := item.Status
	item.mu.Unlock()

	if oldFolder == newFolder {
		return nil
	}

	oldPath := filepath.Join(oldFolder, itemFilename)
	newPath := filepath.Join(newFolder, itemFilename)
	oldPart := oldPath + ".part"
	newPart := newPath + ".part"

	hasFile := false
	if _, err := os.Stat(oldPath); err == nil {
		hasFile = true
	}
	hasPart := false
	if fi, err := os.Stat(oldPart); err == nil && !fi.IsDir() {
		hasPart = true
	}

	// If no physical file or .part file exists on disk yet, simply update target folder
	if !hasFile && !hasPart {
		item.mu.Lock()
		item.TargetFolder = newFolder
		item.mu.Unlock()
		go m.CheckFileExistence(id)
		m.triggerBroadcast()
		m.saveToDisk()
		logger.Infof("Queue", "Changed target location of queued item '%s' to '%s'", itemFilename, newFolder)
		return nil
	}

	// File exists on disk: perform move asynchronously with real-time progress
	go func() {
		item.mu.Lock()
		item.Status = StatusMoving
		item.MoveProgress = 0
		item.Speed = 0
		item.mu.Unlock()
		m.triggerBroadcast()

		var moveErr error
		if hasFile {
			moveErr = m.moveFileWithProgress(item, oldPath, newPath)
		} else if hasPart {
			moveErr = m.moveFileWithProgress(item, oldPart, newPart)
		}

		item.mu.Lock()
		if moveErr == nil {
			item.TargetFolder = newFolder
			item.Status = prevStatus
			item.MoveProgress = 100
			item.Speed = 0
			logger.Infof("Queue", "Successfully moved '%s' from '%s' to '%s'", itemFilename, oldFolder, newFolder)
		} else {
			item.Status = prevStatus
			item.MoveProgress = 0
			item.Speed = 0
			logger.Errorf("Queue", "Failed to move '%s' to '%s': %v", itemFilename, newFolder, moveErr)
		}
		item.mu.Unlock()

		m.CheckFileExistence(id)
		m.triggerBroadcast()
		m.saveToDisk()
	}()

	return nil
}

func (m *Manager) moveFileWithProgress(item *DownloadItem, oldPath string, newPath string) error {
	_ = os.MkdirAll(filepath.Dir(newPath), 0755)

	// 1. Try atomic rename first (instant if on the same disk/volume)
	if err := os.Rename(oldPath, newPath); err == nil {
		item.mu.Lock()
		item.MoveProgress = 100
		item.mu.Unlock()
		m.triggerBroadcast()
		return nil
	}

	srcStat, err := os.Stat(oldPath)
	if err != nil {
		return err
	}
	if srcStat.IsDir() {
		return m.moveDirWithProgress(item, oldPath, newPath)
	}

	// 2. Cross-device move fallback (stream copy with real-time progress)
	src, err := os.Open(oldPath)
	if err != nil {
		return err
	}
	defer src.Close()
	totalSize := srcStat.Size()

	dst, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer dst.Close()

	buf := make([]byte, 4*1024*1024) // 4MB buffer
	var copied, lastCopied int64
	lastReport := time.Now()

	for {
		nr, rerr := src.Read(buf)
		if nr > 0 {
			nw, werr := dst.Write(buf[:nr])
			if werr != nil {
				dst.Close()
				_ = os.Remove(newPath)
				return werr
			}
			copied += int64(nw)

			now := time.Now()
			elapsed := now.Sub(lastReport)
			if elapsed >= 200*time.Millisecond || copied == totalSize {
				var speed int64
				if elapsed.Seconds() > 0 {
					speed = int64(float64(copied-lastCopied) / elapsed.Seconds())
				}
				item.mu.Lock()
				if totalSize > 0 {
					item.MoveProgress = math.Round((float64(copied)/float64(totalSize))*1000) / 10
				}
				item.Speed = speed
				item.mu.Unlock()
				m.triggerBroadcast()
				lastReport = now
				lastCopied = copied
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			dst.Close()
			_ = os.Remove(newPath)
			return rerr
		}
	}

	_ = dst.Sync()
	_ = dst.Close()
	_ = src.Close()

	// Verify target file size before deleting source
	if dstStat, err := os.Stat(newPath); err == nil && dstStat.Size() == totalSize {
		_ = os.Remove(oldPath)
		return nil
	}
	return fmt.Errorf("copied file size mismatch")
}

func (m *Manager) moveDirWithProgress(item *DownloadItem, oldDir string, newDir string) error {
	var totalSize int64
	var fileList []string

	err := filepath.Walk(oldDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
			fileList = append(fileList, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	var copied, lastCopied int64
	lastReport := time.Now()
	buf := make([]byte, 4*1024*1024)

	for _, srcPath := range fileList {
		rel, err := filepath.Rel(oldDir, srcPath)
		if err != nil {
			return err
		}
		destPath := filepath.Join(newDir, rel)
		_ = os.MkdirAll(filepath.Dir(destPath), 0755)

		src, err := os.Open(srcPath)
		if err != nil {
			return err
		}

		dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			src.Close()
			return err
		}

		for {
			nr, rerr := src.Read(buf)
			if nr > 0 {
				nw, werr := dst.Write(buf[:nr])
				if werr != nil {
					src.Close()
					dst.Close()
					return werr
				}
				copied += int64(nw)

				now := time.Now()
				elapsed := now.Sub(lastReport)
				if elapsed >= 200*time.Millisecond || copied == totalSize {
					var speed int64
					if elapsed.Seconds() > 0 {
						speed = int64(float64(copied-lastCopied) / elapsed.Seconds())
					}
					item.mu.Lock()
					if totalSize > 0 {
						item.MoveProgress = math.Round((float64(copied)/float64(totalSize))*1000) / 10
					}
					item.Speed = speed
					item.mu.Unlock()
					m.triggerBroadcast()
					lastReport = now
					lastCopied = copied
				}
			}
			if rerr != nil {
				if rerr == io.EOF {
					break
				}
				src.Close()
				dst.Close()
				return rerr
			}
		}
		_ = dst.Sync()
		dst.Close()
		src.Close()
	}

	_ = os.RemoveAll(oldDir)
	return nil
}
