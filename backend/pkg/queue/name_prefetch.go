package queue

import (
	"context"
	"strings"
	"sync"

	"gdrive-downloader/pkg/discord"
	"gdrive-downloader/pkg/gdrive"
)

const namePrefetchWorkers = 8

// prefetchFileNames resolves real Google Drive filenames for every single-file
// link in parallel, so pasting many links shows real names immediately
// instead of "Google Drive File [id]" placeholders.
func (m *Manager) prefetchFileNames(ctx context.Context, rawURLs []string) map[string]string {
	seen := make(map[string]bool)
	var ids []string
	for _, rawURL := range rawURLs {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" || discord.IsDiscordURL(rawURL) {
			continue
		}
		if _, isFolder := gdrive.IsFolderURL(rawURL); isFolder {
			continue
		}
		fileID, err := gdrive.ExtractFileID(rawURL)
		if err != nil || seen[fileID] {
			continue
		}
		seen[fileID] = true
		ids = append(ids, fileID)
	}

	names := make(map[string]string, len(ids))
	var mu sync.Mutex
	var wg sync.WaitGroup
	jobs := make(chan string)
	for w := 0; w < namePrefetchWorkers && w < len(ids); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fileID := range jobs {
				fn, _, err := m.downloader.GetFileInfo(ctx, fileID)
				if err != nil || fn == "" || strings.HasPrefix(fn, "Google Drive File [") || strings.HasPrefix(fn, "gdrive_") {
					continue
				}
				mu.Lock()
				names[fileID] = fn
				mu.Unlock()
			}
		}()
	}
	for _, id := range ids {
		jobs <- id
	}
	close(jobs)
	wg.Wait()
	return names
}
