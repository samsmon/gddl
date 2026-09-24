package auth

import (
	"strings"

	"gdrive-downloader/pkg/gdrive"
)

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
