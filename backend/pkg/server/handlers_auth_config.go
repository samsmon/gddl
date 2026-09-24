package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"gdrive-downloader/pkg/logger"
	"gdrive-downloader/pkg/warp"
)

type ConfigData struct {
	DownloadFolder     string  `json:"download_folder"`
	MaxConcurrency     int     `json:"max_concurrency"`
	ChunksPerDownload  int     `json:"chunks_per_download"`
	GoogleCookie       string  `json:"google_cookie,omitempty"`
	HasLogin           bool    `json:"has_login"`
	AuthEnabled        bool    `json:"auth_enabled"`
	Username           string  `json:"username"`
	AutoWarpEnabled    *bool   `json:"auto_warp_enabled,omitempty"`
	AutoWarpMinSpeedMB float64 `json:"auto_warp_min_speed_mb,omitempty"`
	WarpProxyPort      int     `json:"warp_proxy_port,omitempty"`
	CustomProxyURL     string  `json:"custom_proxy_url,omitempty"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type PasswordChangeRequest struct {
	OldPassword string `json:"old_password"`
	NewUsername string `json:"new_username"`
	NewPassword string `json:"new_password"`
}

type AuthToggleRequest struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	authEnabled := s.authMgr.IsAuthEnabled()
	var authenticated bool
	var username string
	if !authEnabled {
		authenticated = true
		username = s.authMgr.GetConfig().Username
	} else if u, ok := s.authMgr.ValidateSession(extractSessionToken(r)); ok {
		authenticated = true
		username = u
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"auth_enabled":  authEnabled,
		"authenticated": authenticated,
		"username":      username,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
		return
	}
	if !s.authMgr.Authenticate(req.Username, req.Password) {
		http.Error(w, `{"error":"invalid username or password"}`, http.StatusUnauthorized)
		return
	}

	token := s.authMgr.CreateSession(req.Username)
	http.SetCookie(w, &http.Cookie{
		Name: "gd_session", Value: token, Path: "/",
		MaxAge: 7 * 24 * 3600, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok", "token": token, "username": req.Username,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if token := extractSessionToken(r); token != "" {
		s.authMgr.RevokeSession(token)
	}
	http.SetCookie(w, &http.Cookie{
		Name: "gd_session", Value: "", Path: "/",
		MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "logged_out"})
}

func (s *Server) handleUpdateCredentials(w http.ResponseWriter, r *http.Request) {
	var req PasswordChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
		return
	}
	if err := s.authMgr.UpdateCredentials(req.OldPassword, req.NewUsername, req.NewPassword); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (s *Server) handleToggleAuth(w http.ResponseWriter, r *http.Request) {
	var req AuthToggleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
		return
	}
	if err := s.authMgr.SetAuthEnabled(req.Enabled); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "updated", "auth_enabled": req.Enabled})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cookie := s.manager.Downloader().GetGoogleCookie()
	cfg := s.authMgr.GetConfig()
	autoWarp, minSpeedMB, warpPort, customProxy := s.authMgr.GetWarpSettings()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ConfigData{
		DownloadFolder:     s.manager.TargetFolder,
		MaxConcurrency:     s.manager.MaxConcurrency,
		ChunksPerDownload:  s.authMgr.GetChunksPerDownload(),
		HasLogin:           cookie != "",
		AuthEnabled:        cfg.AuthEnabled,
		Username:           cfg.Username,
		AutoWarpEnabled:    &autoWarp,
		AutoWarpMinSpeedMB: minSpeedMB,
		WarpProxyPort:      warpPort,
		CustomProxyURL:     customProxy,
	})
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	var cfg ConfigData
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, `{"error":"invalid config"}`, http.StatusBadRequest)
		return
	}
	if cfg.DownloadFolder != "" {
		s.manager.TargetFolder = cfg.DownloadFolder
	}
	if cfg.MaxConcurrency > 0 {
		s.manager.MaxConcurrency = cfg.MaxConcurrency
	}
	if cfg.ChunksPerDownload > 0 {
		s.manager.Downloader().SetChunksPerDownload(cfg.ChunksPerDownload)
		s.bypassMgr.SetChunksPerDownload(cfg.ChunksPerDownload)
		s.manager.DiscordDownloader().SetChunksPerDownload(cfg.ChunksPerDownload)
	}
	if cfg.GoogleCookie != "" {
		s.manager.Downloader().SetGoogleCookie(cfg.GoogleCookie)
	}

	curAuto, curMinSpeed, curPort, _ := s.authMgr.GetWarpSettings()
	if cfg.AutoWarpEnabled != nil {
		curAuto = *cfg.AutoWarpEnabled
	}
	if cfg.AutoWarpMinSpeedMB > 0 {
		curMinSpeed = cfg.AutoWarpMinSpeedMB
	}
	if cfg.WarpProxyPort > 0 {
		curPort = cfg.WarpProxyPort
	}
	_ = s.authMgr.SaveWarpSettings(curAuto, curMinSpeed, curPort, cfg.CustomProxyURL)
	if s.warpCtrl != nil {
		s.warpCtrl.UpdateConfig(curAuto, curMinSpeed, curPort, cfg.CustomProxyURL)
	}
	_ = s.authMgr.UpdateConfig(cfg.DownloadFolder, cfg.MaxConcurrency, cfg.ChunksPerDownload, cfg.GoogleCookie)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

func (s *Server) handleGetWarpStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.warpCtrl == nil {
		json.NewEncoder(w).Encode(warp.Status{})
		return
	}
	json.NewEncoder(w).Encode(s.warpCtrl.GetStatus())
}

func (s *Server) handleToggleWarpProxy(w http.ResponseWriter, r *http.Request) {
	if s.warpCtrl == nil {
		http.Error(w, `{"error":"warp controller unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		ProxyActive *bool `json:"proxy_active"`
		AutoEnabled *bool `json:"auto_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
		return
	}
	if req.AutoEnabled != nil {
		_, minSpeed, port, customProxy := s.authMgr.GetWarpSettings()
		_ = s.authMgr.SaveWarpSettings(*req.AutoEnabled, minSpeed, port, customProxy)
		s.warpCtrl.UpdateConfig(*req.AutoEnabled, minSpeed, port, customProxy)
	}
	if req.ProxyActive != nil {
		if err := s.warpCtrl.SetProxyActive(*req.ProxyActive); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.warpCtrl.GetStatus())
}

func (s *Server) handleRotateWarpIP(w http.ResponseWriter, r *http.Request) {
	if s.warpCtrl == nil {
		http.Error(w, `{"error":"warp controller unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	rotated, err := s.warpCtrl.TriggerAutoBypassOrRotate("Manual IP rotation requested from Web UI", true)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"rotated": rotated, "status": s.warpCtrl.GetStatus()})
}

func (s *Server) handleSaveWarpConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AutoEnabled    bool    `json:"auto_enabled"`
		MinSpeedMB     float64 `json:"min_speed_mb"`
		ProxyPort      int     `json:"proxy_port"`
		CustomProxyURL string  `json:"custom_proxy_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
		return
	}
	if req.MinSpeedMB <= 0 {
		req.MinSpeedMB = 5.0
	}
	if req.ProxyPort <= 0 {
		req.ProxyPort = 40000
	}
	_ = s.authMgr.SaveWarpSettings(req.AutoEnabled, req.MinSpeedMB, req.ProxyPort, req.CustomProxyURL)
	if s.warpCtrl != nil {
		s.warpCtrl.UpdateConfig(req.AutoEnabled, req.MinSpeedMB, req.ProxyPort, req.CustomProxyURL)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.warpCtrl.GetStatus())
}

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}
	entries := logger.GetEntries(limit)
	if entries == nil {
		entries = []logger.LogEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (s *Server) handleClearLogs(w http.ResponseWriter, r *http.Request) {
	logger.Clear()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
