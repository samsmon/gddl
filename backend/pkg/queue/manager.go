package queue

import (
	"context"
	"sync"
	"time"

	"gdrive-downloader/pkg/discord"
	"gdrive-downloader/pkg/gdrive"
	"gdrive-downloader/pkg/warp"
)

type DownloadStatus string

const (
	StatusQueued      DownloadStatus = "queued"
	StatusDownloading DownloadStatus = "downloading"
	StatusCompressing DownloadStatus = "compressing"
	StatusMoving      DownloadStatus = "moving"
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
	Chunks          int            `json:"chunks,omitempty"`
	AutoRetryCount  int            `json:"auto_retry_count,omitempty"`

	// Folder support
	IsFolder            bool                    `json:"is_folder"`
	ZipMode             bool                    `json:"zip_mode"`
	FolderTitle         string                  `json:"folder_title,omitempty"`
	TotalFiles          int                     `json:"total_files,omitempty"`
	CompletedFiles      int                     `json:"completed_files,omitempty"`
	CurrentFile         string                  `json:"current_file,omitempty"`
	CompressionProgress float64                 `json:"compression_progress,omitempty"`
	MoveProgress        float64                 `json:"move_progress,omitempty"`
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
		MoveProgress:        it.MoveProgress,
		FolderFiles:         childFiles,
		Chunks:              it.Chunks,
		AutoRetryCount:      it.AutoRetryCount,
	}
}

type Manager struct {
	mu                sync.RWMutex
	items             map[string]*DownloadItem
	order             []string
	queueChan         chan *DownloadItem
	subscribers       map[chan []DownloadItem]bool
	subMu             sync.RWMutex
	notifyChan        chan struct{}
	downloader        *gdrive.Downloader
	discordDownloader *discord.Downloader
	warpController    *warp.Controller
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
		queueChan:         make(chan *DownloadItem, 100000),
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
	go m.speedWatchdogLoop()

	return m, nil
}

func (m *Manager) Downloader() *gdrive.Downloader {
	return m.downloader
}

func (m *Manager) DiscordDownloader() *discord.Downloader {
	return m.discordDownloader
}

func (m *Manager) triggerBroadcast() {
	select {
	case m.notifyChan <- struct{}{}:
	default:
	}
}

func (m *Manager) broadcasterLoop() {
	ticker := time.NewTicker(150 * time.Millisecond)
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
