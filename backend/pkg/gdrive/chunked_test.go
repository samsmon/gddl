package gdrive

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

func TestChunkedDownloader_DownloadSegmented(t *testing.T) {
	// Generate 4MB random test data
	testData := make([]byte, 4*1024*1024)
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
	destFile := filepath.Join(destDir, "output.bin")

	cd := NewChunkedDownloader(server.Client())
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
}
