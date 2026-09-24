package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

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
