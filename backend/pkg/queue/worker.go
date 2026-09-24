package queue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gdrive-downloader/pkg/discord"
	"gdrive-downloader/pkg/logger"
	"gdrive-downloader/pkg/warp"
)

// SetWarpController attaches the WARP/Proxy controller to the manager and binds both GDrive and Discord downloaders.
func (m *Manager) SetWarpController(wc *warp.Controller) {
	m.mu.Lock()
	m.warpController = wc
	m.mu.Unlock()

	if wc == nil {
		return
	}

	onRateLimit := func(reason string) {
		if wc.IsAutoEnabled() {
			_, _ = wc.TriggerAutoBypassOrRotate(reason, false)
			m.triggerBroadcast()
		}
	}

	m.discordDownloader.BindProxyController(wc.RegisterTransport, wc.RotationEpoch, onRateLimit)
	m.downloader.BindProxyController(wc.RegisterTransport, wc.RotationEpoch, onRateLimit)
}

// WarpController returns the active WARP/Proxy controller.
func (m *Manager) WarpController() *warp.Controller {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.warpController
}

// speedWatchdogLoop continuously monitors total active download speed every second.
// If active transfers stay below the configured minimum speed threshold for 7 consecutive seconds,
// it automatically activates or rotates the WARP SOCKS5 proxy / custom proxy pool.
func (m *Manager) speedWatchdogLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lowSpeedSec := 0

	for range ticker.C {
		wc := m.WarpController()
		if wc == nil || !wc.IsAutoEnabled() {
			if lowSpeedSec > 0 {
				lowSpeedSec = 0
				if wc != nil {
					wc.SetLowSpeedDuration(0)
				}
			}
			continue
		}

		m.mu.RLock()
		var totalSpeed int64
		var matureDownloadingCount int
		now := time.Now()

		for _, it := range m.items {
			it.mu.RLock()
			if it.Status == StatusDownloading {
				totalSpeed += it.Speed
				// Give newly started downloads a 4-second grace period to establish TCP & ramp up
				if now.Sub(it.LastTryAt) >= 4*time.Second {
					matureDownloadingCount++
				}
			}
			it.mu.RUnlock()
		}
		m.mu.RUnlock()

		if matureDownloadingCount == 0 {
			if lowSpeedSec > 0 {
				lowSpeedSec = 0
				wc.SetLowSpeedDuration(0)
			}
			continue
		}

		minBytes := wc.MinSpeedBytesPerSec()
		if totalSpeed < minBytes {
			lowSpeedSec++
			wc.SetLowSpeedDuration(lowSpeedSec)

			if lowSpeedSec >= 7 {
				speedMB := float64(totalSpeed) / (1024.0 * 1024.0)
				minMB := float64(minBytes) / (1024.0 * 1024.0)
				reason := fmt.Sprintf("Speed %.2f MB/s < %.1f MB/s threshold for %ds", speedMB, minMB, lowSpeedSec)
				lowSpeedSec = 0
				wc.SetLowSpeedDuration(0)

				go func(r string) {
					if rotated, _ := wc.TriggerAutoBypassOrRotate(r, false); rotated {
						m.triggerBroadcast()
					}
				}(reason)
			}
		} else if lowSpeedSec > 0 {
			lowSpeedSec = 0
			wc.SetLowSpeedDuration(0)
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
		configuredChunks := m.downloader.GetChunksPerDownload()
		item.Status = StatusDownloading
		item.LastTryAt = time.Now()
		item.Error = ""
		item.Chunks = configuredChunks
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
		if item.Filename != "" && !strings.HasPrefix(item.Filename, "File ") && !strings.HasPrefix(item.Filename, "Folder ") && !strings.HasPrefix(item.Filename, "Google Drive File [") {
			desiredName = item.Filename
		}

		m.mu.Lock()
		if desiredName != "" {
			desiredName = m.getUniqueFilename(item.TargetFolder, desiredName, item.ID)
			item.mu.Lock()
			item.Filename = desiredName
			item.mu.Unlock()
		}
		m.mu.Unlock()

		displayName := item.Filename
		if displayName == "" {
			displayName = item.FileID
		}
		logger.Infof("Queue", "Starting download '%s'", displayName)

		var filename string
		var err error

		if discord.IsDiscordURL(item.URL) {
			item.mu.Lock()
			item.Chunks = configuredChunks
			item.mu.Unlock()
			m.discordDownloader.SetChunksPerDownload(configuredChunks)
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
					if totalBytes > 0 && totalBytes <= 10*1024*1024 {
						item.Chunks = 1
					} else {
						item.Chunks = configuredChunks
					}
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
					if totalBytes <= 10*1024*1024 {
						item.Chunks = 1
					} else {
						item.Chunks = configuredChunks
					}
					item.mu.Unlock()
				},
				desiredName,
			)
		}

		item.mu.Lock()
		item.cancelFunc = nil
		if filename != "" {
			displayName = filename
		}

		if err != nil {
			if strings.Contains(err.Error(), "context canceled") {
				if item.Status != StatusPaused {
					item.Status = StatusCancelled
				}
				logger.Warnf("Queue", "Download cancelled for '%s'", displayName)
			} else {
				isCorrupt := strings.HasPrefix(err.Error(), "CORRUPT:")
				isStreamErr := strings.Contains(err.Error(), "INTERNAL_ERROR") || strings.Contains(err.Error(), "stream error") || strings.Contains(err.Error(), "connection reset") || strings.Contains(err.Error(), "unexpected EOF")
				if item.AutoRetryCount < 1 && (isCorrupt || isStreamErr) && ctx.Err() == nil {
					item.AutoRetryCount++
					logger.Warnf("Queue", "Download '%s' encountered error: %v. Initiating automatic self-healing restart (attempt %d/1)...", displayName, err, item.AutoRetryCount)
					if item.Filename != "" {
						_ = os.Remove(filepath.Join(item.TargetFolder, item.Filename))
						_ = os.Remove(filepath.Join(item.TargetFolder, item.Filename+".part"))
						_ = os.Remove(filepath.Join(item.TargetFolder, item.Filename+".part.gddl-chunks"))
						_ = os.Remove(filepath.Join(item.TargetFolder, item.Filename+".gddl-chunks"))
					}
					item.Status = StatusQueued
					item.Error = ""
					item.Speed = 0
					item.DownloadedBytes = 0
					item.Percentage = 0
					item.LastTryAt = time.Now()
					item.mu.Unlock()
					m.queueChan <- item
					m.triggerBroadcast()
					m.saveToDisk()
					continue
				}

				if isCorrupt {
					item.Status = StatusCorrupted
					item.Error = strings.TrimSpace(strings.TrimPrefix(err.Error(), "CORRUPT:"))
					item.Filename = filename
					logger.Errorf("Integrity", "Download corrupted for '%s': %s", displayName, item.Error)
				} else {
					item.Status = StatusFailed
					item.Error = err.Error()
					logger.Errorf("Queue", "Download failed for '%s': %v", displayName, err)
				}
			}
			item.Speed = 0
			item.ETASeconds = 0
		} else {
			item.Status = StatusCompleted
			item.Filename = filename
			item.Percentage = 100
			item.Speed = 0
			item.ETASeconds = 0
			logger.Successf("Queue", "Finished downloading '%s'", filename)
		}
		item.mu.Unlock()
		m.triggerBroadcast()
		m.saveToDisk()
	}
}
