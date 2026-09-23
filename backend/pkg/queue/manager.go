package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gdrive-downloader/pkg/discord"
	"gdrive-downloader/pkg/gdrive"
	"gdrive-downloader/pkg/logger"
)

type DownloadStatus string

const (
	StatusQueued      DownloadStatus = "queued"
	StatusDownloading DownloadStatus = "downloading"
	StatusCompressing DownloadStatus = "compressing"
	StatusPaused      DownloadStatus = "paused"
	StatusCompleted   DownloadStatus = "completed"
	StatusFailed      DownloadStatus = "failed"
	StatusCancelled   DownloadStatus = "cancelled"
	StatusCorrupted   DownloadStatus = "corrupted"
	StatusMissing     DownloadStatus = "missing"
)

type ChildFileItem struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Status   string `json:"status"` // "queued", "downloading", "completed", "failed"
	Size     int64  `json:"size,omitempty"`
}

type ConflictInfo struct {
	URL              string `json:"url"`
	FileID           string `json:"file_id"`
	IsFolder         bool   `json:"is_folder"`
	Title            string `json:"title"`
	ExpectedFilename string `json:"expected_filename"`
	TargetPath       string `json:"target_path"`
	ExistsInList     bool   `json:"exists_in_list"`
	ExistingID       string `json:"existing_id,omitempty"`
	ExistingStatus   string `json:"existing_status,omitempty"`
	ExistingBytes    int64  `json:"existing_bytes,omitempty"`
	ExistsOnDisk     bool   `json:"exists_on_disk"`
	DiskSize         int64  `json:"disk_size"`
	DiskIsDir        bool   `json:"disk_is_dir"`
	SuggestedAction  string `json:"suggested_action"` // "rename", "overwrite", or "monitor"
}

