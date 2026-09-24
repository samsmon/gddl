package auth

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gdrive-downloader/pkg/gdrive"
)

var (
	curlCookieRegex   = regexp.MustCompile(`(?i)(?:-H|--header)\s+[\$]?[\^'"]+(?:cookie:\s*)([^\r\n'"\^]+)`)
	curlCookieBRegex  = regexp.MustCompile(`(?i)(?:-b|--cookie)\s+[\$]?[\^'"]+([^\r\n'"\^]+)`)
	headerCookieRegex = regexp.MustCompile(`(?im)^\s*cookie:\s*([^\r\n]+)`)
)

func CleanCookieString(raw string) string {
	str := strings.TrimSpace(raw)
	if str == "" {
		return ""
	}

	if (strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]")) || (strings.HasPrefix(str, "{") && strings.HasSuffix(str, "}")) {
		var arr []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal([]byte(str), &arr); err == nil && len(arr) > 0 {
			var parts []string
			for _, item := range arr {
				if item.Name != "" {
					parts = append(parts, fmt.Sprintf("%s=%s", item.Name, item.Value))
				}
			}
			if len(parts) > 0 {
				str = strings.Join(parts, "; ")
			}
		}
	} else if m := curlCookieRegex.FindStringSubmatch(str); len(m) > 1 {
		str = strings.TrimSpace(m[1])
	} else if m := curlCookieBRegex.FindStringSubmatch(str); len(m) > 1 {
		str = strings.TrimSpace(m[1])
	} else if m := headerCookieRegex.FindStringSubmatch(str); len(m) > 1 {
		str = strings.TrimSpace(m[1])
	}

	var b strings.Builder
	for _, r := range str {
		if r >= 32 && r < 127 {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func (m *Manager) GetChunksPerDownload() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.config.ChunksPerDownload <= 0 {
		return 4
	}
	return m.config.ChunksPerDownload
}

func (m *Manager) SetChunksPerDownload(n int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n <= 0 {
		n = 4
	}
	m.config.ChunksPerDownload = n
	return m.saveLocked()
}

func (m *Manager) UpdateConfig(folder string, concurrency int, chunks int, googleCookie string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if folder != "" {
		m.config.DownloadFolder = folder
	}
	if concurrency > 0 {
		m.config.MaxConcurrency = concurrency
	}
	if chunks > 0 {
		m.config.ChunksPerDownload = chunks
	}
	if googleCookie != "" {
		googleCookie = CleanCookieString(googleCookie)
		m.config.GoogleCookie = googleCookie
		if len(m.config.GoogleCookies) == 0 {
			m.config.GoogleCookies = []CookieEntry{
				{ID: "primary", Label: "Primary Account", Cookie: googleCookie},
			}
		} else {
			m.config.GoogleCookies[0].Cookie = googleCookie
		}
	}

	return m.saveLocked()
}

func (m *Manager) SetGoogleCookie(cookie string) error {
	cookie = CleanCookieString(cookie)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.GoogleCookie = cookie
	if cookie != "" {
		if len(m.config.GoogleCookies) == 0 {
			m.config.GoogleCookies = []CookieEntry{
				{ID: "primary", Label: "Primary Account", Cookie: cookie},
			}
		} else {
			m.config.GoogleCookies[0].Cookie = cookie
		}
	} else {
		m.config.GoogleCookies = nil
	}
	return m.saveLocked()
}

func (m *Manager) GetCookies() []CookieEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]CookieEntry, len(m.config.GoogleCookies))
	copy(res, m.config.GoogleCookies)
	return res
}

func (m *Manager) AddCookie(label, cookie string) (CookieEntry, error) {
	cookie = CleanCookieString(cookie)
	if cookie == "" {
		return CookieEntry{}, fmt.Errorf("cookie cannot be empty")
	}
	label = strings.TrimSpace(label)

	m.mu.Lock()
	defer m.mu.Unlock()

	if label == "" {
		label = fmt.Sprintf("Account %d", len(m.config.GoogleCookies)+1)
	}

	entry := CookieEntry{
		ID:     fmt.Sprintf("cookie_%d", time.Now().UnixNano()),
		Label:  label,
		Cookie: cookie,
	}
	m.config.GoogleCookies = append(m.config.GoogleCookies, entry)
	if m.config.GoogleCookie == "" {
		m.config.GoogleCookie = cookie
	}
	_ = m.saveLocked()
	return entry, nil
}

