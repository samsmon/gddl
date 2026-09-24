package queue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gdrive-downloader/pkg/discord"
	"gdrive-downloader/pkg/gdrive"
	"gdrive-downloader/pkg/logger"
)

func (m *Manager) Add(rawURLs []string, targetFolder string, zipMode bool) ([]*DownloadItem, error) {
	return m.AddWithResolutions(rawURLs, targetFolder, zipMode, nil)
}

func (m *Manager) AddWithResolutions(rawURLs []string, targetFolder string, zipMode bool, resolutions map[string]string) ([]*DownloadItem, error) {
	if targetFolder == "" {
		targetFolder = m.TargetFolder
	}

	var added []*DownloadItem
	now := time.Now()

	for _, rawURL := range rawURLs {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			continue
		}

		// Check if it is a Google Drive folder
		if folderID, isFolder := gdrive.IsFolderURL(rawURL); isFolder {
			action := ""
			if resolutions != nil {
				action = resolutions[folderID]
				if action == "" {
					action = resolutions[rawURL]
				}
			}
			added = append(added, m.addFolderURL(rawURL, folderID, targetFolder, zipMode, action, now)...)
			continue
		}

		// Check if Discord URL
		isDiscord := discord.IsDiscordURL(rawURL)
		var fileID string
		var fn string

		if isDiscord {
			dFn, _, isExp, dErr := discord.ExtractDiscordFileInfo(rawURL)
			if dErr != nil {
				continue
			}
			_, attID, _, _ := discord.ExtractDiscordIDs(rawURL)
			if attID != "" {
				fileID = attID
			} else {
				fileID = dFn
			}
			fn = dFn
			if isExp {
				id := fmt.Sprintf("%d_discord_%s", time.Now().UnixNano(), fileID)
				item := &DownloadItem{
					ID:           id,
					URL:          rawURL,
					FileID:       fileID,
					Filename:     fn,
					TargetFolder: targetFolder,
					Status:       StatusFailed,
					Error:        "Discord CDN link has expired. Please copy a new link from Discord.",
					CreatedAt:    now,
					LastTryAt:    now,
				}
				m.mu.Lock()
				m.items[id] = item
				m.order = append(m.order, id)
				added = append(added, item)
				m.mu.Unlock()
				continue
			}
		} else {
			// Single Google Drive file link
			fID, err := gdrive.ExtractFileID(rawURL)
			if err != nil {
				continue
			}
			fileID = fID
		}

		action := ""
		if resolutions != nil {
			action = resolutions[fileID]
			if action == "" {
				action = resolutions[rawURL]
			}
		}

		var existingItem *DownloadItem
		m.mu.RLock()
		for _, it := range m.items {
			if it.FileID == fileID || it.URL == rawURL {
				existingItem = it
				break
			}
		}
		m.mu.RUnlock()

		if !isDiscord {
			fn = fmt.Sprintf("Google Drive File [%s]", fileID)
			if existingItem != nil && existingItem.Filename != "" && !strings.HasPrefix(existingItem.Filename, "Google Drive File [") {
				fn = existingItem.Filename
			} else if len(rawURLs) <= 1 {
				realFn, _, err := m.downloader.GetFileInfo(context.Background(), fileID)
				if err == nil && realFn != "" {
					fn = realFn
				}
			}
		} else {
			if existingItem != nil && existingItem.Filename != "" {
				fn = existingItem.Filename
			}
		}

		if action == "rename" || (action == "" && existingItem == nil) {
			m.mu.Lock()
			fn = m.getUniqueFilename(targetFolder, fn, "")
			m.mu.Unlock()
		} else if action == "overwrite" {
			if existingItem != nil {
				existingItem.mu.Lock()
				if existingItem.cancelFunc != nil {
					existingItem.cancelFunc()
				}
				existingItem.mu.Unlock()
			}
			_ = os.Remove(filepath.Join(targetFolder, fn))
		} else if action == "monitor" {
			destPath := filepath.Join(targetFolder, fn)
			if fi, statErr := os.Stat(destPath); statErr == nil && fi.Size() > 0 {
				id := fmt.Sprintf("%d_%s", time.Now().UnixNano(), fileID)
				if existingItem != nil {
					id = existingItem.ID
				}

				var realFn string
				var remoteSize int64
				var err error

				if isDiscord {
					realFn, remoteSize, err = m.discordDownloader.GetFileInfo(context.Background(), rawURL)
				} else {
					realFn, remoteSize, err = m.downloader.GetFileInfo(context.Background(), fileID)
				}

				if err == nil && realFn != "" {
					fn = realFn
				}

				status := StatusCompleted
				var perc float64 = 100
				errMsg := ""

				if remoteSize > 0 && fi.Size() < remoteSize {
					status = StatusQueued
					perc = float64(fi.Size()) / float64(remoteSize) * 100.0
					logger.Infof("Monitor", "File '%s' is partial (%d / %d bytes). Resuming download...", fn, fi.Size(), remoteSize)
				} else {
					logger.Successf("Monitor", "Verified single file '%s' (%d bytes) is complete", fn, fi.Size())
				}

				item := &DownloadItem{
					ID:              id,
					URL:             rawURL,
					FileID:          fileID,
					Filename:        fn,
					TargetFolder:    targetFolder,
					DownloadedBytes: fi.Size(),
					TotalBytes:      remoteSize,
					Percentage:      perc,
					Status:          status,
					Error:           errMsg,
					CreatedAt:       now,
					LastTryAt:       now,
				}

				m.mu.Lock()
				m.items[id] = item
				if existingItem == nil {
					m.order = append(m.order, id)
				}
				added = append(added, item)
				m.mu.Unlock()

				if status == StatusQueued {
					m.queueChan <- item
				}
				continue
			}
		}

		id := fmt.Sprintf("%d_%s", time.Now().UnixNano(), fileID)
		if action == "overwrite" && existingItem != nil {
			id = existingItem.ID
		}
		item := &DownloadItem{
			ID:           id,
			URL:          rawURL,
			FileID:       fileID,
			Filename:     fn,
			TargetFolder: targetFolder,
			Status:       StatusQueued,
			CreatedAt:    now,
			LastTryAt:    now,
		}

		m.mu.Lock()
		m.items[id] = item
		if existingItem == nil || action != "overwrite" {
			m.order = append(m.order, id)
		}
		added = append(added, item)
		m.mu.Unlock()

		m.queueChan <- item
	}

	m.triggerBroadcast()
	m.saveToDisk()
	return added, nil
}
