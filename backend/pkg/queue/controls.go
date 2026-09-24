package queue

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gdrive-downloader/pkg/logger"
)

func (m *Manager) drainQueueChan() {
	for {
		select {
		case <-m.queueChan:
		default:
			return
		}
	}
}

func (m *Manager) snapshotOrderedItems() []*DownloadItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]*DownloadItem, 0, len(m.order))
	for _, id := range m.order {
		if it, ok := m.items[id]; ok {
			items = append(items, it)
		}
	}
	return items
}

func removeItemFilesOnDisk(targetFolder, id, filename string) {
	_ = os.RemoveAll(filepath.Join(targetFolder, fmt.Sprintf(".tmp_gdrive_%s", id)))
	if filename != "" && targetFolder != "" {
		base := filepath.Join(targetFolder, filename)
		for _, suffix := range []string{"", ".part", ".gddl-chunks", ".part.gddl-chunks"} {
			_ = os.Remove(base + suffix)
		}
	}
}

func (m *Manager) Pause(id string) error {
	m.mu.RLock()
	item, exists := m.items[id]
	m.mu.RUnlock()
	if !exists {
		return fmt.Errorf("item not found: %s", id)
	}
	item.mu.Lock()
	if item.Status == StatusCompleted || item.Status == StatusPaused || item.Status == StatusMoving {
		item.mu.Unlock()
		return nil
	}
	item.Status = StatusPaused
	if item.cancelFunc != nil {
		item.cancelFunc()
		item.cancelFunc = nil
	}
	item.Speed, item.ETASeconds = 0, 0
	if item.IsFolder && item.ZipMode {
		for i := range item.FolderFiles {
			if item.FolderFiles[i].Status == "downloading" {
				item.FolderFiles[i].Status = "paused"
			}
		}
	}
	item.mu.Unlock()
	logger.Infof("Queue", "Paused download '%s'", item.Filename)
	m.triggerBroadcast()
	m.saveToDisk()
	return nil
}

func (m *Manager) Start(id string) error {
	m.mu.RLock()
	item, exists := m.items[id]
	m.mu.RUnlock()
	if !exists {
		return fmt.Errorf("item not found: %s", id)
	}
	item.mu.Lock()
	if item.Status == StatusDownloading || item.Status == StatusQueued || item.Status == StatusCompressing || item.Status == StatusMoving || item.Status == StatusCompleted {
		item.mu.Unlock()
		return nil
	}
	item.Status, item.LastTryAt, item.Error = StatusQueued, time.Now(), ""
	if item.IsFolder && item.ZipMode {
		for i := range item.FolderFiles {
			if item.FolderFiles[i].Status != "completed" {
				item.FolderFiles[i].Status = "queued"
			}
		}
	}
	item.mu.Unlock()
	logger.Infof("Queue", "Resumed/Started download '%s'", item.Filename)
	m.queueChan <- item
	m.triggerBroadcast()
	m.saveToDisk()
	return nil
}

func (m *Manager) Restart(id string) error {
	m.mu.RLock()
	item, exists := m.items[id]
	m.mu.RUnlock()
	if !exists {
		return fmt.Errorf("item not found: %s", id)
	}
	item.mu.Lock()
	if item.cancelFunc != nil {
		item.cancelFunc()
		item.cancelFunc = nil
	}
	item.DownloadedBytes, item.TotalBytes, item.Percentage = 0, 0, 0
	item.Speed, item.ETASeconds, item.CompletedFiles = 0, 0, 0
	item.CompressionProgress, item.CurrentFile, item.Error = 0, "", ""
	item.Status, item.LastTryAt = StatusQueued, time.Now()
	for i := range item.FolderFiles {
		item.FolderFiles[i].Status = "queued"
	}
	targetFolder, itemId, filename := item.TargetFolder, item.ID, item.Filename
	item.mu.Unlock()

	removeItemFilesOnDisk(targetFolder, itemId, filename)
	logger.Infof("Queue", "Restarted download '%s' from beginning (staging cleared)", filename)
	m.queueChan <- item
	m.triggerBroadcast()
	m.saveToDisk()
	return nil
}

func (m *Manager) PauseAll() int {
	m.drainQueueChan()
	pausedCount := 0
	for _, item := range m.snapshotOrderedItems() {
		item.mu.Lock()
		if item.Status == StatusDownloading || item.Status == StatusQueued || item.Status == StatusCompressing {
			item.Status = StatusPaused
			if item.cancelFunc != nil {
				item.cancelFunc()
				item.cancelFunc = nil
			}
			item.Speed, item.ETASeconds = 0, 0
			if item.IsFolder && item.ZipMode {
				for i := range item.FolderFiles {
					if item.FolderFiles[i].Status == "downloading" || item.FolderFiles[i].Status == "queued" {
						item.FolderFiles[i].Status = "paused"
					}
				}
			}
			pausedCount++
		}
		item.mu.Unlock()
	}
	logger.Infof("Queue", "PauseAll: paused %d downloads", pausedCount)
	m.saveToDisk()
	m.triggerBroadcast()
	return pausedCount
}

