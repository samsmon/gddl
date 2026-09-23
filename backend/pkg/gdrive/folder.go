package gdrive

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type FolderFileInfo struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
}

type FolderInfo struct {
	FolderID string           `json:"folder_id"`
	Title    string           `json:"title"`
	Files    []FolderFileInfo `json:"files"`
}

var (
	folderRegexes = []*regexp.Regexp{
		regexp.MustCompile(`drive\.google\.com/drive/(?:u/\d+/)?folders/([a-zA-Z0-9_-]+)`),
		regexp.MustCompile(`drive\.google\.com/open\?id=([a-zA-Z0-9_-]+)&.*usp=drive_link`),
		regexp.MustCompile(`drive\.google\.com/folderview\?id=([a-zA-Z0-9_-]+)`),
	}

	titleRegex = regexp.MustCompile(`<title>([^<]+?)(?:\s*-\s*Google Drive)?</title>`)

	// Match aria-label="filename [tags]" followed by ssk='5:auSv138:id-...
	fileItemRegex = regexp.MustCompile(`aria-label="([^"]+?)(?:\s+(?:Image|Dibagikan|Shared|Archive|Folder|Document|PDF|Video|Audio))*"\s+data-handled-by-drag-and-drop="true"\s+ssk='5:auSv138:([a-zA-Z0-9_-]{25,})-`)

	stripIndexSuffix = regexp.MustCompile(`-\d+$`)
)

func IsFolderURL(rawURL string) (string, bool) {
	clean := strings.TrimSpace(rawURL)
	for _, rgx := range folderRegexes {
		m := rgx.FindStringSubmatch(clean)
		if len(m) > 1 {
			return m[1], true
		}
	}
	return "", false
}

func (d *Downloader) FetchFolderInfo(ctx context.Context, folderID string) (*FolderInfo, error) {
	url := fmt.Sprintf("https://drive.google.com/drive/folders/%s", folderID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	d.cookieLock.RLock()
	cookie := d.googleCookie
	d.cookieLock.RUnlock()
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch folder: HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	body := string(bodyBytes)

	// Extract title
	folderTitle := "Google Drive Folder"
	if tm := titleRegex.FindStringSubmatch(body); len(tm) > 1 {
		folderTitle = strings.TrimSpace(html.UnescapeString(tm[1]))
		folderTitle = strings.TrimSuffix(folderTitle, " - Google Drive")
		folderTitle = sanitizeFilename(folderTitle)
	}

	// Extract files
	seen := make(map[string]bool)
	var files []FolderFileInfo

	matches := fileItemRegex.FindAllStringSubmatch(body, -1)
	for _, m := range matches {
		if len(m) > 2 {
			name := strings.TrimSpace(html.UnescapeString(m[1]))
			id := strings.TrimSpace(m[2])
			id = stripIndexSuffix.ReplaceAllString(id, "")
			if id != "" && name != "" && !seen[id] {
				seen[id] = true
				files = append(files, FolderFileInfo{
					ID:       id,
					Filename: sanitizeFilename(name),
				})
			}
		}
	}

	// Fallback check row by row if none found with fileItemRegex
	if len(files) == 0 {
		rowRgx := regexp.MustCompile(`<tr[^>]*data-id="([a-zA-Z0-9_-]{25,})"[^>]*>([\s\S]*?)<\/tr>`)
		nameRgx := regexp.MustCompile(`<strong class="DNoYtb">([^<]+)<\/strong>`)
		for _, r := range rowRgx.FindAllStringSubmatch(body, -1) {
			if len(r) > 2 {
				id := stripIndexSuffix.ReplaceAllString(strings.TrimSpace(r[1]), "")
				if nm := nameRgx.FindStringSubmatch(r[2]); len(nm) > 1 {
					name := strings.TrimSpace(html.UnescapeString(nm[1]))
					if id != "" && name != "" && !seen[id] {
						seen[id] = true
						files = append(files, FolderFileInfo{
							ID:       id,
							Filename: sanitizeFilename(name),
						})
					}
				}
			}
		}
	}

	return &FolderInfo{
		FolderID: folderID,
		Title:    folderTitle,
		Files:    files,
	}, nil
}

func CompressFolderToZip(ctx context.Context, srcDir, zipPath string, onProgress func(current, total int, filename string)) error {
	var filesToZip []string
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if ctx != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if !info.IsDir() {
			filesToZip = append(filesToZip, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	total := len(filesToZip)
	if total == 0 {
		return errors.New("no files to compress in directory")
	}

	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for idx, filePath := range filesToZip {
		if ctx != nil && ctx.Err() != nil {
			zipWriter.Close()
			zipFile.Close()
			os.Remove(zipPath)
			return ctx.Err()
		}

		relPath, err := filepath.Rel(srcDir, filePath)
		if err != nil {
			relPath = filepath.Base(filePath)
		}

		if onProgress != nil {
			onProgress(idx+1, total, relPath)
		}

		f, err := os.Open(filePath)
		if err != nil {
			return err
		}

		fi, err := f.Stat()
		if err != nil {
			f.Close()
			return err
		}

		header, err := zip.FileInfoHeader(fi)
		if err != nil {
			f.Close()
			return err
		}

		header.Name = filepath.ToSlash(relPath)
		header.Method = zip.Deflate

		w, err := zipWriter.CreateHeader(header)
		if err != nil {
			f.Close()
			return err
		}

		buf := make([]byte, 1024*1024)
		_, err = io.CopyBuffer(w, f, buf)
		f.Close()
		if err != nil {
			return err
		}
	}

	return zipWriter.Close()
}
