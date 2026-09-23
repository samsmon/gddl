package gdrive

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gdrive-downloader/pkg/logger"
)

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

type ProgressCallback func(downloadedBytes, totalBytes, speed int64, etaSeconds int64, percentage float64)

type progressReader struct {
	ctx            context.Context
	reader         io.Reader
	totalBytes     int64
	downloaded     int64
	lastDownloaded int64
	lastReport     time.Time
	onProgress     ProgressCallback
}

func (pr *progressReader) Read(p []byte) (int, error) {
	if err := pr.ctx.Err(); err != nil {
		return 0, err
	}

	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.downloaded += int64(n)
		now := time.Now()
		elapsed := now.Sub(pr.lastReport).Seconds()

		if elapsed >= 0.3 {
			speed := int64(float64(pr.downloaded-pr.lastDownloaded) / elapsed)
			pr.lastDownloaded = pr.downloaded
			pr.lastReport = now

			var percentage float64
			var eta int64
			if pr.totalBytes > 0 {
				percentage = float64(pr.downloaded) / float64(pr.totalBytes) * 100
				if speed > 0 {
					eta = (pr.totalBytes - pr.downloaded) / speed
				}
			}

			if pr.onProgress != nil {
				pr.onProgress(pr.downloaded, pr.totalBytes, speed, eta, percentage)
			}
		}
	}

	return n, err
}

type CookieProvider func(excludeID ...string) (cookie string, cookieID string, hasMore bool)
type CookieExhaustedNotifier func(cookieID string)

type Downloader struct {
	client                  *http.Client
	cookieLock              sync.RWMutex
	googleCookie            string
	cookieProvider          CookieProvider
	cookieExhaustedNotifier CookieExhaustedNotifier
}

func NewDownloader() (*Downloader, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dialer.DialContext(ctx, "tcp4", addr)
			if err != nil {
				return dialer.DialContext(ctx, network, addr)
			}
			return conn, nil
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ReadBufferSize:        1024 * 1024, // 1MB buffer to handle high-bandwidth Gigabit/Fast Wi-Fi connections
		WriteBufferSize:       1024 * 1024,
	}

	return &Downloader{
		client: &http.Client{
			Jar:       jar,
			Transport: transport,
			Timeout:   0,
		},
	}, nil
}

func (d *Downloader) SetGoogleCookie(cookie string) {
	d.cookieLock.Lock()
	defer d.cookieLock.Unlock()
	d.googleCookie = strings.TrimSpace(cookie)
}

func (d *Downloader) SetCookieProvider(provider CookieProvider, onExhausted CookieExhaustedNotifier) {
	d.cookieLock.Lock()
	defer d.cookieLock.Unlock()
	d.cookieProvider = provider
	d.cookieExhaustedNotifier = onExhausted
}

func (d *Downloader) GetGoogleCookie() string {
	d.cookieLock.RLock()
	defer d.cookieLock.RUnlock()
	return d.googleCookie
}