func (m *Manager) ResumeAll() int {
	m.drainQueueChan()
	now, resumedCount := time.Now(), 0
	for _, item := range m.snapshotOrderedItems() {
		item.mu.Lock()
		if item.Status == StatusDownloading || item.Status == StatusCompressing || item.Status == StatusMoving || item.Status == StatusCompleted {
			item.mu.Unlock()
			continue
		}
		if item.Status == StatusPaused || item.Status == StatusFailed || item.Status == StatusCancelled || item.Status == StatusQueued {
			item.Status, item.Error = StatusQueued, ""
			item.Speed, item.ETASeconds, item.LastTryAt = 0, 0, now
			if item.IsFolder && item.ZipMode {
				for i := range item.FolderFiles {
					if item.FolderFiles[i].Status != "completed" {
						item.FolderFiles[i].Status = "queued"
					}
				}
			}
			item.mu.Unlock()
			select {
			case m.queueChan <- item:
			default:
				go func(it *DownloadItem) { m.queueChan <- it }(item)
			}
			resumedCount++
		} else {
			item.mu.Unlock()
		}
	}
	logger.Infof("Queue", "ResumeAll: queued %d downloads in queue order", resumedCount)
	m.saveToDisk()
	m.triggerBroadcast()
	return resumedCount
}

func (m *Manager) Cancel(id string) error {
	m.mu.RLock()
	item, exists := m.items[id]
	m.mu.RUnlock()
	if !exists {
		return fmt.Errorf("item not found: %s", id)
	}
	item.mu.Lock()
	if item.cancelFunc != nil {
		item.cancelFunc()
		item.cancelFunc = nil
	}
	item.Status = StatusCancelled
	item.Speed, item.ETASeconds = 0, 0
	item.mu.Unlock()
	m.triggerBroadcast()
	m.saveToDisk()
	return nil
}

func (m *Manager) Delete(id string, deleteFile bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, exists := m.items[id]
	if !exists {
		return fmt.Errorf("item not found: %s", id)
	}
	item.mu.Lock()
	if item.cancelFunc != nil {
		item.cancelFunc()
		item.cancelFunc = nil
	}
	targetFolder, filename, isFolder, zipMode := item.TargetFolder, item.Filename, item.IsFolder, item.ZipMode
	item.mu.Unlock()

	_ = os.RemoveAll(filepath.Join(targetFolder, fmt.Sprintf(".tmp_gdrive_%s", id)))
	if deleteFile {
		if isFolder && !zipMode {
			if targetFolder != "" && targetFolder != m.TargetFolder {
				_ = os.RemoveAll(targetFolder)
			}
		} else {
			removeItemFilesOnDisk(targetFolder, id, filename)
		}
	}

	delete(m.items, id)
	newOrder := make([]string, 0, len(m.order))
	for _, oid := range m.order {
		if oid != id {
			newOrder = append(newOrder, oid)
		}
	}
	m.order = newOrder
	m.saveToDiskLocked()
	m.triggerBroadcast()
	return nil
}

func (m *Manager) ClearCompleted(deleteFile bool) {
	m.mu.Lock()
	var newOrder []string
	var itemsToDelete []*DownloadItem
	for _, id := range m.order {
		item := m.items[id]
		item.mu.RLock()
		st := item.Status
		item.mu.RUnlock()
		if st == StatusCompleted || st == StatusFailed || st == StatusCancelled || st == StatusCorrupted {
			delete(m.items, id)
			if deleteFile {
				itemsToDelete = append(itemsToDelete, item)
			}
		} else {
			newOrder = append(newOrder, id)
		}
	}
	m.order = newOrder
	m.saveToDiskLocked()
	m.mu.Unlock()

	if deleteFile {
		for _, item := range itemsToDelete {
			item.mu.RLock()
			tgt, fn, isFolder, zipMode, folderTitle, id := item.TargetFolder, item.Filename, item.IsFolder, item.ZipMode, item.FolderTitle, item.ID
			item.mu.RUnlock()
			if isFolder && zipMode {
				_ = os.Remove(filepath.Join(tgt, fn))
				_ = os.RemoveAll(filepath.Join(tgt, fmt.Sprintf(".tmp_gdrive_%s", id)))
			} else if isFolder && !zipMode && folderTitle != "" {
				if subfolder := filepath.Join(tgt, folderTitle); subfolder != tgt && subfolder != filepath.Dir(subfolder) {
					_ = os.RemoveAll(subfolder)
				}
			} else if fn != "" {
				_ = os.Remove(filepath.Join(tgt, fn))
			}
		}
	}
	m.triggerBroadcast()
}
