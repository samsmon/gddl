package queue

import (
	"testing"
	"time"
)

func TestDiscordQueueIntegration(t *testing.T) {
	mgr := &Manager{
		items:       make(map[string]*DownloadItem),
		order:       make([]string, 0),
		queueChan:   make(chan *DownloadItem, 100),
		subscribers: make(map[chan []DownloadItem]bool),
	}

	oldURL1 := "https://cdn.discordapp.com/attachments/111222333/444555666/track01.flac?ex=66000000&is=65000000&hm=deadbeef"
	oldURL2 := "https://cdn.discordapp.com/attachments/111222333/777888999/track02.flac?ex=66000000&is=65000000&hm=deadbeef"
	gdriveURL := "https://drive.google.com/file/d/1abcdef123456/view"

	item1 := &DownloadItem{
		ID:        "item-1",
		URL:       oldURL1,
		Filename:  "track01.flac",
		Status:    StatusFailed,
		Error:     "Discord CDN attachment link has expired",
		CreatedAt: time.Now(),
	}

	item2 := &DownloadItem{
		ID:        "item-2",
		URL:       oldURL2,
		Filename:  "track02.flac",
		Status:    StatusCompleted, // Completed should not be returned
		CreatedAt: time.Now(),
	}

	item3 := &DownloadItem{
		ID:        "item-3",
		URL:       gdriveURL,
		Filename:  "archive.zip",
		Status:    StatusFailed, // GDrive item should not be returned by GetUnfinishedDiscordItems
		CreatedAt: time.Now(),
	}

	mgr.mu.Lock()
	mgr.items[item1.ID] = item1
	mgr.items[item2.ID] = item2
	mgr.items[item3.ID] = item3
	mgr.order = append(mgr.order, item1.ID, item2.ID, item3.ID)
	mgr.mu.Unlock()

	// 1. Test GetUnfinishedDiscordItems
	unfinished := mgr.GetUnfinishedDiscordItems()
	if len(unfinished) != 1 {
		t.Fatalf("expected 1 unfinished Discord item, got %d", len(unfinished))
	}

	if unfinished[0].ID != "item-1" {
		t.Errorf("expected item-1, got %s", unfinished[0].ID)
	}
	if unfinished[0].AttachmentID != "444555666" {
		t.Errorf("expected AttachmentID 444555666, got %s", unfinished[0].AttachmentID)
	}
	if unfinished[0].ChannelID != "111222333" {
		t.Errorf("expected ChannelID 111222333, got %s", unfinished[0].ChannelID)
	}
	if unfinished[0].Filename != "track01.flac" {
		t.Errorf("expected Filename track01.flac, got %s", unfinished[0].Filename)
	}

	// 2. Test BatchUpdateDiscordURLs
	freshURL1 := "https://cdn.discordapp.com/attachments/111222333/444555666/track01.flac?ex=7fffffff&is=66ffffff&hm=fresh123"
	unmatchedURL := "https://cdn.discordapp.com/attachments/999/888/other.rar?ex=7fffffff"

	updated, notFound, err := mgr.BatchUpdateDiscordURLs([]string{freshURL1, unmatchedURL})
	if err != nil {
		t.Fatalf("BatchUpdateDiscordURLs failed: %v", err)
	}

	if updated != 1 {
		t.Errorf("expected 1 updated, got %d", updated)
	}
	if notFound != 1 {
		t.Errorf("expected 1 not found, got %d", notFound)
	}

	item1.mu.RLock()
	defer item1.mu.RUnlock()

	if item1.URL != freshURL1 {
		t.Errorf("expected URL to be updated to %s, got %s", freshURL1, item1.URL)
	}
	if item1.Status != StatusQueued {
		t.Errorf("expected Status to be reset to queued, got %s", item1.Status)
	}
	if item1.Error != "" {
		t.Errorf("expected Error to be cleared, got %s", item1.Error)
	}

	// Check that item1 was pushed to queueChan
	select {
	case queuedItem := <-mgr.queueChan:
		if queuedItem.ID != item1.ID {
			t.Errorf("expected queuedItem ID to be %s, got %s", item1.ID, queuedItem.ID)
		}
	default:
		t.Errorf("expected item1 to be in queueChan, but channel was empty")
	}
}
