package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gdrive-downloader/pkg/gdrive"
)

var (
	curlCookieRegex  = regexp.MustCompile(`(?i)(?:-H|--header)\s+[\$]?[\^'"]+(?:cookie:\s*)([^\r\n'"\^]+)`)
	curlCookieBRegex = regexp.MustCompile(`(?i)(?:-b|--cookie)\s+[\$]?[\^'"]+([^\r\n'"\^]+)`)
	headerCookieRegex = regexp.MustCompile(`(?im)^\s*cookie:\s*([^\r\n]+)`)
)

func CleanCookieString(raw string) string {
	str := strings.TrimSpace(raw)
	if str == "" {
		return ""
	}

	// 1. JSON (Cookie-Editor / EditThisCookie export)
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
		// 2. cURL (-H 'cookie: ...')
		str = strings.TrimSpace(m[1])
	} else if m := curlCookieBRegex.FindStringSubmatch(str); len(m) > 1 {
		// 3. cURL (-b '...')
		str = strings.TrimSpace(m[1])
	} else if m := headerCookieRegex.FindStringSubmatch(str); len(m) > 1 {
		// 4. Raw headers (Cookie: ...)
		str = strings.TrimSpace(m[1])
	}

	// Sanitize any control characters, newlines, CR, or invalid bytes that break net/http header fields
	var b strings.Builder
	for _, r := range str {
		if r >= 32 && r < 127 {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

type CookieEntry struct {
	ID             string     `json:"id"`
	Label          string     `json:"label"`
	Cookie         string     `json:"cookie"`
	ExhaustedUntil *time.Time `json:"exhausted_until,omitempty"`
}

type Config struct {
	AuthEnabled             bool               `json:"auth_enabled"`
	Username                string             `json:"username"`
	PasswordHash            string             `json:"password_hash"`
	Salt                    string             `json:"salt"`
	DownloadFolder          string             `json:"download_folder"`
	MaxConcurrency          int                `json:"max_concurrency"`
	ChunksPerDownload       int                `json:"chunks_per_download"`
	GoogleCookie            string             `json:"google_cookie"`
	GoogleCookies           []CookieEntry      `json:"google_cookies,omitempty"`
	GoogleOAuthClientID     string             `json:"google_oauth_client_id,omitempty"`
	GoogleOAuthClientSecret string             `json:"google_oauth_client_secret,omitempty"`
	GoogleOAuthRedirectURI  string             `json:"google_oauth_redirect_uri,omitempty"`
	GoogleOAuthToken        *gdrive.OAuthToken `json:"google_oauth_token,omitempty"`
	GoogleOAuthEmail        string             `json:"google_oauth_email,omitempty"`
	AutoBypassQuota         bool               `json:"auto_bypass_quota"`
	AutoWarpEnabled         *bool              `json:"auto_warp_enabled,omitempty"`
	AutoWarpMinSpeedMB      float64            `json:"auto_warp_min_speed_mb,omitempty"`
	WarpProxyPort           int                `json:"warp_proxy_port,omitempty"`
	CustomProxyURL          string             `json:"custom_proxy_url,omitempty"`
}

type SessionInfo struct {
	Username  string
	ExpiresAt time.Time
}

type Manager struct {
	configPath string
	mu         sync.RWMutex
	config     Config

	sessionsMu sync.RWMutex
	sessions   map[string]SessionInfo
}

func NewManager(configPath string, defaultFolder string, defaultConcurrency int) (*Manager, error) {
	m := &Manager{
		configPath: configPath,
		sessions:   make(map[string]SessionInfo),
	}

	if err := m.loadOrInit(defaultFolder, defaultConcurrency); err != nil {
		return nil, err
	}

	// Clean up expired sessions periodically
	go m.cleanupLoop()

	return m, nil
}

func (m *Manager) loadOrInit(defaultFolder string, defaultConcurrency int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	defaultWarpTrue := true

	data, err := os.ReadFile(m.configPath)
	if err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err == nil {
			m.config = cfg
			if m.config.DownloadFolder == "" {
				m.config.DownloadFolder = defaultFolder
			}
			if m.config.MaxConcurrency <= 0 {
				m.config.MaxConcurrency = defaultConcurrency
			}
			if m.config.ChunksPerDownload <= 0 {
				m.config.ChunksPerDownload = 4
			}
			if m.config.AutoWarpEnabled == nil {
				m.config.AutoWarpEnabled = &defaultWarpTrue
			}
			if m.config.AutoWarpMinSpeedMB <= 0 {
				m.config.AutoWarpMinSpeedMB = 5.0
			}
			if m.config.WarpProxyPort <= 0 {
				m.config.WarpProxyPort = 40000
			}
			if len(m.config.GoogleCookies) == 0 && m.config.GoogleCookie != "" {
				m.config.GoogleCookies = []CookieEntry{
					{
						ID:     "primary",
						Label:  "Primary Account",
						Cookie: m.config.GoogleCookie,
					},
				}
			}
			return nil
		}
	}

	// First time initialization with defaults
	salt := generateRandomHex(16)
	m.config = Config{
		AuthEnabled:        true,
		Username:           "admin",
		Salt:               salt,
		PasswordHash:       hashPassword("adminadmin", salt),
		DownloadFolder:     defaultFolder,
		MaxConcurrency:     defaultConcurrency,
		ChunksPerDownload:  4,
		AutoBypassQuota:    true,
		AutoWarpEnabled:    &defaultWarpTrue,
		AutoWarpMinSpeedMB: 5.0,
		WarpProxyPort:      40000,
	}

	return m.saveLocked()
}

func (m *Manager) saveLocked() error {
	dir := filepath.Dir(m.configPath)
	if dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0600)
}

