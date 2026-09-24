package gdrive

import (
	"context"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

var (
	filePatterns = []*regexp.Regexp{
		regexp.MustCompile(`drive\.usercontent\.google\.com/download\?.*[?&]id=([a-zA-Z0-9_-]+)`),
		regexp.MustCompile(`/file/d/([a-zA-Z0-9_-]+)`),
		regexp.MustCompile(`[?&]id=([a-zA-Z0-9_-]+)`),
		regexp.MustCompile(`/open\?id=([a-zA-Z0-9_-]+)`),
		regexp.MustCompile(`/uc\?id=([a-zA-Z0-9_-]+)`),
		regexp.MustCompile(`/drive/folders/([a-zA-Z0-9_-]+)`),
	}

	rawIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{20,}$`)

	formActionPattern   = regexp.MustCompile(`id="download-form"\s+action="([^"]+)"`)
	inputFieldPattern1  = regexp.MustCompile(`<input[^>]+name="([^"]+)"[^>]+value="([^"]*)"`)
	inputFieldPattern2  = regexp.MustCompile(`<input[^>]+value="([^"]*)"[^>]+name="([^"]+)"`)
	htmlFilenamePattern = regexp.MustCompile(`<span class="uc-name-size">\s*<a[^>]*>([^<]+)</a>`)
)

func ExtractFileID(rawURL string) (string, error) {
	clean := strings.TrimSpace(rawURL)
	if rawIDPattern.MatchString(clean) {
		return clean, nil
	}

	for _, pattern := range filePatterns {
		matches := pattern.FindStringSubmatch(clean)
		if len(matches) > 1 {
			return matches[1], nil
		}
	}

	return "", fmt.Errorf("could not extract Google Drive file ID from: %s", rawURL)
}

func parseConfirmationHTML(bodyStr, fileID string) (nextURL string, fallbackFilename string) {
	if fnMatches := htmlFilenamePattern.FindStringSubmatch(bodyStr); len(fnMatches) > 1 {
		fallbackFilename = html.UnescapeString(fnMatches[1])
	}

	formAction := "https://drive.usercontent.google.com/download"
	if actionMatches := formActionPattern.FindStringSubmatch(bodyStr); len(actionMatches) > 1 {
		formAction = actionMatches[1]
		if strings.HasPrefix(formAction, "/") {
			formAction = "https://drive.google.com" + formAction
		}
	}

	queryParams := url.Values{}
	for _, match := range inputFieldPattern1.FindAllStringSubmatch(bodyStr, -1) {
		if len(match) > 2 && match[1] != "" {
			queryParams.Set(match[1], match[2])
		}
	}
	for _, match := range inputFieldPattern2.FindAllStringSubmatch(bodyStr, -1) {
		if len(match) > 2 && match[2] != "" && queryParams.Get(match[2]) == "" {
			queryParams.Set(match[2], match[1])
		}
	}

	if queryParams.Get("id") == "" {
		queryParams.Set("id", fileID)
	}
	if queryParams.Get("export") == "" {
		queryParams.Set("export", "download")
	}
	if queryParams.Get("confirm") == "" {
		queryParams.Set("confirm", "t")
	}

	return fmt.Sprintf("%s?%s", formAction, queryParams.Encode()), fallbackFilename
}

func extractFilename(disposition string) string {
	if disposition == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(disposition)
	if err == nil {
		if fn, ok := params["filename*"]; ok {
			parts := strings.SplitN(fn, "''", 2)
			if len(parts) == 2 {
				decoded, err := url.PathUnescape(parts[1])
				if err == nil {
					return decoded
				}
			}
		}
		if fn, ok := params["filename"]; ok {
			return fn
		}
	}
	return ""
}

func SanitizeFilename(name string) string {
	invalid := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	clean := invalid.ReplaceAllString(name, "_")
	return strings.TrimSpace(clean)
}

func sanitizeFilename(name string) string {
	return SanitizeFilename(name)
}

// UniqueFilePath generates a unique file path with extra suffix (2), (3), etc.
func UniqueFilePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	dir := filepath.Dir(path)
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)

	counter := 2
	for {
		newPath := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, counter, ext))
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
		counter++
	}
}

func uniqueFilePath(path string) string {
	return UniqueFilePath(path)
}

// UniqueFilename returns a unique filename within dir.
func UniqueFilename(dir, filename string) string {
	return filepath.Base(UniqueFilePath(filepath.Join(dir, filename)))
}

// GetFileInfo inspects a Google Drive fileID and discovers its filename and size.
func (d *Downloader) GetFileInfo(ctx context.Context, fileID string) (string, int64, error) {
	initialURL := fmt.Sprintf("https://drive.usercontent.google.com/download?id=%s&export=download&authuser=0&confirm=t", fileID)
	req, err := http.NewRequestWithContext(ctx, "GET", initialURL, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	if cookie := d.GetGoogleCookie(); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	var filename string
	var totalSize int64 = resp.ContentLength
	if totalSize < 0 {
		totalSize = 0
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*64))
		bodyStr := string(bodyBytes)
		if fnMatches := htmlFilenamePattern.FindStringSubmatch(bodyStr); len(fnMatches) > 1 {
			filename = html.UnescapeString(fnMatches[1])
		}
	} else {
		filename = extractFilename(resp.Header.Get("Content-Disposition"))
	}

	if filename == "" {
		filename = fmt.Sprintf("gdrive_%s.bin", fileID)
	}
	filename = SanitizeFilename(filename)
	return filename, totalSize, nil
}
