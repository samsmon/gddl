package queue

import (
	"strings"

	"gdrive-downloader/pkg/discord"
	"gdrive-downloader/pkg/logger"
)

type UnfinishedDiscordItem struct {
	ID              string `json:"id"`
	Filename        string `json:"filename"`
	URL             string `json:"url"`
	ChannelID       string `json:"channel_id,omitempty"`
	AttachmentID    string `json:"attachment_id,omitempty"`
	Status          string `json:"status"`
	DownloadedBytes int64  `json:"downloaded_bytes"`
	TotalBytes      int64  `json:"total_bytes"`
	Error           string `json:"error,omitempty"`
}

// GetUnfinishedDiscordItems returns all Discord downloads that are not completed (failed, queued, paused, downloading).
func (m *Manager) GetUnfinishedDiscordItems() []UnfinishedDiscordItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []UnfinishedDiscordItem
	for _, id := range m.order {
		it, exists := m.items[id]
		if !exists {
			continue
		}

		it.mu.RLock()
		isDisc := discord.IsDiscordURL(it.URL)
		status := it.Status
		urlStr := it.URL
		filename := it.Filename
		dlBytes := it.DownloadedBytes
		totBytes := it.TotalBytes
		errMsg := it.Error
		it.mu.RUnlock()

		if !isDisc || status == StatusCompleted {
			continue
		}

		chID, attID, parsedFn, _ := discord.ExtractDiscordIDs(urlStr)
		if filename == "" {
			filename = parsedFn
		}

		result = append(result, UnfinishedDiscordItem{
			ID:              id,
			Filename:        filename,
			URL:             urlStr,
			ChannelID:       chID,
			AttachmentID:    attID,
			Status:          string(status),
			DownloadedBytes: dlBytes,
			TotalBytes:      totBytes,
			Error:           errMsg,
		})
	}

	return result
}

// BatchUpdateDiscordURLs updates Discord downloads with refreshed signed URLs, clearing errors and resuming downloads.
func (m *Manager) BatchUpdateDiscordURLs(newURLs []string) (int, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	updatedCount := 0
	notFoundCount := 0

	for _, rawURL := range newURLs {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			continue
		}

		chID, attID, filename, err := discord.ExtractDiscordIDs(rawURL)
		if err != nil {
			notFoundCount++
			continue
		}

		// Find matching item in queue
		var matchedItem *DownloadItem
		for _, it := range m.items {
			it.mu.RLock()
			itemURL := it.URL
			itemFn := it.Filename
			itemStatus := it.Status
			it.mu.RUnlock()

			// Skip completed items
			if itemStatus == StatusCompleted {
				continue
			}

			// Match by Attachment ID first
			if attID != "" && strings.Contains(itemURL, "/"+attID+"/") {
				matchedItem = it
				break
			}

			// Match by clean filename as fallback
			if filename != "" && strings.EqualFold(itemFn, filename) {
				matchedItem = it
				break
			}
		}

		if matchedItem != nil {
			matchedItem.mu.Lock()
			matchedItem.URL = rawURL
			if matchedItem.Filename == "" && filename != "" {
				matchedItem.Filename = filename
			}
			matchedItem.Error = ""
			wasInactive := matchedItem.Status == StatusFailed || matchedItem.Status == StatusPaused
			if wasInactive {
				matchedItem.Status = StatusQueued
			}
			matchedItem.mu.Unlock()

			if wasInactive {
				select {
				case m.queueChan <- matchedItem:
				default:
				}
			}

			logger.Infof("Queue", "Refreshed Discord URL for '%s' (Attachment ID: %s, Channel: %s)", matchedItem.Filename, attID, chID)
			updatedCount++
		} else {
			notFoundCount++
		}
	}

	if updatedCount > 0 {
		m.saveToDiskLocked()
		m.triggerBroadcast()
	}

	return updatedCount, notFoundCount, nil
}
