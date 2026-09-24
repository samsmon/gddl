package queue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gdrive-downloader/pkg/gdrive"
	"gdrive-downloader/pkg/logger"
)

func (m *Manager) addFolderURL(rawURL, folderID, targetFolder string, zipMode bool, action string, now time.Time) []*DownloadItem {
	var added []*DownloadItem

	folderInfo, err := m.downloader.FetchFolderInfo(context.Background(), folderID)
	if err != nil {
		id := fmt.Sprintf("%d_folder_%s", time.Now().UnixNano(), folderID)
		item := &DownloadItem{
			ID:           id,
			URL:          rawURL,
			FileID:       folderID,
			Filename:     fmt.Sprintf("Folder %s", folderID),
			TargetFolder: targetFolder,
			Status:       StatusFailed,
			Error:        fmt.Sprintf("Failed to load folder contents: %v", err),
			CreatedAt:    now,
			LastTryAt:    now,
			IsFolder:     true,
			ZipMode:      zipMode,
		}
		m.mu.Lock()
		m.items[id] = item
		m.order = append(m.order, id)
		added = append(added, item)
		m.mu.Unlock()
		return added
	}

	if zipMode {
		zipFilename := folderInfo.Title + ".zip"
		zipDstPath := filepath.Join(targetFolder, zipFilename)

		var existingItem *DownloadItem
		m.mu.RLock()
		for _, it := range m.items {
			if it.FileID == folderID || it.URL == rawURL {
				existingItem = it
				break
			}
		}
		m.mu.RUnlock()

		if action == "rename" {
			zipFilename = gdrive.UniqueFilename(targetFolder, zipFilename)
			zipDstPath = filepath.Join(targetFolder, zipFilename)
		} else if action == "overwrite" {
			if existingItem != nil {
				existingItem.mu.Lock()
				if existingItem.cancelFunc != nil {
					existingItem.cancelFunc()
				}
				existingItem.mu.Unlock()
			}
			_ = os.Remove(zipDstPath)
		} else if action == "monitor" {
			if fi, statErr := os.Stat(zipDstPath); statErr == nil && fi.Size() > 0 {
				integrityErr := gdrive.VerifyFileIntegrity(zipDstPath, 0)
				childFiles := make([]ChildFileItem, len(folderInfo.Files))
				for i, f := range folderInfo.Files {
					childFiles[i] = ChildFileItem{
						ID:       f.ID,
						Filename: f.Filename,
						Status:   "completed",
					}
				}

				id := fmt.Sprintf("%d_folder_%s", time.Now().UnixNano(), folderID)
				if existingItem != nil {
					id = existingItem.ID
				}

				status := StatusCompleted
				errMsg := ""
				if integrityErr != nil {
					status = StatusCorrupted
					errMsg = fmt.Sprintf("Existing ZIP is corrupted or incomplete: %v", integrityErr)
					logger.Errorf("Monitor", "Existing archive '%s' integrity check failed: %v", zipFilename, integrityErr)
				} else {
					logger.Successf("Monitor", "Re-monitored verified complete ZIP '%s' (%d bytes)", zipFilename, fi.Size())
				}

				item := &DownloadItem{
					ID:                  id,
					URL:                 rawURL,
					FileID:              folderID,
					Filename:            zipFilename,
					FolderTitle:         folderInfo.Title,
					TargetFolder:        targetFolder,
					Status:              status,
					Error:               errMsg,
					TotalBytes:          fi.Size(),
					DownloadedBytes:     fi.Size(),
					Percentage:          100,
					CompressionProgress: 100,
					CreatedAt:           now,
					LastTryAt:           now,
					IsFolder:            true,
					ZipMode:             true,
					TotalFiles:          len(folderInfo.Files),
					CompletedFiles:      len(folderInfo.Files),
					FolderFiles:         childFiles,
					folderFiles:         folderInfo.Files,
				}

				m.mu.Lock()
				m.items[id] = item
				if existingItem == nil {
					m.order = append(m.order, id)
				}
				added = append(added, item)
				m.mu.Unlock()
				return added
			}
		}

		id := fmt.Sprintf("%d_folder_%s", time.Now().UnixNano(), folderID)
		if action == "overwrite" && existingItem != nil {
			id = existingItem.ID
		}
		childFiles := make([]ChildFileItem, len(folderInfo.Files))
		for i, f := range folderInfo.Files {
			childFiles[i] = ChildFileItem{
				ID:       f.ID,
				Filename: f.Filename,
				Status:   "queued",
			}
		}
		item := &DownloadItem{
			ID:           id,
			URL:          rawURL,
			FileID:       folderID,
			Filename:     zipFilename,
			FolderTitle:  folderInfo.Title,
			TargetFolder: targetFolder,
			Status:       StatusQueued,
			CreatedAt:    now,
			LastTryAt:    now,
			IsFolder:     true,
			ZipMode:      true,
			TotalFiles:   len(folderInfo.Files),
			FolderFiles:  childFiles,
			folderFiles:  folderInfo.Files,
		}
		m.mu.Lock()
		m.items[id] = item
		if existingItem == nil || action != "overwrite" {
			m.order = append(m.order, id)
		}
		added = append(added, item)
		m.mu.Unlock()
		m.queueChan <- item
	} else {
		folderName := folderInfo.Title
		if action == "rename" {
			folderName = gdrive.UniqueFilename(targetFolder, folderName)
		}
		folderPath := filepath.Join(targetFolder, folderName)
		if action == "overwrite" {
			_ = os.RemoveAll(folderPath)
		}
		_ = os.MkdirAll(folderPath, 0755)

		m.mu.Lock()
		var folderChildItems []*DownloadItem
		for _, f := range folderInfo.Files {
			childPath := filepath.Join(folderPath, f.Filename)
			id := fmt.Sprintf("%d_%s", time.Now().UnixNano(), f.ID)
			status := StatusQueued
			var dlBytes int64 = 0
			var perc float64 = 0

			if action == "monitor" {
				if fi, statErr := os.Stat(childPath); statErr == nil && fi.Size() > 0 {
					status = StatusCompleted
					dlBytes = fi.Size()
					perc = 100
				}
			}

			item := &DownloadItem{
				ID:              id,
				URL:             fmt.Sprintf("https://drive.google.com/file/d/%s/view", f.ID),
				FileID:          f.ID,
				Filename:        f.Filename,
				TargetFolder:    folderPath,
				FolderTitle:     folderName,
				Status:          status,
				DownloadedBytes: dlBytes,
				TotalBytes:      dlBytes,
				Percentage:      perc,
				CreatedAt:       now,
				LastTryAt:       now,
				IsFolder:        false,
			}
			m.items[id] = item
			m.order = append(m.order, id)
			added = append(added, item)
			if status == StatusQueued {
				folderChildItems = append(folderChildItems, item)
			}
		}
		m.mu.Unlock()

		for _, item := range folderChildItems {
			m.queueChan <- item
		}
	}

	return added
}
