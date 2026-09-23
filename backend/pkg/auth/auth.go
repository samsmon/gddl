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
	"strings"
	"sync"
	"time"
)

type CookieEntry struct {
	ID             string     `json:"id"`
	Label          string     `json:"label"`
	Cookie         string     `json:"cookie"`
	ExhaustedUntil *time.Time `json:"exhausted_until,omitempty"`
}

type Config struct {
	AuthEnabled    bool          `json:"auth_enabled"`
	Username       string        `json:"username"`
	PasswordHash   string        `json:"password_hash"`
	Salt           string        `json:"salt"`
	DownloadFolder string        `json:"download_folder"`
	MaxConcurrency int           `json:"max_concurrency"`
	GoogleCookie   string        `json:"google_cookie"`
	GoogleCookies  []CookieEntry `json:"google_cookies,omitempty"`
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
		AuthEnabled:    true,
		Username:       "admin",
		Salt:           salt,
		PasswordHash:   hashPassword("adminadmin", salt),
		DownloadFolder: defaultFolder,
		MaxConcurrency: defaultConcurrency,
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

func (m *Manager) UpdateConfig(folder string, concurrency int, googleCookie string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if folder != "" {
		m.config.DownloadFolder = folder
	}
	if concurrency > 0 {
		m.config.MaxConcurrency = concurrency
	}
	if googleCookie != "" {
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
	cookie = strings.TrimSpace(cookie)
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
