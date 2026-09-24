package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"gdrive-downloader/pkg/gdrive"
)

type OAuthStatusResponse struct {
	Configured          bool       `json:"configured"`
	Connected           bool       `json:"connected"`
	ClientID            string     `json:"client_id,omitempty"`
	RedirectURI         string     `json:"redirect_uri"`
	RedirectURIOverride string     `json:"redirect_uri_override,omitempty"`
	Email               string     `json:"email"`
	Expiry              *time.Time `json:"expiry,omitempty"`
	AutoBypass          bool       `json:"auto_bypass"`
}

type SaveOAuthConfigRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
	AutoBypass   bool   `json:"auto_bypass"`
}

type ManualCodeRequest struct {
	Code string `json:"code"`
}

type ImportTokenRequest struct {
	Token string `json:"token"`
}

func (s *Server) handleGetOAuthStatus(w http.ResponseWriter, r *http.Request) {
	isConfigured, isConnected, email, expiry, clientID := s.oauthMgr.GetStatus()
	autoBypass := s.bypassMgr.IsAutoBypass()
	redirectURI := s.getOAuthRedirectURI(r)

	res := OAuthStatusResponse{
		Configured:          isConfigured,
		Connected:           isConnected,
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		RedirectURIOverride: s.authMgr.GetOAuthRedirectURI(),
		Email:               email,
		Expiry:              expiry,
		AutoBypass:          autoBypass,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (s *Server) handleSaveOAuthConfig(w http.ResponseWriter, r *http.Request) {
	var req SaveOAuthConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
		return
	}

	req.ClientID = strings.TrimSpace(req.ClientID)
	req.ClientSecret = strings.TrimSpace(req.ClientSecret)
	req.RedirectURI = strings.TrimSpace(req.RedirectURI)

	_ = s.authMgr.SetOAuthCredentials(req.ClientID, req.ClientSecret, req.RedirectURI, req.AutoBypass)
	s.oauthMgr.UpdateConfig(req.ClientID, req.ClientSecret, req.RedirectURI)
	s.bypassMgr.SetAutoBypass(req.AutoBypass)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) getRequestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	return "http"
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	privateBlocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"fc00::/7",
		"fe80::/10",
	}
	for _, cidr := range privateBlocks {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil && block.Contains(ip) {
			return true
		}
	}
	return false
}

func (s *Server) getOAuthRedirectURI(r *http.Request) string {
	if s.authMgr != nil {
		if override := s.authMgr.GetOAuthRedirectURI(); override != "" {
			return override
		}
	}

	scheme := s.getRequestScheme(r)
	host := r.Host

	h, port, err := net.SplitHostPort(host)
	if err != nil {
		h = host
		port = ""
	}

	ip := net.ParseIP(h)
	// Google OAuth2 blocks private IPs (RFC 1918) like 192.168.x.x with Error 400: invalid_request.
	// For local development and private network usage, substitute with localhost.
	if (ip != nil && isPrivateIP(ip)) || h == "localhost" || h == "127.0.0.1" {
		if port != "" {
			return fmt.Sprintf("%s://localhost:%s/api/gdrive/oauth/callback", scheme, port)
		}
		return fmt.Sprintf("%s://localhost/api/gdrive/oauth/callback", scheme)
	}

	return fmt.Sprintf("%s://%s/api/gdrive/oauth/callback", scheme, host)
}

func (s *Server) handleGetOAuthAuthURL(w http.ResponseWriter, r *http.Request) {
	redirectURI := s.getOAuthRedirectURI(r)

	// Update OAuth manager with active redirectURI
	clientID, clientSecret, _, _, _, _ := s.authMgr.GetOAuthSettings()
	s.oauthMgr.UpdateConfig(clientID, clientSecret, redirectURI)

	authURL, err := s.oauthMgr.BuildAuthURL("gddl_state")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"auth_url":     authURL,
		"redirect_uri": redirectURI,
	})
}

func (s *Server) handleOAuthManualCode(w http.ResponseWriter, r *http.Request) {
	var req ManualCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
		return
	}

	code := gdrive.ExtractCode(req.Code)
	if code == "" {
		http.Error(w, `{"error":"authorization code or URL is empty"}`, http.StatusBadRequest)
		return
	}

	redirectURI := s.getOAuthRedirectURI(r)
	clientID, clientSecret, _, _, _, _ := s.authMgr.GetOAuthSettings()
	s.oauthMgr.UpdateConfig(clientID, clientSecret, redirectURI)

	_, email, err := s.oauthMgr.ExchangeCode(r.Context(), code)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"email":   email,
	})
}

func (s *Server) handleOAuthImportToken(w http.ResponseWriter, r *http.Request) {
	var req ImportTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
		return
	}

	tok, email, err := s.oauthMgr.ImportRcloneToken(r.Context(), req.Token)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"email":   email,
		"expiry":  tok.Expiry,
	})
}

func (s *Server) handleOAuthDisconnect(w http.ResponseWriter, r *http.Request) {
	s.oauthMgr.ClearToken()
	_ = s.authMgr.ClearOAuthToken()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleOAuthCleanupTemp(w http.ResponseWriter, r *http.Request) {
	count, err := s.bypassMgr.EmptyTempFolder(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"deleted_count": count,
	})
}