func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveLocked()
}

func (m *Manager) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
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
				{
					ID:     "primary",
					Label:  "Primary Account",
					Cookie: googleCookie,
				},
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
				{
					ID:     "primary",
					Label:  "Primary Account",
					Cookie: cookie,
				},
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

	id := fmt.Sprintf("cookie_%d", time.Now().UnixNano())
	entry := CookieEntry{
		ID:     id,
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

func (m *Manager) IsAuthEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.AuthEnabled
}

func (m *Manager) SetAuthEnabled(enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.AuthEnabled = enabled
	return m.saveLocked()
}

func (m *Manager) Authenticate(username, password string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if username != m.config.Username {
		return false
	}

	expectedHash := hashPassword(password, m.config.Salt)
	return expectedHash == m.config.PasswordHash
}

func (m *Manager) UpdateCredentials(oldPassword, newUsername, newPassword string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if newPassword != "" {
		if oldPassword == "" {
			return errors.New("current password is required")
		}
		expectedHash := hashPassword(oldPassword, m.config.Salt)
		if expectedHash != m.config.PasswordHash {
			return errors.New("current password incorrect")
		}
		if len(newPassword) < 4 {
			return errors.New("new password must be at least 4 characters")
		}
		salt := generateRandomHex(16)
		m.config.Salt = salt
		m.config.PasswordHash = hashPassword(newPassword, salt)
	}

	if newUsername != "" {
		m.config.Username = newUsername
	}

	return m.saveLocked()
}

// SetInitialCredentials forces credentials to be set without requiring oldPassword (useful for env vars / flags)
func (m *Manager) SetInitialCredentials(username, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if username != "" {
		m.config.Username = username
	}
	if password != "" {
		salt := generateRandomHex(16)
		m.config.Salt = salt
		m.config.PasswordHash = hashPassword(password, salt)
	}
	return m.saveLocked()
}

func (m *Manager) CreateSession(username string) string {
	token := generateRandomHex(32)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	m.sessionsMu.Lock()
	defer m.sessionsMu.Unlock()

	m.sessions[token] = SessionInfo{
		Username:  username,
		ExpiresAt: expiresAt,
	}

	return token
}

func (m *Manager) ValidateSession(token string) (string, bool) {
	if token == "" {
		return "", false
	}

	m.sessionsMu.RLock()
	defer m.sessionsMu.RUnlock()

	info, exists := m.sessions[token]
	if !exists {
		return "", false
	}

	if time.Now().After(info.ExpiresAt) {
		return "", false
	}

	return info.Username, true
}

func (m *Manager) RevokeSession(token string) {
	if token == "" {
		return
	}

	m.sessionsMu.Lock()
	defer m.sessionsMu.Unlock()

	delete(m.sessions, token)
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		m.sessionsMu.Lock()
		now := time.Now()
		for token, info := range m.sessions {
			if now.After(info.ExpiresAt) {
				delete(m.sessions, token)
			}
		}
		m.sessionsMu.Unlock()
	}
}

func hashPassword(password, salt string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", password, salt)))
	return hex.EncodeToString(sum[:])
}

func generateRandomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
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

