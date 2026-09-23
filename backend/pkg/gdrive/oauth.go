package gdrive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"gdrive-downloader/pkg/logger"
)

const (
	GoogleDriveScope       = "https://www.googleapis.com/auth/drive"
	GoogleOAuthAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleOAuthTokenURL    = "https://oauth2.googleapis.com/token"
	GoogleDriveAPIEndpoint = "https://www.googleapis.com/drive/v3"
	GoogleUserInfoEndpoint = "https://www.googleapis.com/oauth2/v2/userinfo"
)

// OAuthToken represents the stored Google OAuth2 token.
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
}

// Expired returns true if the access token is expired or close to expiring (within 2 minutes).
func (t *OAuthToken) Expired() bool {
	if t == nil || t.AccessToken == "" {
		return true
	}
	return time.Now().Add(2 * time.Minute).After(t.Expiry)
}

// OAuthManager handles Google Drive OAuth2 token exchange, auto-refresh, and API authentication.
type OAuthManager struct {
	mu           sync.RWMutex
	clientID     string
	clientSecret string
	redirectURI  string
	token        *OAuthToken
	email        string
	onTokenSave  func(token *OAuthToken, email string)
	httpClient   *http.Client
}

// NewOAuthManager creates a new OAuthManager instance.
func NewOAuthManager(clientID, clientSecret, redirectURI string, token *OAuthToken, email string, onTokenSave func(token *OAuthToken, email string)) *OAuthManager {
	return &OAuthManager{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		token:        token,
		email:        email,
		onTokenSave:  onTokenSave,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// UpdateConfig updates the client ID, secret, and redirect URI.
func (om *OAuthManager) UpdateConfig(clientID, clientSecret, redirectURI string) {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.clientID = strings.TrimSpace(clientID)
	om.clientSecret = strings.TrimSpace(clientSecret)
	if redirectURI != "" {
		om.redirectURI = strings.TrimSpace(redirectURI)
	}
}

// SetToken manually updates the token and email.
func (om *OAuthManager) SetToken(token *OAuthToken, email string) {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.token = token
	om.email = email
}

// ClearToken disconnects the Google account.
func (om *OAuthManager) ClearToken() {
	om.mu.Lock()
	om.token = nil
	om.email = ""
	saveCb := om.onTokenSave
	om.mu.Unlock()

	if saveCb != nil {
		saveCb(nil, "")
	}
}

// IsConfigured returns true if client ID and secret are set.
func (om *OAuthManager) IsConfigured() bool {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.clientID != "" && om.clientSecret != ""
}

// IsConnected returns true if there is a valid or refreshable token.
func (om *OAuthManager) IsConnected() bool {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.token != nil && (om.token.AccessToken != "" || om.token.RefreshToken != "")
}

// GetStatus returns the current connection state.
func (om *OAuthManager) GetStatus() (isConfigured bool, isConnected bool, email string, expiry *time.Time, clientID string) {
	om.mu.RLock()
	defer om.mu.RUnlock()
	isConfigured = om.clientID != "" && om.clientSecret != ""
	isConnected = om.token != nil && (om.token.AccessToken != "" || om.token.RefreshToken != "")
	email = om.email
	if om.token != nil && !om.token.Expiry.IsZero() {
		exp := om.token.Expiry
		expiry = &exp
	}
	clientID = om.clientID
	return
}

// BuildAuthURL generates the Google OAuth consent URL.
func (om *OAuthManager) BuildAuthURL(state string) (string, error) {
	om.mu.RLock()
	clientID := om.clientID
	redirectURI := om.redirectURI
	om.mu.RUnlock()

	if clientID == "" {
		return "", errors.New("google OAuth Client ID is not configured")
	}
	if redirectURI == "" {
		return "", errors.New("redirect URI is not configured")
	}

	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", GoogleDriveScope+" https://www.googleapis.com/auth/userinfo.email")
	params.Set("access_type", "offline")
	params.Set("prompt", "consent") // Ensures refresh_token is returned
	if state != "" {
		params.Set("state", state)
	}

	return fmt.Sprintf("%s?%s", GoogleOAuthAuthURL, params.Encode()), nil
}

// ExtractCode extracts authorization code from a raw code or full callback URL (rclone style).
func ExtractCode(input string) string {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return ""
	}

	// 1. If it looks like a full URL: http://127.0.0.1:5244/api/gdrive/oauth/callback?code=...
	if strings.Contains(raw, "://") || strings.Contains(raw, "?") || strings.Contains(raw, "&") {
		// Try parsing as URL
		if u, err := url.Parse(raw); err == nil {
			if code := u.Query().Get("code"); code != "" {
				return strings.TrimSpace(code)
			}
		}
		// Fallback query string parse
		parts := strings.Split(raw, "?")
		qStr := raw
		if len(parts) > 1 {
			qStr = parts[1]
		}
		if vals, err := url.ParseQuery(qStr); err == nil {
			if code := vals.Get("code"); code != "" {
				return strings.TrimSpace(code)
			}
		}
	}

	// 2. Otherwise assume raw code (e.g. 4/0A...)
	return raw
}

// ExchangeCode trades an authorization code for access and refresh tokens.
func (om *OAuthManager) ExchangeCode(ctx context.Context, codeOrURL string) (*OAuthToken, string, error) {
	code := ExtractCode(codeOrURL)
	if code == "" {
		return nil, "", errors.New("empty authorization code")
	}

	om.mu.RLock()
	clientID := om.clientID
	clientSecret := om.clientSecret
	redirectURI := om.redirectURI
	om.mu.RUnlock()

	if clientID == "" || clientSecret == "" {
		return nil, "", errors.New("google OAuth Client ID or Client Secret is missing")
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", GoogleOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := om.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed sending token exchange request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed reading token response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("google token exchange failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, "", fmt.Errorf("failed parsing token JSON: %w", err)
	}

	if res.AccessToken == "" {
		return nil, "", errors.New("google did not return an access_token")
	}

	tok := &OAuthToken{
		AccessToken:  res.AccessToken,
		TokenType:    res.TokenType,
		RefreshToken: res.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(res.ExpiresIn) * time.Second),
	}

	// Fetch user's email for display
	email, _ := om.fetchUserEmail(ctx, tok.AccessToken)

	om.mu.Lock()
	if tok.RefreshToken == "" && om.token != nil && om.token.RefreshToken != "" {
		// Keep existing refresh token if Google didn't reissue one
		tok.RefreshToken = om.token.RefreshToken
	}
	om.token = tok
	om.email = email
	saveCb := om.onTokenSave
	om.mu.Unlock()

	if saveCb != nil {
		saveCb(tok, email)
	}

	logger.Infof("OAuth", "Successfully authenticated Google Drive account: %s", email)
	return tok, email, nil
}