type DownloadItem struct {
	mu              sync.RWMutex
	ID              string         `json:"id"`
	URL             string         `json:"url"`
	FileID          string         `json:"file_id"`
	Filename        string         `json:"filename"`
	TargetFolder    string         `json:"target_folder"`
	DownloadedBytes int64          `json:"downloaded_bytes"`
	TotalBytes      int64          `json:"total_bytes"`
	Speed           int64          `json:"speed"` // bytes/sec
	ETASeconds      int64          `json:"eta_seconds"`
	Percentage      float64        `json:"percentage"`
	Status          DownloadStatus `json:"status"`
	Error           string         `json:"error,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	LastTryAt       time.Time      `json:"last_try_at"`

	// Folder support
	IsFolder            bool                    `json:"is_folder"`
	ZipMode             bool                    `json:"zip_mode"`
	FolderTitle         string                  `json:"folder_title,omitempty"`
	TotalFiles          int                     `json:"total_files,omitempty"`
	CompletedFiles      int                     `json:"completed_files,omitempty"`
	CurrentFile         string                  `json:"current_file,omitempty"`
	CompressionProgress float64                 `json:"compression_progress,omitempty"`
	FolderFiles         []ChildFileItem         `json:"folder_files,omitempty"`
	folderFiles         []gdrive.FolderFileInfo `json:"-"`

	cancelFunc context.CancelFunc `json:"-"`
}

func (it *DownloadItem) Snapshot() DownloadItem {
	it.mu.RLock()
	defer it.mu.RUnlock()

	var childFiles []ChildFileItem
	if len(it.FolderFiles) > 0 {
		childFiles = make([]ChildFileItem, len(it.FolderFiles))
		copy(childFiles, it.FolderFiles)
	}

	return DownloadItem{
		ID:                  it.ID,
		URL:                 it.URL,
		FileID:              it.FileID,
		Filename:            it.Filename,
		TargetFolder:        it.TargetFolder,
		DownloadedBytes:     it.DownloadedBytes,
		TotalBytes:          it.TotalBytes,
		Speed:               it.Speed,
		ETASeconds:          it.ETASeconds,
		Percentage:          it.Percentage,
		Status:              it.Status,
		Error:               it.Error,
		CreatedAt:           it.CreatedAt,
		LastTryAt:           it.LastTryAt,
		IsFolder:            it.IsFolder,
		ZipMode:             it.ZipMode,
		FolderTitle:         it.FolderTitle,
		TotalFiles:          it.TotalFiles,
		CompletedFiles:      it.CompletedFiles,
		CurrentFile:         it.CurrentFile,
		CompressionProgress: it.CompressionProgress,
		FolderFiles:         childFiles,
	}
}

type Manager struct {
	mu             sync.RWMutex
	items          map[string]*DownloadItem
	order          []string
	queueChan      chan *DownloadItem
	subscribers    map[chan []DownloadItem]bool
	subMu          sync.RWMutex
	notifyChan     chan struct{}
	downloader        *gdrive.Downloader
	discordDownloader *discord.Downloader
	MaxConcurrency    int
	TargetFolder      string
	dataFile          string
}

func NewManager(defaultFolder string, concurrency int, dataFile ...string) (*Manager, error) {
	dl, err := gdrive.NewDownloader()
	if err != nil {
		return nil, err
	}

	discordDl := discord.NewDownloader()

	if concurrency <= 0 {
		concurrency = 2
	}

	df := "downloads.json"
	if len(dataFile) > 0 && dataFile[0] != "" {
		df = dataFile[0]
	}

	m := &Manager{
		items:             make(map[string]*DownloadItem),
		order:             make([]string, 0),
		queueChan:         make(chan *DownloadItem, 2000),
		subscribers:       make(map[chan []DownloadItem]bool),
		notifyChan:        make(chan struct{}, 1),
		downloader:        dl,
		discordDownloader: discordDl,
		MaxConcurrency:    concurrency,
		TargetFolder:      defaultFolder,
		dataFile:          df,
	}

	m.loadFromDisk()
	m.CheckAllFiles()

	for i := 0; i < concurrency; i++ {
		go m.worker()
	}

	go m.broadcasterLoop()

	return m, nil
}

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

func (m *Manager) Downloader() *gdrive.Downloader {
	return m.downloader
}

func (m *Manager) triggerBroadcast() {
	select {
	case m.notifyChan <- struct{}{}:
	default:
	}
}

func (m *Manager) broadcasterLoop() {
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.broadcast()
		case <-m.notifyChan:
			m.broadcast()
		}
	}
}

func (m *Manager) worker() {
	for item := range m.queueChan {
		item.mu.Lock()
		if item.Status == StatusCancelled || item.Status == StatusPaused {
			item.mu.Unlock()
			continue
		}
		item.Status = StatusDownloading
		item.LastTryAt = time.Now()
		item.Error = ""
		ctx, cancel := context.WithCancel(context.Background())
		item.cancelFunc = cancel
		isFolderZip := item.IsFolder && item.ZipMode
		item.mu.Unlock()
		m.triggerBroadcast()

		if isFolderZip {
			m.downloadFolderZip(ctx, item)
			continue
		}

		var desiredName string
		if item.Filename != "" && !strings.HasPrefix(item.Filename, "File ") && !strings.HasPrefix(item.Filename, "Folder ") {
			desiredName = item.Filename
		}

		var filename string
		var err error

		if discord.IsDiscordURL(item.URL) {
			filename, _, err = m.discordDownloader.Download(
				ctx,
				item.URL,
				item.TargetFolder,
				func(downloadedBytes, totalBytes, speed int64, etaSeconds int64, percentage float64) {
					item.mu.Lock()
					item.DownloadedBytes = downloadedBytes
					item.TotalBytes = totalBytes
					item.Speed = speed
					item.ETASeconds = etaSeconds
					item.Percentage = percentage
					item.mu.Unlock()
				},
				desiredName,
			)
		} else {
			filename, _, err = m.downloader.Download(
				ctx,
				item.FileID,
				item.TargetFolder,
				func(downloadedBytes, totalBytes, speed int64, etaSeconds int64, percentage float64) {
					item.mu.Lock()
					item.DownloadedBytes = downloadedBytes
					item.TotalBytes = totalBytes
					item.Speed = speed
					item.ETASeconds = etaSeconds
					item.Percentage = percentage
					item.mu.Unlock()
				},
				desiredName,
			)
		}

		item.mu.Lock()
		item.cancelFunc = nil
		if err != nil {
			if strings.Contains(err.Error(), "context canceled") {
				// Check if user paused or cancelled
				if item.Status != StatusPaused {
					item.Status = StatusCancelled
				}
			} else if strings.HasPrefix(err.Error(), "CORRUPT:") {
				item.Status = StatusCorrupted
				item.Error = strings.TrimSpace(strings.TrimPrefix(err.Error(), "CORRUPT:"))
				item.Filename = filename
			} else {
				item.Status = StatusFailed
				item.Error = err.Error()
			}
			item.Speed = 0
			item.ETASeconds = 0
		} else {
			item.Status = StatusCompleted
			item.Filename = filename
			item.Percentage = 100
			item.Speed = 0
			item.ETASeconds = 0
		}
		item.mu.Unlock()
		m.triggerBroadcast()
		m.saveToDisk()
	}
}

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
			childFiles[i] = ChildFileItem{
				ID:       f.ID,
				Filename: f.Filename,
				Status:   "queued",
			}
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
		} else {
			// If missing or incomplete on disk, ensure status is queued
			if i < len(item.FolderFiles) && item.FolderFiles[i].Status != "completed" {
				item.FolderFiles[i].Status = "queued"
			}
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

		var downloadedForFile int64
		var written int64
		var dlErr error

		// Retry up to 2 times for transient network/Google errors
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

			if dlErr == nil {
				break
			}

			if ctx.Err() != nil || strings.Contains(dlErr.Error(), "context canceled") {
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
			fileID = dFilename
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
					fileID = fn
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
				fn, _, err := m.downloader.GetFileInfo(ctx, fileID)
				if err == nil && fn != "" {
					expectedFilename = fn
					title = fn
				} else {
					expectedFilename = fmt.Sprintf("gdrive_%s.bin", fileID)
					title = expectedFilename
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

			folderInfo, err := m.downloader.FetchFolderInfo(context.Background(), folderID)
			if err != nil {
				// Record item as failed with error
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
				continue
			}

			if zipMode {
				zipFilename := folderInfo.Title + ".zip"
				zipDstPath := filepath.Join(targetFolder, zipFilename)

				// Find existing item in list if any
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
						continue
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
				// Folder Mode: create local subfolder and enqueue child files
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
			fileID = dFn
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
			fn = fmt.Sprintf("File %s", fileID)
			if existingItem != nil && existingItem.Filename != "" && !strings.HasPrefix(existingItem.Filename, "File ") {
				fn = existingItem.Filename
			} else {
				realFn, _, err := m.downloader.GetFileInfo(context.Background(), fileID)
				if err == nil && realFn != "" {
					fn = realFn
				}
			}
		} else {
			if existingItem != nil && existingItem.Filename != "" {
				fn = existingItem.Filename
			} else {
				realFn, _, err := m.discordDownloader.GetFileInfo(context.Background(), rawURL)
				if err == nil && realFn != "" {
					fn = realFn
				}
			}
		}

		if action == "rename" {
			fn = gdrive.UniqueFilename(targetFolder, fn)
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

func (m *Manager) Pause(id string) error {
	m.mu.RLock()
	item, exists := m.items[id]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("item not found: %s", id)
	}

	item.mu.Lock()
	item.Status = StatusPaused
	if item.cancelFunc != nil {
		item.cancelFunc()
		item.cancelFunc = nil
	}
	item.Speed = 0
	item.ETASeconds = 0
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
	if item.Status == StatusDownloading || item.Status == StatusQueued || item.Status == StatusCompressing {
		item.mu.Unlock()
		return nil
	}
	item.Status = StatusQueued
	item.LastTryAt = time.Now()
	item.Error = ""
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
	item.DownloadedBytes = 0
	item.TotalBytes = 0
	item.Percentage = 0
	item.Speed = 0
	item.ETASeconds = 0
	item.CompletedFiles = 0
	item.CompressionProgress = 0
	item.CurrentFile = ""
	item.Error = ""
	item.Status = StatusQueued
	item.LastTryAt = time.Now()
	for i := range item.FolderFiles {
		item.FolderFiles[i].Status = "queued"
	}
	targetFolder := item.TargetFolder
	itemId := item.ID
	item.mu.Unlock()

	// Clean up staging directory so restart is completely clean from 0
	stagingDir := filepath.Join(targetFolder, fmt.Sprintf(".tmp_gdrive_%s", itemId))
	_ = os.RemoveAll(stagingDir)
	if item.Filename != "" {
		_ = os.Remove(filepath.Join(targetFolder, item.Filename+".part"))
	}

	logger.Infof("Queue", "Restarted download '%s' from beginning (staging cleared)", item.Filename)

	m.queueChan <- item
	m.triggerBroadcast()
	m.saveToDisk()
	return nil
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
	item.Speed = 0
	item.ETASeconds = 0
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
	targetFolder := item.TargetFolder
	filename := item.Filename
	isFolder := item.IsFolder
	zipMode := item.ZipMode
	item.mu.Unlock()

	// Clean up temporary staging directory if any
	stagingDir := filepath.Join(targetFolder, fmt.Sprintf(".tmp_gdrive_%s", id))
	_ = os.RemoveAll(stagingDir)

	if deleteFile {
		if isFolder && !zipMode {
			if targetFolder != "" && targetFolder != m.TargetFolder {
				_ = os.RemoveAll(targetFolder)
			}
		} else {
			if filename != "" && targetFolder != "" {
				filePath := filepath.Join(targetFolder, filename)
				_ = os.Remove(filePath)
				_ = os.Remove(filePath + ".part")
			}
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
			tgt := item.TargetFolder
			fn := item.Filename
			isFolder := item.IsFolder
			zipMode := item.ZipMode
			folderTitle := item.FolderTitle
			id := item.ID
			item.mu.RUnlock()

			if isFolder && zipMode {
				_ = os.Remove(filepath.Join(tgt, fn))
				_ = os.RemoveAll(filepath.Join(tgt, fmt.Sprintf(".tmp_gdrive_%s", id)))
			} else if isFolder && !zipMode && folderTitle != "" {
				subfolder := filepath.Join(tgt, folderTitle)
				if subfolder != tgt && subfolder != filepath.Dir(subfolder) {
					_ = os.RemoveAll(subfolder)
				}
			} else if fn != "" {
				_ = os.Remove(filepath.Join(tgt, fn))
			}
		}
	}

	m.triggerBroadcast()
}

func (m *Manager) GetList() []DownloadItem {
	m.mu.RLock()
	itemsCopy := make([]*DownloadItem, 0, len(m.order))
	for i := len(m.order) - 1; i >= 0; i-- {
		id := m.order[i]
		if it, ok := m.items[id]; ok {
			itemsCopy = append(itemsCopy, it)
		}
	}
	m.mu.RUnlock()

	result := make([]DownloadItem, 0, len(itemsCopy))
	for _, it := range itemsCopy {
		result = append(result, it.Snapshot())
	}
	return result
}

func (m *Manager) Subscribe() (chan []DownloadItem, func()) {
	ch := make(chan []DownloadItem, 20)
	m.subMu.Lock()
	m.subscribers[ch] = true
	m.subMu.Unlock()

	// Initial snapshot
	ch <- m.GetList()

	unsubscribe := func() {
		m.subMu.Lock()
		delete(m.subscribers, ch)
		close(ch)
		m.subMu.Unlock()
	}

	return ch, unsubscribe
}

func (m *Manager) broadcast() {
	m.subMu.RLock()
	subCount := len(m.subscribers)
	m.subMu.RUnlock()

	if subCount == 0 {
		return
	}

	list := m.GetList()

	m.subMu.RLock()
	defer m.subMu.RUnlock()

	for ch := range m.subscribers {
		select {
		case ch <- list:
		default:
		}
	}
}
