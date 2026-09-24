package queue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (m *Manager) saveToDiskLocked() {
	if m.dataFile == "" {
		return
	}
	itemsCopy := make([]DownloadItem, 0, len(m.order))
	for _, id := range m.order {
		if it, ok := m.items[id]; ok {
			itemsCopy = append(itemsCopy, it.Snapshot())
		}
	}
	data, err := json.MarshalIndent(itemsCopy, "", "  ")
	if err != nil {
		return
	}
	tmpFile := m.dataFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err == nil {
		_ = os.Rename(tmpFile, m.dataFile)
	}
}

func (m *Manager) saveToDisk() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.saveToDiskLocked()
}

func (m *Manager) loadFromDisk() {
	if m.dataFile == "" {
		return
	}
	data, err := os.ReadFile(m.dataFile)
	if err != nil {
		return
	}
	var items []DownloadItem
	if err := json.Unmarshal(data, &items); err != nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, itemSnapshot := range items {
		// If item was mid-flight when process exited, reset to paused
		if itemSnapshot.Status == StatusDownloading || itemSnapshot.Status == StatusCompressing {
			itemSnapshot.Status = StatusPaused
			itemSnapshot.Speed = 0
			itemSnapshot.ETASeconds = 0
		}

		it := &DownloadItem{
			ID:                  itemSnapshot.ID,
			URL:                 itemSnapshot.URL,
			FileID:              itemSnapshot.FileID,
			Filename:            itemSnapshot.Filename,
			TargetFolder:        itemSnapshot.TargetFolder,
			DownloadedBytes:     itemSnapshot.DownloadedBytes,
			TotalBytes:          itemSnapshot.TotalBytes,
			Speed:               itemSnapshot.Speed,
			ETASeconds:          itemSnapshot.ETASeconds,
			Percentage:          itemSnapshot.Percentage,
			Status:              itemSnapshot.Status,
			Error:               itemSnapshot.Error,
			CreatedAt:           itemSnapshot.CreatedAt,
			LastTryAt:           itemSnapshot.LastTryAt,
			IsFolder:            itemSnapshot.IsFolder,
			ZipMode:             itemSnapshot.ZipMode,
			FolderTitle:         itemSnapshot.FolderTitle,
			TotalFiles:          itemSnapshot.TotalFiles,
			CompletedFiles:      itemSnapshot.CompletedFiles,
			CurrentFile:         itemSnapshot.CurrentFile,
			CompressionProgress: itemSnapshot.CompressionProgress,
			FolderFiles:         itemSnapshot.FolderFiles,
		}

		m.items[it.ID] = it
		m.order = append(m.order, it.ID)

		if it.Status == StatusQueued {
			select {
			case m.queueChan <- it:
			default:
			}
		}
	}
}

func (m *Manager) CheckFileExistence(id string) (bool, error) {
	m.mu.RLock()
	item, ok := m.items[id]
	m.mu.RUnlock()
	if !ok {
		return false, fmt.Errorf("item not found: %s", id)
	}

	item.mu.Lock()
	defer item.mu.Unlock()

	// Only verify completed or missing items
	if item.Status != StatusCompleted && item.Status != StatusMissing && item.Status != StatusCorrupted {
		return true, nil
	}

	targetPath := filepath.Join(item.TargetFolder, item.Filename)
	if item.IsFolder && !item.ZipMode && item.FolderTitle != "" {
		targetPath = filepath.Join(item.TargetFolder, item.FolderTitle)
	}

	_, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			if item.Status != StatusMissing {
				item.Status = StatusMissing
				item.Error = fmt.Sprintf("File or folder not found on disk at '%s' (might be moved, renamed, or deleted)", targetPath)
				go m.triggerBroadcast()
				go m.saveToDisk()
			}
			return false, nil
		}
		return false, err
	}

	// File exists! If it was marked missing, restore to completed
	if item.Status == StatusMissing {
		item.Status = StatusCompleted
		item.Error = ""
		go m.triggerBroadcast()
		go m.saveToDisk()
	}
	return true, nil
}

func (m *Manager) CheckAllFiles() {
	m.mu.RLock()
	ids := make([]string, len(m.order))
	copy(ids, m.order)
	m.mu.RUnlock()

	for _, id := range ids {
		_, _ = m.CheckFileExistence(id)
	}
}

// getUniqueFilename resolves filename collisions against files on disk and items currently in the queue.
// Must be called with m.mu held or safe for reading m.items.
func (m *Manager) getUniqueFilename(targetFolder, filename, excludeItemID string) string {
	if filename == "" {
		return filename
	}

	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	candidate := filename
	counter := 2

	for {
		collision := false

		// 1. Check if final file exists on disk
		finalPath := filepath.Join(targetFolder, candidate)
		if fi, err := os.Stat(finalPath); err == nil && !fi.IsDir() {
			collision = true
		}

		// 2. Check if another item in queue is using this filename in the same folder
		if !collision {
			for _, it := range m.items {
				if it.ID == excludeItemID {
					continue
				}
				it.mu.RLock()
				sameFolder := strings.EqualFold(filepath.Clean(it.TargetFolder), filepath.Clean(targetFolder))
				sameName := strings.EqualFold(it.Filename, candidate)
				it.mu.RUnlock()

				if sameFolder && sameName {
					collision = true
					break
				}
			}
		}

		if !collision {
			return candidate
		}

		candidate = fmt.Sprintf("%s (%d)%s", base, counter, ext)
		counter++
	}
}