// GetValidAccessToken returns a valid access token, auto-refreshing if expired.
func (om *OAuthManager) GetValidAccessToken(ctx context.Context) (string, error) {
	om.mu.Lock()
	defer om.mu.Unlock()

	if om.token == nil {
		return "", errors.New("not authenticated with Google Drive")
	}

	// If not expired, return existing access token
	if !om.token.Expired() {
		return om.token.AccessToken, nil
	}

	// Must refresh
	if om.token.RefreshToken == "" {
		return "", errors.New("access token expired and no refresh token available; please log in again")
	}

	logger.Infof("OAuth", "Access token expired. Refreshing token with Google...")

	form := url.Values{}
	form.Set("client_id", om.clientID)
	form.Set("client_secret", om.clientSecret)
	form.Set("refresh_token", om.token.RefreshToken)
	form.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, "POST", GoogleOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := om.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed refreshing token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed reading refresh response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("google refresh token failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("failed parsing refresh response JSON: %w", err)
	}

	om.token.AccessToken = res.AccessToken
	if res.TokenType != "" {
		om.token.TokenType = res.TokenType
	}
	om.token.Expiry = time.Now().Add(time.Duration(res.ExpiresIn) * time.Second)

	saveCb := om.onTokenSave
	email := om.email
	tokCopy := *om.token

	go func() {
		if saveCb != nil {
			saveCb(&tokCopy, email)
		}
	}()

	logger.Infof("OAuth", "Successfully refreshed Google Drive access token (expires in %ds)", res.ExpiresIn)
	return om.token.AccessToken, nil
}

// fetchUserEmail retrieves the authenticated user's email address.
func (om *OAuthManager) fetchUserEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", GoogleUserInfoEndpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := om.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("userinfo returned status %d", resp.StatusCode)
	}

	var userInfo struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return "", err
	}
	return userInfo.Email, nil
}

// ImportRcloneToken parses and applies a token generated from 'rclone authorize "drive"' or rclone.conf.
func (om *OAuthManager) ImportRcloneToken(ctx context.Context, rawInput string) (*OAuthToken, string, error) {
	raw := strings.TrimSpace(rawInput)
	if raw == "" {
		return nil, "", errors.New("token payload cannot be empty")
	}

	// Locate JSON block between { and }
	startIdx := strings.Index(raw, "{")
	endIdx := strings.LastIndex(raw, "}")
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return nil, "", errors.New("no valid JSON object found in input")
	}

	jsonBlob := raw[startIdx : endIdx+1]

	var parsed struct {
		AccessToken  string      `json:"access_token"`
		TokenType    string      `json:"token_type"`
		RefreshToken string      `json:"refresh_token"`
		Expiry       interface{} `json:"expiry"`
	}

	if err := json.Unmarshal([]byte(jsonBlob), &parsed); err != nil {
		return nil, "", fmt.Errorf("invalid token JSON: %w", err)
	}

	if parsed.AccessToken == "" && parsed.RefreshToken == "" {
		return nil, "", errors.New("token JSON must contain at least access_token or refresh_token")
	}

	tokenType := parsed.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}

	var expiry time.Time
	if parsed.Expiry != nil {
		switch v := parsed.Expiry.(type) {
		case string:
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				expiry = t
			} else if t, err := time.Parse("2006-01-02T15:04:05.999999999Z07:00", v); err == nil {
				expiry = t
			}
		}
	}
	if expiry.IsZero() {
		// Default 1 hour from now
		expiry = time.Now().Add(1 * time.Hour)
	}

	tok := &OAuthToken{
		AccessToken:  parsed.AccessToken,
		TokenType:    tokenType,
		RefreshToken: parsed.RefreshToken,
		Expiry:       expiry,
	}

	// Attempt to resolve email
	email := ""
	if tok.AccessToken != "" {
		em, err := om.fetchUserEmail(ctx, tok.AccessToken)
		if err == nil && em != "" {
			email = em
		}
	}
	if email == "" {
		email = "Google Account (Rclone Import)"
	}

	om.mu.Lock()
	om.token = tok
	om.email = email
	saveCb := om.onTokenSave
	om.mu.Unlock()

	if saveCb != nil {
		saveCb(tok, email)
	}

	logger.Successf("OAuth", "Successfully imported Rclone Google Drive token for account: %s", email)
	return tok, email, nil
}

