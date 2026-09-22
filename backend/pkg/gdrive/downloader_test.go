package gdrive

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractFileID(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantID  string
		wantErr bool
	}{
		{
			name:    "drive.usercontent.google.com with params",
			url:     "https://drive.usercontent.google.com/download?id=13g1RiKb1EVZYChQBgmGXwG_o0thaLZ9x&export=download&authuser=0",
			wantID:  "13g1RiKb1EVZYChQBgmGXwG_o0thaLZ9x",
			wantErr: false,
		},
		{
			name:    "standard file view URL",
			url:     "https://drive.google.com/file/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/view?usp=sharing",
			wantID:  "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms",
			wantErr: false,
		},
		{
			name:    "open id URL",
			url:     "https://drive.google.com/open?id=1AbCdEfGhIjKlMnOpQrStUvWxYz12345",
			wantID:  "1AbCdEfGhIjKlMnOpQrStUvWxYz12345",
			wantErr: false,
		},
		{
			name:    "uc export URL",
			url:     "https://drive.google.com/uc?id=1AbCdEfGhIjKlMnOpQrStUvWxYz12345&export=download",
			wantID:  "1AbCdEfGhIjKlMnOpQrStUvWxYz12345",
			wantErr: false,
		},
		{
			name:    "folders URL",
			url:     "https://drive.google.com/drive/folders/1AbCdEfGhIjKlMnOpQrStUvWxYz12345",
			wantID:  "1AbCdEfGhIjKlMnOpQrStUvWxYz12345",
			wantErr: false,
		},
		{
			name:    "raw valid ID string",
			url:     "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms",
			wantID:  "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms",
			wantErr: false,
		},
		{
			name:    "invalid URL",
			url:     "https://example.com/not-a-gdrive-link",
			wantID:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, err := ExtractFileID(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractFileID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotID != tt.wantID {
				t.Errorf("ExtractFileID() got = %v, want %v", gotID, tt.wantID)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal.mp3", "normal.mp3"},
		{"song:part1*<remix>?.flac", "song_part1__remix__.flac"},
		{"folder/name\\file.zip", "folder_name_file.zip"},
		{"[250326] 【アンティーカ盤】 (FLAC).zip", "[250326] 【アンティーカ盤】 (FLAC).zip"},
	}

	for _, tt := range tests {
		result := sanitizeFilename(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}

func TestVerifyFileIntegrity(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Empty file
	emptyFile := filepath.Join(tmpDir, "empty.zip")
	_ = os.WriteFile(emptyFile, []byte{}, 0644)
	if err := VerifyFileIntegrity(emptyFile, 0); err == nil {
		t.Errorf("expected error on empty file, got nil")
	}

	// 2. HTML Error masquerading as zip
	htmlFile := filepath.Join(tmpDir, "fake.zip")
	_ = os.WriteFile(htmlFile, []byte("<!DOCTYPE html><html><body>Error 403</body></html>"), 0644)
	if err := VerifyFileIntegrity(htmlFile, 0); err == nil {
		t.Errorf("expected error on HTML disguised as zip, got nil")
	}

	// 3. Size mismatch
	sizeFile := filepath.Join(tmpDir, "test.bin")
	_ = os.WriteFile(sizeFile, []byte("12345"), 0644)
	if err := VerifyFileIntegrity(sizeFile, 100); err == nil {
		t.Errorf("expected error on size mismatch, got nil")
	}

	// 4. Valid zip file
	validZip := filepath.Join(tmpDir, "valid.zip")
	zf, err := os.Create(validZip)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	w, _ := zw.Create("hello.txt")
	_, _ = w.Write([]byte("world"))
	zw.Close()
	zf.Close()

	stat, _ := os.Stat(validZip)
	if err := VerifyFileIntegrity(validZip, stat.Size()); err != nil {
		t.Errorf("expected valid zip to pass integrity check, got: %v", err)
	}

	// 5. Corrupted / truncated zip
	corruptZip := filepath.Join(tmpDir, "corrupt.zip")
	// Only write first 20 bytes of a zip (missing central directory at end)
	zipBytes, _ := os.ReadFile(validZip)
	_ = os.WriteFile(corruptZip, zipBytes[:len(zipBytes)/2], 0644)
	if err := VerifyFileIntegrity(corruptZip, 0); err == nil {
		t.Errorf("expected corrupted zip to fail integrity check, got nil")
	}
}
