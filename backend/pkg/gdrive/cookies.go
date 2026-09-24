package gdrive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gdrive-downloader/pkg/logger"
)

type CookieProvider func(excludeID ...string) (cookie string, cookieID string, hasMore bool)
type CookieExhaustedNotifier func(cookieID string)

func (d *Downloader) populateCookieJar(cookieStr string) {
	if cookieStr == "" || d.client == nil || d.client.Jar == nil {
		return
	}
	rawCookies := strings.Split(cookieStr, ";")
	var cookies []*http.Cookie
	for _, raw := range rawCookies {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parts := strings.SplitN(raw, "=", 2)
		if len(parts) == 2 {
			name := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if name != "" {
				cookies = append(cookies, &http.Cookie{
					Name:   name,
					Value:  val,
					Path:   "/",
					Domain: ".google.com",
				})
			}
		}
	}
	if len(cookies) == 0 {
		return
	}
	targets := []string{
		"https://google.com",
		"https://drive.google.com",
		"https://drive.usercontent.google.com",
		"https://docs.googleusercontent.com",
		"https://googleusercontent.com",
	}
	for _, target := range targets {
		if u, err := url.Parse(target); err == nil {
			d.client.Jar.SetCookies(u, cookies)
		}
	}
}

func (d *Downloader) SetGoogleCookie(cookie string) {
	d.cookieLock.Lock()
	defer d.cookieLock.Unlock()
	d.googleCookie = strings.TrimSpace(cookie)
	d.populateCookieJar(d.googleCookie)
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

func (d *Downloader) resolveInitialCookie() (string, string, CookieProvider, CookieExhaustedNotifier) {
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
	if currentCookie != "" {
		d.populateCookieJar(currentCookie)
	}
	return currentCookie, currentCookieID, provider, notifier
}

func (d *Downloader) tryAltDownloadEndpoint(ctx context.Context, fileID, currentCookie string) (*http.Response, string, bool) {
	altURL := fmt.Sprintf("https://drive.google.com/uc?id=%s&export=download&confirm=t", fileID)
	altReq, _ := http.NewRequestWithContext(ctx, "GET", altURL, nil)
	if altReq == nil {
		return nil, "", false
	}
	altReq.Header.Set("User-Agent", defaultUserAgent)
	if currentCookie != "" {
		altReq.Header.Set("Cookie", currentCookie)
	}
	altResp, altErr := d.client.Do(altReq)
	if altErr != nil {
		return nil, "", false
	}
	altCT := altResp.Header.Get("Content-Type")
	if !strings.Contains(altCT, "text/html") {
		return altResp, "", true
	}
	altBytes, _ := io.ReadAll(io.LimitReader(altResp.Body, 1024*512))
	altResp.Body.Close()
	altStr := string(altBytes)
	if !strings.Contains(altStr, "Quota exceeded") && !strings.Contains(altStr, "Too many users have viewed or downloaded") {
		return nil, altStr, true
	}
	return nil, "", false
}

func (d *Downloader) rotateOrBypassQuota(
	ctx context.Context,
	fileID string,
	targetFolder string,
	desiredFilename []string,
	onProgress ProgressCallback,
	currentCookie *string,
	currentCookieID *string,
	triedAltEndpoint *bool,
	provider CookieProvider,
	notifier CookieExhaustedNotifier,
	stage2 bool,
) (rotated bool, filename string, written int64, err error) {
	if notifier != nil && *currentCookieID != "" {
		notifier(*currentCookieID)
	}
	if provider != nil {
		if nextCookie, nextID, ok := provider(*currentCookieID); ok {
			if stage2 {
				logger.Warnf("Download", "File %s: Quota exceeded on account '%s'. Auto-switching to account '%s' and restarting clean download...", fileID, *currentCookieID, nextID)
			} else {
				logger.Warnf("Download", "File %s: Google Drive quota exceeded on account '%s'. Auto-switching to account '%s' and restarting clean download...", fileID, *currentCookieID, nextID)
			}
			*currentCookie = nextCookie
			*currentCookieID = nextID
			d.populateCookieJar(*currentCookie)
			*triedAltEndpoint = false
			return true, "", 0, nil
		}
	}
	if bm := d.GetBypassManager(); bm != nil && bm.IsAvailable() && bm.IsAutoBypass() {
		desired := ""
		if len(desiredFilename) > 0 {
			desired = desiredFilename[0]
		}
		if stage2 {
			logger.Warnf("Download", "File %s: Quota exceeded in stage 2. Executing automated OAuth bypass via ggdl_temp...", fileID)
		} else {
			logger.Warnf("Download", "File %s: Google Drive quota exceeded. Executing automated OAuth bypass via ggdl_temp...", fileID)
		}
		fn, sz, bErr := bm.ExecuteBypass(ctx, fileID, targetFolder, desired, onProgress)
		return false, fn, sz, bErr
	}
	errMsg := "Google Drive download quota exceeded for this file (all available accounts in Cookie Pool exhausted / 24h cooldown)"
	if stage2 {
		errMsg = "Google Drive quota exceeded for this file (all available accounts in Cookie Pool exhausted / 24h cooldown)"
	}
	logger.Errorf("Download", "File %s: %s", fileID, errMsg)
	return false, "", 0, errors.New(errMsg)
}
