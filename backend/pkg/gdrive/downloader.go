package gdrive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"sync"
	"time"

	"gdrive-downloader/pkg/logger"
)

type Downloader struct {
	client                  *http.Client
	transport               *http.Transport
	chunkedDownloader       *ChunkedDownloader
	cookieLock              sync.RWMutex
	googleCookie            string
	cookieProvider          CookieProvider
	cookieExhaustedNotifier CookieExhaustedNotifier
	bypassManager           *BypassManager
	chunksPerDownload       int
}

func NewDownloader() (*Downloader, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if conn, err := dialer.DialContext(ctx, "tcp4", addr); err == nil {
				return conn, nil
			}
			return dialer.DialContext(ctx, network, addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ReadBufferSize:        1024 * 1024,
		WriteBufferSize:       1024 * 1024,
	}
	client := &http.Client{
		Jar:       jar,
		Transport: transport,
		Timeout:   0,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 15 {
				return errors.New("stopped after 15 redirects")
			}
			if len(via) > 0 {
				prev := via[len(via)-1]
				if cookie := prev.Header.Get("Cookie"); cookie != "" {
					req.Header.Set("Cookie", cookie)
				}
				if ua := prev.Header.Get("User-Agent"); ua != "" {
					req.Header.Set("User-Agent", ua)
				}
			}
			return nil
		},
	}
	return &Downloader{
		client:            client,
		transport:         transport,
		chunkedDownloader: NewChunkedDownloader(client),
		chunksPerDownload: 4,
	}, nil
}

// BindProxyController connects this downloader and its chunked downloader to the WARP/Proxy controller.
func (d *Downloader) BindProxyController(registerTransport func(*http.Transport), epochProvider func() uint64, onRateLimit func(string)) {
	if registerTransport != nil {
		registerTransport(d.transport)
		registerTransport(d.chunkedDownloader.Transport())
	}
	d.chunkedDownloader.SetEpochProvider(epochProvider)
	d.chunkedDownloader.SetRateLimitCallback(onRateLimit)
}

func (d *Downloader) SetChunksPerDownload(n int) {
	if d == nil {
		return
	}
	d.cookieLock.Lock()
	defer d.cookieLock.Unlock()
	if n <= 0 {
		n = 4
	}
	d.chunksPerDownload = n
	if d.bypassManager != nil {
		d.bypassManager.SetChunksPerDownload(n)
	}
}

func (d *Downloader) GetChunksPerDownload() int {
	if d == nil {
		return 4
	}
	d.cookieLock.RLock()
	defer d.cookieLock.RUnlock()
	if d.chunksPerDownload <= 0 {
		return 4
	}
	return d.chunksPerDownload
}

func (d *Downloader) SetBypassManager(bm *BypassManager) {
	d.cookieLock.Lock()
	defer d.cookieLock.Unlock()
	d.bypassManager = bm
	if bm != nil && d.chunksPerDownload > 0 {
		bm.SetChunksPerDownload(d.chunksPerDownload)
	}
}

func (d *Downloader) GetBypassManager() *BypassManager {
	d.cookieLock.RLock()
	defer d.cookieLock.RUnlock()
	return d.bypassManager
}