func (m *Manager) RemoveCookie(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := -1
	for i, c := range m.config.GoogleCookies {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("cookie not found")
	}

	m.config.GoogleCookies = append(m.config.GoogleCookies[:idx], m.config.GoogleCookies[idx+1:]...)
	if len(m.config.GoogleCookies) > 0 {
		m.config.GoogleCookie = m.config.GoogleCookies[0].Cookie
	} else {
		m.config.GoogleCookie = ""
	}
	return m.saveLocked()
}

func (m *Manager) ResetCookieCooldown(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.config.GoogleCookies {
		if m.config.GoogleCookies[i].ID == id {
			m.config.GoogleCookies[i].ExhaustedUntil = nil
			return m.saveLocked()
		}
	}
	return fmt.Errorf("cookie not found")
}

func (m *Manager) ResetAllCookieCooldowns() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.config.GoogleCookies {
		m.config.GoogleCookies[i].ExhaustedUntil = nil
	}
	return m.saveLocked()
}

func (m *Manager) MarkCookieExhausted(idOrCookie string, duration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	until := time.Now().Add(duration)
	for i := range m.config.GoogleCookies {
		if m.config.GoogleCookies[i].ID == idOrCookie || m.config.GoogleCookies[i].Cookie == idOrCookie {
			m.config.GoogleCookies[i].ExhaustedUntil = &until
			return m.saveLocked()
		}
	}
	return nil
}

func (m *Manager) GetNextActiveCookie(excludeID ...string) (CookieEntry, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for i := range m.config.GoogleCookies {
		c := &m.config.GoogleCookies[i]
		if len(excludeID) > 0 && (c.ID == excludeID[0] || c.Cookie == excludeID[0]) {
			continue
		}
		if c.ExhaustedUntil != nil {
			if now.After(*c.ExhaustedUntil) {
				c.ExhaustedUntil = nil
			} else {
				continue
			}
		}
		return *c, true
	}
	return CookieEntry{}, false
}

func (m *Manager) GetOAuthSettings() (clientID string, clientSecret string, redirectURI string, token *gdrive.OAuthToken, email string, autoBypass bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.GoogleOAuthClientID, m.config.GoogleOAuthClientSecret, m.config.GoogleOAuthRedirectURI, m.config.GoogleOAuthToken, m.config.GoogleOAuthEmail, m.config.AutoBypassQuota
}

func (m *Manager) SetOAuthCredentials(clientID, clientSecret, redirectURI string, autoBypass bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.GoogleOAuthClientID = strings.TrimSpace(clientID)
	m.config.GoogleOAuthClientSecret = strings.TrimSpace(clientSecret)
	m.config.GoogleOAuthRedirectURI = strings.TrimSpace(redirectURI)
	m.config.AutoBypassQuota = autoBypass
	return m.saveLocked()
}

func (m *Manager) GetOAuthRedirectURI() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.GoogleOAuthRedirectURI
}

func (m *Manager) SaveOAuthToken(token *gdrive.OAuthToken, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.GoogleOAuthToken = token
	m.config.GoogleOAuthEmail = email
	return m.saveLocked()
}

func (m *Manager) ClearOAuthToken() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.GoogleOAuthToken = nil
	m.config.GoogleOAuthEmail = ""
	return m.saveLocked()
}

func (m *Manager) SetAutoBypassQuota(enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.AutoBypassQuota = enabled
	return m.saveLocked()
}

func (m *Manager) GetWarpSettings() (autoEnabled bool, minSpeedMB float64, proxyPort int, customProxyURL string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	autoEnabled = true
	if m.config.AutoWarpEnabled != nil {
		autoEnabled = *m.config.AutoWarpEnabled
	}
	minSpeedMB = m.config.AutoWarpMinSpeedMB
	if minSpeedMB <= 0 {
		minSpeedMB = 5.0
	}
	proxyPort = m.config.WarpProxyPort
	if proxyPort <= 0 {
		proxyPort = 40000
	}
	customProxyURL = m.config.CustomProxyURL
	return
}

func (m *Manager) SaveWarpSettings(autoEnabled bool, minSpeedMB float64, proxyPort int, customProxyURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.AutoWarpEnabled = &autoEnabled
	if minSpeedMB > 0 {
		m.config.AutoWarpMinSpeedMB = minSpeedMB
	}
	if proxyPort > 0 {
		m.config.WarpProxyPort = proxyPort
	}
	m.config.CustomProxyURL = strings.TrimSpace(customProxyURL)
	return m.saveLocked()
}
