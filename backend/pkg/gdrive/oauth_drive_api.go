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
	"time"

	"gdrive-downloader/pkg/logger"
)

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
		expiry = time.Now().Add(1 * time.Hour)
	}

	tok := &OAuthToken{
		AccessToken:  parsed.AccessToken,
		TokenType:    tokenType,
		RefreshToken: parsed.RefreshToken,
		Expiry:       expiry,
	}

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