func (d *Downloader) Download(ctx context.Context, fileID string, targetFolder string, onProgress ProgressCallback, desiredFilename ...string) (string, int64, error) {
	if err := os.MkdirAll(targetFolder, 0755); err != nil {
		return "", 0, fmt.Errorf("failed to create target folder: %w", err)
	}
	logger.Infof("Download", "Initiating download for file ID '%s' into '%s'", fileID, targetFolder)
	currentCookie, currentCookieID, provider, notifier := d.resolveInitialCookie()

	triedAltEndpoint := false
	for attempt := 0; attempt < 5; attempt++ {
		initialURL := fmt.Sprintf("https://drive.usercontent.google.com/download?id=%s&export=download&authuser=0&confirm=t", fileID)
		req, err := http.NewRequestWithContext(ctx, "GET", initialURL, nil)
		if err != nil {
			return "", 0, err
		}
		req.Header.Set("User-Agent", defaultUserAgent)
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
		if strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
			bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*512))
			resp.Body.Close()
			if err != nil {
				return "", 0, fmt.Errorf("failed reading response HTML: %w", err)
			}
			bodyStr := string(bodyBytes)
			if strings.Contains(bodyStr, "ServiceLogin") || strings.Contains(bodyStr, "accounts.google.com/signin") {
				logger.Warnf("Download", "File %s: Account '%s' cookie was not recognized as signed-in by Google (may be expired)", fileID, currentCookieID)
			}
			if strings.Contains(bodyStr, "Quota exceeded") || strings.Contains(bodyStr, "Too many users have viewed or downloaded") {
				if !triedAltEndpoint {
					triedAltEndpoint = true
					if altResp, altHTML, ok := d.tryAltDownloadEndpoint(ctx, fileID, currentCookie); ok {
						if altResp != nil {
							return d.downloadStream(ctx, altResp, fileID, targetFolder, fallbackHTMLFilename, currentCookie, onProgress, desiredFilename...)
						}
						bodyStr = altHTML
						goto parseForm
					}
				}
				rotated, fn, sz, qErr := d.rotateOrBypassQuota(ctx, fileID, targetFolder, desiredFilename, onProgress, &currentCookie, &currentCookieID, &triedAltEndpoint, provider, notifier, false)
				if rotated {
					continue
				}
				return fn, sz, qErr
			}
			if strings.Contains(bodyStr, "Access denied") || strings.Contains(bodyStr, "You need access") {
				if bm := d.GetBypassManager(); bm != nil && bm.IsAvailable() {
					desired := ""
					if len(desiredFilename) > 0 {
						desired = desiredFilename[0]
					}
					logger.Infof("Download", "File %s: Access denied on public link. Attempting direct OAuth download with authenticated Google account...", fileID)
					return bm.DownloadClonedFile(ctx, fileID, targetFolder, desired, onProgress)
				}
				errMsg := "Access denied: link requires Google login or private folder access"
				logger.Errorf("Download", "File %s: %s", fileID, errMsg)
				return "", 0, errors.New(errMsg)
			}
			if strings.Contains(bodyStr, "docs.google.com/spreadsheets") || strings.Contains(bodyStr, "docs.google.com/document") {
				errMsg := "Google Docs/Sheets online document (cannot be downloaded directly as binary file)"
				logger.Errorf("Download", "File %s: %s", fileID, errMsg)
				return "", 0, errors.New(errMsg)
			}
		parseForm:
			var nextURL string
			nextURL, fallbackHTMLFilename = parseConfirmationHTML(bodyStr, fileID)
			logger.Infof("Download", "File %s: Bypassing Google virus/size warning form", fileID)
			nextReq, err := http.NewRequestWithContext(ctx, "GET", nextURL, nil)
			if err != nil {
				return "", 0, err
			}
			nextReq.Header.Set("User-Agent", defaultUserAgent)
			nextReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
			nextReq.Header.Set("Referer", fmt.Sprintf("https://drive.google.com/file/d/%s/view", fileID))
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
				htmlSnippet := string(finalBytes)
				if len(htmlSnippet) > 300 {
					htmlSnippet = htmlSnippet[:300]
				}
				logger.Warnf("Download", "File %s: Stage 2 received HTML response (snippet: %q)", fileID, htmlSnippet)
				contentLower := strings.ToLower(string(finalBytes))
				if strings.Contains(contentLower, "quota exceeded") || strings.Contains(contentLower, "too many users have viewed or downloaded") {
					rotated, fn, sz, qErr := d.rotateOrBypassQuota(ctx, fileID, targetFolder, desiredFilename, onProgress, &currentCookie, &currentCookieID, &triedAltEndpoint, provider, notifier, true)
					if rotated {
						continue
					}
					return fn, sz, qErr
				}
				errMsg := fmt.Sprintf("Google Drive returned HTML instead of file (file may require private access or login): %s", strings.TrimSpace(htmlSnippet))
				logger.Errorf("Download", "File %s: %s", fileID, errMsg)
				return "", 0, errors.New(errMsg)
			}
		}
		return d.downloadStream(ctx, finalResp, fileID, targetFolder, fallbackHTMLFilename, currentCookie, onProgress, desiredFilename...)
	}
	return "", 0, errors.New("Google Drive download attempts exceeded limit")
}
