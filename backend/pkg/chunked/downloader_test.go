package chunked

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

func createRangeServer(t *testing.T, data []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if !strings.HasPrefix(rangeHeader, "bytes=") {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}

		parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
		if len(parts) != 2 {
			http.Error(w, "invalid range", http.StatusBadRequest)
			return
		}

		start, _ := strconv.ParseInt(parts[0], 10, 64)
		end, _ := strconv.ParseInt(parts[1], 10, 64)
		if end >= int64(len(data)) {
			end = int64(len(data) - 1)
		}

		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(data[start : end+1])
	}))
}

func TestDownloader_DownloadSegmented_Fresh(t *testing.T) {
	testData := make([]byte, 4*1024*1024)
	if _, err := rand.Read(testData); err != nil {
		t.Fatalf("failed generating test data: %v", err)
	}

	server := createRangeServer(t, testData)
	defer server.Close()

	destDir := t.TempDir()
	destFile := filepath.Join(destDir, "fresh.bin")

	cd := NewDownloader(server.Client())
	totalSize := int64(len(testData))

	downloaded, err := cd.DownloadSegmented(
		context.Background(),
		destFile,
		server.URL,
		totalSize,
		http.Header{},
		4,
		nil,
	)

	if err != nil {
		t.Fatalf("DownloadSegmented failed: %v", err)
	}

	if downloaded != totalSize {
		t.Errorf("downloaded = %d; want %d", downloaded, totalSize)
	}

	savedBytes, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("failed reading saved file: %v", err)
	}

	if !bytes.Equal(savedBytes, testData) {
		t.Errorf("saved file content does not match original test data byte-for-byte!")
	}

	if HasChunkMeta(destFile) {
		t.Errorf("expected chunk metadata to be cleaned up after completion")
	}
}

func TestDownloader_DownloadSegmented_ResumeFromPartialFile(t *testing.T) {
	testData := make([]byte, 4*1024*1024)
	if _, err := rand.Read(testData); err != nil {
		t.Fatalf("failed generating test data: %v", err)
	}

	server := createRangeServer(t, testData)
	defer server.Close()

	destDir := t.TempDir()
	destFile := filepath.Join(destDir, "resume_file.bin")

	// Pre-write first 1MB to simulate interrupted single stream download
	prewrittenSize := 1024 * 1024
	if err := os.WriteFile(destFile, testData[:prewrittenSize], 0644); err != nil {
		t.Fatalf("failed writing partial file: %v", err)
	}

	cd := NewDownloader(server.Client())
	totalSize := int64(len(testData))

	downloaded, err := cd.DownloadSegmented(
		context.Background(),
		destFile,
		server.URL,
		totalSize,
		http.Header{},
		4,
		nil,
	)

	if err != nil {
		t.Fatalf("DownloadSegmented resume failed: %v", err)
	}

	if downloaded != totalSize {
		t.Errorf("downloaded = %d; want %d", downloaded, totalSize)
	}

	savedBytes, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("failed reading saved file: %v", err)
	}

	if !bytes.Equal(savedBytes, testData) {
		t.Errorf("saved file content does not match original test data byte-for-byte!")
	}
}

func TestDownloader_DownloadSegmented_ResumeFromMeta(t *testing.T) {
	testData := make([]byte, 4*1024*1024)
	if _, err := rand.Read(testData); err != nil {
		t.Fatalf("failed generating test data: %v", err)
	}

	server := createRangeServer(t, testData)
	defer server.Close()

	destDir := t.TempDir()
	destFile := filepath.Join(destDir, "resume_meta.bin")

	// Create file truncated to full size
	f, err := os.Create(destFile)
	if err != nil {
		t.Fatalf("failed creating destFile: %v", err)
	}
	_ = f.Truncate(int64(len(testData)))

	// Write chunk 0 completely (0..1MB-1) and chunk 1 halfway (1MB..1.5MB)
	_, _ = f.WriteAt(testData[0:1024*1024], 0)
	_, _ = f.WriteAt(testData[1024*1024:1536*1024], 1024*1024)
	f.Close()

	// Write mock chunk metadata
	chunk1MB := int64(1024 * 1024)
	segments := []*ChunkSegment{
		{Index: 0, Start: 0, End: chunk1MB - 1},
		{Index: 1, Start: chunk1MB, End: 2*chunk1MB - 1},
		{Index: 2, Start: 2 * chunk1MB, End: 3*chunk1MB - 1},
		{Index: 3, Start: 3 * chunk1MB, End: int64(len(testData)) - 1},
	}
	segments[0].Current.Store(chunk1MB)       // 100% complete
	segments[1].Current.Store(1536 * 1024)    // 50% complete
	segments[2].Current.Store(2 * chunk1MB)   // 0% complete
	segments[3].Current.Store(3 * chunk1MB)   // 0% complete

	if err := saveChunkState(destFile, int64(len(testData)), segments); err != nil {
		t.Fatalf("failed saving mock chunk state: %v", err)
	}

	cd := NewDownloader(server.Client())
	totalSize := int64(len(testData))

	downloaded, err := cd.DownloadSegmented(
		context.Background(),
		destFile,
		server.URL,
		totalSize,
		http.Header{},
		4,
		nil,
	)

	if err != nil {
		t.Fatalf("DownloadSegmented meta resume failed: %v", err)
	}

	if downloaded != totalSize {
		t.Errorf("downloaded = %d; want %d", downloaded, totalSize)
	}

	savedBytes, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("failed reading saved file: %v", err)
	}

	if !bytes.Equal(savedBytes, testData) {
		t.Errorf("saved file content does not match original test data byte-for-byte!")
	}

	if HasChunkMeta(destFile) {
		t.Errorf("expected chunk metadata to be cleaned up after completion")
	}
}