func (d *Downloader) Download(ctx context.Context, fileID string, targetFolder string, onProgress ProgressCallback, desiredFilename ...string) (string, int64, error) {
	if err := os.MkdirAll(targetFolder, 0755); err != nil {
		return "", 0, fmt.Errorf("failed to create target folder: %w", err)
	}

	logger.Infof("Download", "Initiating download for file ID '%s' into '%s'", fileID, targetFolder)

	currentCookie := d.GetGoogleCookie()
	currentCookieID := "primary"
	d.cookieLock.RLock()
	provider := d.cookieProvider
	notifier := d.cookieExhaustedNotifier
	d.cookieLock.RUnlock()

	if provider != nil {
		if c, id, ok := provider(); ok {
			currentCookie = c
			currentCookieID = id
		}
	}

	for attempt := 0; attempt < 5; attempt++ {
		initialURL := fmt.Sprintf("https://drive.usercontent.google.com/download?id=%s&export=download&authuser=0&confirm=t", fileID)
		req, err := http.NewRequestWithContext(ctx, "GET", initialURL, nil)
		if err != nil {
			return "", 0, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

		if currentCookie != "" {
			req.Header.Set("Cookie", currentCookie)
		}

		resp, err := d.client.Do(req)
		if err != nil {
			if ctx.Err() == nil && !strings.Contains(err.Error(), "context canceled") {
				logger.Errorf("Download", "HTTP request error for %s: %v", fileID, err)
			}
			return "", 0, err
		}

		finalResp := resp
		var fallbackHTMLFilename string

		// Check if Google returned an HTML page (virus scan warning or confirmation form)
		contentType := resp.Header.Get("Content-Type")
		if strings.Contains(contentType, "text/html") {
			bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*512))
			resp.Body.Close()
			if err != nil {
				return "", 0, fmt.Errorf("failed reading response HTML: %w", err)
			}
			bodyStr := string(bodyBytes)

			// Check known Google errors
			if strings.Contains(bodyStr, "Quota exceeded") || strings.Contains(bodyStr, "Too many users have viewed or downloaded") {
				if notifier != nil && currentCookieID != "" {
					notifier(currentCookieID)
				}
				if provider != nil {
					if nextCookie, nextID, ok := provider(currentCookieID); ok {
						logger.Warnf("Download", "File %s: Google Drive quota exceeded on account '%s'. Auto-switching to account '%s' and restarting clean download...", fileID, currentCookieID, nextID)
						currentCookie = nextCookie
						currentCookieID = nextID
						continue
					}
				}
				errMsg := "Google Drive download quota exceeded for this file (all available accounts in Cookie Pool exhausted / 24h cooldown)"
				logger.Errorf("Download", "File %s: %s", fileID, errMsg)
				return "", 0, errors.New(errMsg)
			}
			if strings.Contains(bodyStr, "Access denied") || strings.Contains(bodyStr, "You need access") {
				errMsg := "Access denied: link requires Google login or private folder access"
				logger.Errorf("Download", "File %s: %s", fileID, errMsg)
				return "", 0, errors.New(errMsg)
			}
			if strings.Contains(bodyStr, "docs.google.com/spreadsheets") || strings.Contains(bodyStr, "docs.google.com/document") {
				errMsg := "Google Docs/Sheets online document (cannot be downloaded directly as binary file)"
				logger.Errorf("Download", "File %s: %s", fileID, errMsg)
				return "", 0, errors.New(errMsg)
			}

			// Extract filename from HTML if available
			if fnMatches := htmlFilenamePattern.FindStringSubmatch(bodyStr); len(fnMatches) > 1 {
				fallbackHTMLFilename = html.UnescapeString(fnMatches[1])
			}

			// Parse form fields from #download-form
			formAction := "https://drive.usercontent.google.com/download"
			if actionMatches := formActionPattern.FindStringSubmatch(bodyStr); len(actionMatches) > 1 {
				formAction = actionMatches[1]
				if strings.HasPrefix(formAction, "/") {
					formAction = "https://drive.google.com" + formAction
				}
			}

			// Gather query params from hidden input fields
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

			logger.Infof("Download", "File %s: Bypassing Google virus/size warning form", fileID)

			nextURL := fmt.Sprintf("%s?%s", formAction, queryParams.Encode())

			nextReq, err := http.NewRequestWithContext(ctx, "GET", nextURL, nil)
			if err != nil {
				return "", 0, err
			}
			nextReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

			if currentCookie != "" {
				nextReq.Header.Set("Cookie", currentCookie)
			}

			finalResp, err = d.client.Do(nextReq)
			if err != nil {
				if ctx.Err() == nil && !strings.Contains(err.Error(), "context canceled") {
					logger.Errorf("Download", "Second-stage download error for %s: %v", fileID, err)
				}
				return "", 0, err
			}

			if strings.Contains(finalResp.Header.Get("Content-Type"), "text/html") {
				finalBytes, _ := io.ReadAll(io.LimitReader(finalResp.Body, 1024*64))
				finalResp.Body.Close()
				content := string(finalBytes)
				if strings.Contains(content, "Quota exceeded") || strings.Contains(content, "Too many users have viewed or downloaded") {
					if notifier != nil && currentCookieID != "" {
						notifier(currentCookieID)
					}
					if provider != nil {
						if nextCookie, nextID, ok := provider(currentCookieID); ok {
							logger.Warnf("Download", "File %s: Quota exceeded on account '%s'. Auto-switching to account '%s' and restarting clean download...", fileID, currentCookieID, nextID)
							currentCookie = nextCookie
							currentCookieID = nextID
							continue
						}
					}
					errMsg := "Google Drive quota exceeded for this file (all available accounts in Cookie Pool exhausted / 24h cooldown)"
					logger.Errorf("Download", "File %s: %s", fileID, errMsg)
					return "", 0, errors.New(errMsg)
				}
				errMsg := "Google Drive returned HTML instead of file (file may require private access or login)"
				logger.Errorf("Download", "File %s: %s", fileID, errMsg)
				return "", 0, errors.New(errMsg)
			}
		}
		defer finalResp.Body.Close()

		var filename string
		var destPath string

		if len(desiredFilename) > 0 && desiredFilename[0] != "" {
			filename = sanitizeFilename(desiredFilename[0])
			destPath = filepath.Join(targetFolder, filename)
		} else {
			// Extract filename from header or HTML fallback
			filename = extractFilename(finalResp.Header.Get("Content-Disposition"))
			if filename == "" && fallbackHTMLFilename != "" {
				filename = fallbackHTMLFilename
			}
			if filename == "" {
				filename = fmt.Sprintf("gdrive_%s.bin", fileID)
			}
			filename = sanitizeFilename(filename)
			destPath = uniqueFilePath(filepath.Join(targetFolder, filename))
			filename = filepath.Base(destPath)
		}

		totalSize := finalResp.ContentLength
		if totalSize < 0 {
			totalSize = 0
		}

		var out *os.File
		var startOffset int64 = 0

		// Check if destination file exists on disk and is partially downloaded
		if fi, statErr := os.Stat(destPath); statErr == nil && fi.Size() > 0 && totalSize > 0 && fi.Size() < totalSize {
			// Attempt HTTP Range resume
			reqURL := finalResp.Request.URL.String()
			rangeReq, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
			if err == nil {
				rangeReq.Header.Set("Range", fmt.Sprintf("bytes=%d-", fi.Size()))
				rangeReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
				if currentCookie != "" {
					rangeReq.Header.Set("Cookie", currentCookie)
				}
				rangeResp, err := d.client.Do(rangeReq)
				if err == nil && rangeResp.StatusCode == http.StatusPartialContent {
					finalResp.Body.Close()
					finalResp = rangeResp
					out, err = os.OpenFile(destPath, os.O_WRONLY|os.O_APPEND, 0644)
					if err == nil {
						startOffset = fi.Size()
						logger.Infof("Download", "Resuming '%s' from byte %d / %d", filename, startOffset, totalSize)
					}
				} else if rangeResp != nil {
					rangeResp.Body.Close()
				}
			}
		}

		if out == nil {
			var err error
			out, err = os.Create(destPath)
			if err != nil {
				return "", 0, fmt.Errorf("failed to create destination file: %w", err)
			}
		}
		defer out.Close()

		logger.Infof("Download", "Receiving '%s' (Length: %d bytes, Starting: %d bytes)", filename, totalSize, startOffset)

		pr := &progressReader{
			ctx:            ctx,
			reader:         finalResp.Body,
			totalBytes:     totalSize,
			downloaded:     startOffset,
			lastDownloaded: startOffset,
			lastReport:     time.Now(),
			onProgress:     onProgress,
		}

		// Use 1MB buffer instead of default 32KB to eliminate syscall overhead for high-speed Wi-Fi/Gigabit connections
		buf := make([]byte, 1024*1024)
		copied, err := io.CopyBuffer(out, pr, buf)
		if err != nil {
			out.Close()
			_ = os.Remove(destPath)
			if ctx.Err() == nil && !strings.Contains(err.Error(), "context canceled") {
				logger.Errorf("Download", "Interrupted/failed downloading '%s': %v", filename, err)
			}
			return filename, copied, err
		}

		out.Close()

		// Verify File Integrity (Check for corruption / truncated file)
		if integrityErr := VerifyFileIntegrity(destPath, totalSize); integrityErr != nil {
			logger.Errorf("Integrity", "Integrity failure on '%s': %v", filename, integrityErr)
			return filename, copied, fmt.Errorf("CORRUPT: %w", integrityErr)
		}

		logger.Successf("Download", "Saved '%s' successfully (%d bytes)", filename, copied)

		if onProgress != nil {
			onProgress(copied, copied, 0, 0, 100.0)
		}

		return filename, copied, nil
	}

	return "", 0, errors.New("Google Drive download attempts exceeded limit")
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
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
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

