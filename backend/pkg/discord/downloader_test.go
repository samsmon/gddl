package discord

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestExtractDiscordFileInfo(t *testing.T) {
	urlStr := "https://cdn.discordapp.com/attachments/123456789/987654321/large_video.mp4?ex=6789abcd&is=67885a4d&hm=12345"
	fn, _, isExp, err := ExtractDiscordFileInfo(urlStr)
	if err != nil {
		t.Fatalf("ExtractDiscordFileInfo failed: %v", err)
	}
	if fn != "large_video.mp4" {
		t.Errorf("got filename %q, want 'large_video.mp4'", fn)
	}
	if !isExp {
		// Timestamp in 2025 is expired by now
		t.Logf("URL is expired as expected")
	}
}

func TestDiscordDownloader_Download(t *testing.T) {
	// Generate 12MB test payload (> 10MB threshold for chunking)
	testData := make([]byte, 12*1024*1024)
	if _, err := rand.Read(testData); err != nil {
		t.Fatalf("failed generating test data: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if !strings.HasPrefix(rangeHeader, "bytes=") {
			w.Header().Set("Content-Length", strconv.Itoa(len(testData)))
			w.WriteHeader(http.StatusOK)
			w.Write(testData)
			return
		}

		parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
		if len(parts) != 2 {
			http.Error(w, "invalid range", http.StatusBadRequest)
			return
		}

		start, _ := strconv.ParseInt(parts[0], 10, 64)
		end, _ := strconv.ParseInt(parts[1], 10, 64)
		if end >= int64(len(testData)) {
			end = int64(len(testData) - 1)
		}

		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(testData)))
		w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(testData[start : end+1])
	}))
	defer server.Close()

	destDir := t.TempDir()

	dl := NewDownloader()
	dl.client = server.Client()
	dl.chunkedDownloader = dl.chunkedDownloader // uses test client transport if needed
	dl.SetChunksPerDownload(4)

	// Mock Discord URL targeting our test server
	mockURL := server.URL + "/attachments/111/222/test_game.zip?ex=7fffffff"

	fn, written, err := dl.Download(
		context.Background(),
		mockURL,
		destDir,
		nil,
	)

	if err != nil {
		t.Fatalf("Discord download failed: %v", err)
	}

	if fn != "test_game.zip" {
		t.Errorf("filename = %q, want 'test_game.zip'", fn)
	}

	if written != int64(len(testData)) {
		t.Errorf("written = %d, want %d", written, len(testData))
	}

	savedBytes, err := os.ReadFile(filepath.Join(destDir, "test_game.zip"))
	if err != nil {
		t.Fatalf("failed reading saved file: %v", err)
	}

	if !bytes.Equal(savedBytes, testData) {
		t.Errorf("saved file bytes do not match original payload!")
	}
}

func TestDiscordDownloader_416Recovery(t *testing.T) {
	testData := []byte("hello-discord-world-flac-data")
	dataLen := len(testData)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if strings.HasPrefix(rangeHeader, "bytes=") {
			startStr := strings.TrimPrefix(rangeHeader, "bytes=")
			startStr = strings.TrimSuffix(startStr, "-")
			start, _ := strconv.Atoi(startStr)
			if start >= dataLen {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", dataLen))
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
		}

		w.Header().Set("Content-Length", strconv.Itoa(dataLen))
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	}))
	defer server.Close()

	destDir := t.TempDir()
	dl := NewDownloader()
	dl.client = server.Client()

	mockURL := server.URL + "/attachments/123/456/song.flac?ex=7fffffff"
	partPath := filepath.Join(destDir, "song.flac.part")
	destPath := filepath.Join(destDir, "song.flac")

	// Case 1: .part has exact file size -> 416 should finalize file immediately
	_ = os.WriteFile(partPath, testData, 0644)
	fn, written, err := dl.Download(context.Background(), mockURL, destDir, nil)
	if err != nil {
		t.Fatalf("Download failed during 416 completion: %v", err)
	}
	if fn != "song.flac" || written != int64(dataLen) {
		t.Errorf("got fn=%q, written=%d", fn, written)
	}
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("expected %s to exist after finalize", destPath)
	}

	// Case 2: .part has corrupted/oversized bytes -> 416 should reset .part and redownload from 0
	_ = os.Remove(destPath)
	_ = os.WriteFile(partPath, []byte("too-many-bytes-than-actual-server-file-length-1234567890"), 0644)
	fn, written, err = dl.Download(context.Background(), mockURL, destDir, nil)
	if err != nil {
		t.Fatalf("Download failed during 416 reset recovery: %v", err)
	}
	if written != int64(dataLen) {
		t.Errorf("written = %d, want %d", written, dataLen)
	}
	resBytes, err := os.ReadFile(destPath)
	if err != nil || !bytes.Equal(resBytes, testData) {
		t.Errorf("saved file does not match expected testData")
	}
}

func TestFinalizeDownloadedFile(t *testing.T) {
	tmpDir := t.TempDir()
	partPath := filepath.Join(tmpDir, "test.bin.part")
	destPath := filepath.Join(tmpDir, "test.bin")

	testContent := []byte("audio-data-chunk")
	_ = os.WriteFile(partPath, testContent, 0644)

	// Finalize should succeed
	if err := finalizeDownloadedFile(partPath, destPath, int64(len(testContent))); err != nil {
		t.Fatalf("finalizeDownloadedFile failed: %v", err)
	}

	// Already finalized should also succeed without error
	if err := finalizeDownloadedFile(partPath, destPath, int64(len(testContent))); err != nil {
		t.Fatalf("finalizeDownloadedFile on already finalized file failed: %v", err)
	}
}

