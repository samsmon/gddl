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
)

func (m *Manager) PrecheckDownloads(rawURLs []string, targetFolder string, zipMode bool) ([]ConflictInfo, error) {
	if targetFolder == "" {
		targetFolder = m.TargetFolder
	}

	var conflicts []ConflictInfo
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	for _, rawURL := range rawURLs {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			continue
		}

		var fileID string
		var isFolder bool
		var title string
		var expectedFilename string

		if discord.IsDiscordURL(rawURL) {
			isFolder = false
			dFilename, _, isExp, dErr := discord.ExtractDiscordFileInfo(rawURL)
			if dErr != nil {
				continue
			}
			_, attID, _, _ := discord.ExtractDiscordIDs(rawURL)
			if attID != "" {
				fileID = attID
			} else {
				fileID = dFilename
			}
			if isExp {
				title = dFilename + " (EXPIRED)"
			} else {
				title = dFilename
			}
			expectedFilename = dFilename

			// If not expired, try fast HEAD for exact Content-Disposition or size
			if !isExp {
				fn, _, headErr := m.discordDownloader.GetFileInfo(ctx, rawURL)
				if headErr == nil && fn != "" {
					expectedFilename = fn
					title = fn
				}
			}
		} else if folderID, isF := gdrive.IsFolderURL(rawURL); isF {
			isFolder = true
			fileID = folderID

			// Fast lookup in existing memory items
			m.mu.RLock()
			for _, it := range m.items {
				if it.FileID == folderID && it.FolderTitle != "" {
					title = it.FolderTitle
					break
				}
			}
			m.mu.RUnlock()

			if title == "" {
				info, err := m.downloader.FetchFolderInfo(ctx, folderID)
				if err == nil && info != nil && info.Title != "" {
					title = info.Title
				} else {
					title = fmt.Sprintf("Folder_%s", folderID)
				}
			}

			if zipMode {
				expectedFilename = title + ".zip"
			} else {
				expectedFilename = title
			}
		} else {
			fID, err := gdrive.ExtractFileID(rawURL)
			if err != nil {
				continue
			}
			fileID = fID

			// Fast lookup in memory items
			m.mu.RLock()
			for _, it := range m.items {
				if it.FileID == fileID && it.Filename != "" && !strings.HasPrefix(it.Filename, "File ") {
					expectedFilename = it.Filename
					title = it.Filename
					break
				}
			}
			m.mu.RUnlock()

			if expectedFilename == "" {
				if len(rawURLs) <= 1 {
					fn, _, err := m.downloader.GetFileInfo(ctx, fileID)
					if err == nil && fn != "" {
						expectedFilename = fn
						title = fn
					}
				}
				if expectedFilename == "" {
					expectedFilename = fmt.Sprintf("gdrive_%s.bin", fileID)
					title = fmt.Sprintf("Google Drive File [%s]", fileID)
				}
			}
		}

		targetPath := filepath.Join(targetFolder, expectedFilename)

		// Check if exists in transfer list
		var existsInList bool
		var existingID string
		var existingStatus string
		var existingBytes int64

		m.mu.RLock()
		for _, it := range m.items {
			if it.FileID == fileID || it.URL == rawURL {
				existsInList = true
				existingID = it.ID
				existingStatus = string(it.Status)
				existingBytes = it.DownloadedBytes
				break
			}
		}
		m.mu.RUnlock()

		// Check if exists on disk
		var existsOnDisk bool
		var diskSize int64
		var diskIsDir bool

		if fi, err := os.Stat(targetPath); err == nil {
			existsOnDisk = true
			diskSize = fi.Size()
			diskIsDir = fi.IsDir()
		}

		if existsInList || existsOnDisk {
			suggested := "monitor"
			if existingStatus == string(StatusDownloading) || existingStatus == string(StatusCompressing) {
				suggested = "rename"
			} else if existsOnDisk && (existingStatus == string(StatusCompleted) || existingStatus == string(StatusMissing) || !existsInList) {
				suggested = "monitor"
			} else if existsInList && !existsOnDisk {
				suggested = "overwrite"
			}

			conflicts = append(conflicts, ConflictInfo{
				URL:              rawURL,
				FileID:           fileID,
				IsFolder:         isFolder,
				Title:            title,
				ExpectedFilename: expectedFilename,
				TargetPath:       targetPath,
				ExistsInList:     existsInList,
				ExistingID:       existingID,
				ExistingStatus:   existingStatus,
				ExistingBytes:    existingBytes,
				ExistsOnDisk:     existsOnDisk,
				DiskSize:         diskSize,
				DiskIsDir:        diskIsDir,
				SuggestedAction:  suggested,
			})
		}
	}

	return conflicts, nil
}
