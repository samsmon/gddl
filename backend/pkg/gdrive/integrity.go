package gdrive

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// VerifyFileIntegrity checks if the downloaded file is complete and not corrupted.
func VerifyFileIntegrity(filePath string, expectedSize int64) error {
	fi, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("file not found on disk: %w", err)
	}

	actualSize := fi.Size()
	if actualSize == 0 {
		return fmt.Errorf("file is empty (0 bytes)")
	}

	// 1. Size Check: If expected size was reported by server, ensure exact match
	if expectedSize > 0 && actualSize != expectedSize {
		return fmt.Errorf("size mismatch: downloaded %d bytes, expected %d bytes (incomplete download)", actualSize, expectedSize)
	}

	// 2. HTML Error Check: Ensure it's not an HTML error response masquerading as media
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("cannot open file for integrity check: %w", err)
	}
	defer f.Close()

	header := make([]byte, 512)
	n, _ := f.Read(header)
	header = header[:n]

	headerLower := bytes.ToLower(header)
	if bytes.HasPrefix(headerLower, []byte("<!doctype html")) || bytes.HasPrefix(headerLower, []byte("<html")) {
		return fmt.Errorf("file is an HTML webpage, not a valid binary file (access denied or expired link)")
	}

	ext := strings.ToLower(filepath.Ext(filePath))

	// 3. ZIP File Validation (Checks central directory and headers)
	if ext == ".zip" {
		zr, err := zip.OpenReader(filePath)
		if err != nil {
			return fmt.Errorf("corrupted zip archive: %w", err)
		}
		defer zr.Close()

		if len(zr.File) == 0 {
			return fmt.Errorf("zip archive contains 0 files")
		}

		// Verify header reading for files
		for i, zf := range zr.File {
			if i > 20 { // Test first 20 file headers
				break
			}
			rc, err := zf.Open()
			if err != nil {
				return fmt.Errorf("corrupted zip entry '%s': %w", zf.Name, err)
			}
			rc.Close()
		}
	}

	// 4. RAR File Magic Bytes Check (Rar!....)
	if ext == ".rar" {
		if len(header) < 7 {
			return fmt.Errorf("file too small to be a valid RAR archive")
		}
		// RAR 4.x: 52 61 72 21 1A 07 00
		// RAR 5.x: 52 61 72 21 1A 07 01 00
		rar4 := []byte{0x52, 0x61, 0x72, 0x21, 0x1A, 0x07, 0x00}
		rar5 := []byte{0x52, 0x61, 0x72, 0x21, 0x1A, 0x07, 0x01, 0x00}
		if !bytes.HasPrefix(header, rar4) && !bytes.HasPrefix(header, rar5) {
			return fmt.Errorf("invalid RAR file header (corrupted or wrong format)")
		}
	}

	// 5. 7-Zip Magic Bytes Check (7z\xBC\xAF\x27\x1C)
	if ext == ".7z" {
		sevenZip := []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C}
		if !bytes.HasPrefix(header, sevenZip) {
			return fmt.Errorf("invalid 7z file header")
		}
	}

	// 6. Audio FLAC Magic Bytes Check (fLaC)
	if ext == ".flac" {
		if !bytes.HasPrefix(header, []byte("fLaC")) {
			return fmt.Errorf("invalid FLAC audio header (missing 'fLaC' marker)")
		}
	}

	// 7. Audio MP3 Magic Bytes Check (ID3 or sync frame \xFF\xFB)
	if ext == ".mp3" {
		if !bytes.HasPrefix(header, []byte("ID3")) && (len(header) < 2 || header[0] != 0xFF || (header[1]&0xE0) != 0xE0) {
			return fmt.Errorf("invalid MP3 audio header (missing ID3 or MPEG sync frame)")
		}
	}

	// 8. TAR.GZ / GZIP Check (\x1F\x8B)
	if ext == ".gz" || ext == ".tgz" {
		if len(header) < 2 || header[0] != 0x1F || header[1] != 0x8B {
			return fmt.Errorf("invalid GZIP header")
		}
	}

	return nil
}
