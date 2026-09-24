package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gdrive-downloader/pkg/auth"
	"gdrive-downloader/pkg/gdrive"
	"gdrive-downloader/pkg/logger"
	"gdrive-downloader/pkg/queue"
)

type Server struct {
	manager   *queue.Manager
	authMgr   *auth.Manager
	oauthMgr  *gdrive.OAuthManager
	bypassMgr *gdrive.BypassManager
	distPath  string
}

type AddRequest struct {
	Links               []string          `json:"links"`
	TargetFolder        string            `json:"target_folder"`
	ZipMode             *bool             `json:"zip_mode"`
	ConflictResolutions map[string]string `json:"conflict_resolutions"`
}

type ResolveFolderRequest struct {
	URL string `json:"url"`
}

type ConfigData struct {
	DownloadFolder    string `json:"download_folder"`
	MaxConcurrency    int    `json:"max_concurrency"`
	ChunksPerDownload int    `json:"chunks_per_download"`
	GoogleCookie      string `json:"google_cookie,omitempty"`
	HasLogin          bool   `json:"has_login"`
	AuthEnabled       bool   `json:"auth_enabled"`
	Username          string `json:"username"`
}

type FolderItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type BrowseResponse struct {
	Current string       `json:"current"`
	Parent  string       `json:"parent"`
	Drives  []FolderItem `json:"drives"`
	Folders []FolderItem `json:"folders"`
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

func NewServer(manager *queue.Manager, authMgr *auth.Manager, distPath string) *Server {
	clientID, clientSecret, redirectURI, token, email, autoBypass := authMgr.GetOAuthSettings()

	oauthMgr := gdrive.NewOAuthManager(
		clientID,
		clientSecret,
		redirectURI,
		token,
		email,
		func(t *gdrive.OAuthToken, em string) {
			_ = authMgr.SaveOAuthToken(t, em)
		},
	)

	bypassMgr := gdrive.NewBypassManager(oauthMgr, autoBypass)

	// Link bypass manager to downloader for automatic quota bypass
	manager.Downloader().SetBypassManager(bypassMgr)

	// Link chunks per download configuration from authMgr to downloader & bypassMgr
	chunks := authMgr.GetChunksPerDownload()
	manager.Downloader().SetChunksPerDownload(chunks)
	bypassMgr.SetChunksPerDownload(chunks)
	manager.DiscordDownloader().SetChunksPerDownload(chunks)

	s := &Server{
		manager:   manager,
		authMgr:   authMgr,
		oauthMgr:  oauthMgr,
		bypassMgr: bypassMgr,
		distPath:  distPath,
	}

	// Link cookie pool from authMgr to downloader with auto-failover and 24h lockout
	manager.Downloader().SetCookieProvider(
		func(excludeID ...string) (string, string, bool) {
			if entry, ok := authMgr.GetNextActiveCookie(excludeID...); ok {
				return entry.Cookie, entry.ID, true
			}
			return "", "", false
		},
		func(cookieID string) {
			_ = authMgr.MarkCookieExhausted(cookieID, 24*time.Hour)
		},
	)

	return s
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Public Auth endpoints
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)

	// Protected Auth management endpoints
	mux.HandleFunc("POST /api/auth/password", s.handleUpdateCredentials)
	mux.HandleFunc("POST /api/auth/toggle", s.handleToggleAuth)

	// Protected Download endpoints
	mux.HandleFunc("POST /api/folders/resolve", s.handleResolveFolder)
	mux.HandleFunc("POST /api/downloads/precheck", s.handlePrecheckDownloads)
	mux.HandleFunc("POST /api/downloads", s.handleAddDownloads)
	mux.HandleFunc("GET /api/downloads", s.handleGetDownloads)
	mux.HandleFunc("DELETE /api/downloads/{id}", s.handleDeleteDownload)
	mux.HandleFunc("POST /api/downloads/{id}/pause", s.handlePauseDownload)
	mux.HandleFunc("POST /api/downloads/{id}/start", s.handleStartDownload)
	mux.HandleFunc("POST /api/downloads/{id}/restart", s.handleRestartDownload)
	mux.HandleFunc("POST /api/downloads/{id}/target-folder", s.handleUpdateDownloadTargetFolder)
	mux.HandleFunc("POST /api/downloads/{id}/check", s.handleCheckDownloadFile)
	mux.HandleFunc("POST /api/downloads/check-all", s.handleCheckAllFiles)
	mux.HandleFunc("POST /api/downloads/clear", s.handleClearCompleted)
	mux.HandleFunc("POST /api/downloads/pause-all", s.handlePauseAllDownloads)
	mux.HandleFunc("POST /api/downloads/resume-all", s.handleResumeAllDownloads)
	mux.HandleFunc("GET /api/events", s.handleEvents)

	// Discord specific batch management endpoints
	mux.HandleFunc("GET /api/discord/unfinished", s.handleGetUnfinishedDiscordItems)
	mux.HandleFunc("POST /api/discord/refresh-urls", s.handleRefreshDiscordURLs)

	// Protected Config & File system
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("POST /api/config", s.handleSaveConfig)
	mux.HandleFunc("POST /api/gdrive/cookie", s.handleSetGoogleCookie)
	mux.HandleFunc("POST /api/gdrive/logout", s.handleClearGoogleCookie)
	mux.HandleFunc("GET /api/gdrive/cookies", s.handleGetGoogleCookies)
	mux.HandleFunc("POST /api/gdrive/cookies", s.handleAddGoogleCookie)
	mux.HandleFunc("POST /api/gdrive/cookies/reset-all", s.handleResetAllGoogleCookies)
	mux.HandleFunc("DELETE /api/gdrive/cookies/{id}", s.handleDeleteGoogleCookie)
	mux.HandleFunc("POST /api/gdrive/cookies/{id}/reset", s.handleResetGoogleCookie)

	// Google OAuth 2.0 & Quota Bypass endpoints
	mux.HandleFunc("GET /api/gdrive/oauth/status", s.handleGetOAuthStatus)
	mux.HandleFunc("POST /api/gdrive/oauth/config", s.handleSaveOAuthConfig)
	mux.HandleFunc("GET /api/gdrive/oauth/auth-url", s.handleGetOAuthAuthURL)
	mux.HandleFunc("GET /api/gdrive/oauth/callback", s.handleOAuthCallback)
	mux.HandleFunc("POST /api/gdrive/oauth/manual-code", s.handleOAuthManualCode)
	mux.HandleFunc("POST /api/gdrive/oauth/import-token", s.handleOAuthImportToken)
	mux.HandleFunc("POST /api/gdrive/oauth/disconnect", s.handleOAuthDisconnect)
	mux.HandleFunc("POST /api/gdrive/oauth/cleanup-temp", s.handleOAuthCleanupTemp)

	mux.HandleFunc("GET /api/fs/browse", s.handleBrowseFS)
	mux.HandleFunc("POST /api/fs/mkdir", s.handleCreateFolder)

	// Protected Logs endpoints
	mux.HandleFunc("GET /api/logs", s.handleGetLogs)
	mux.HandleFunc("DELETE /api/logs", s.handleClearLogs)

	// Static files for frontend SPA
	if s.distPath != "" {
		if _, err := os.Stat(s.distPath); err == nil {
			fileServer := http.FileServer(http.Dir(s.distPath))
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/") {
					http.NotFound(w, r)
					return
				}
				// SPA fallback to index.html if file doesn't exist
				fpath := filepath.Join(s.distPath, filepath.Clean(r.URL.Path))
				if _, err := os.Stat(fpath); os.IsNotExist(err) {
					http.ServeFile(w, r, filepath.Join(s.distPath, "index.html"))
					return
				}
				fileServer.ServeHTTP(w, r)
			})
		}
	}

	return s.corsMiddleware(s.authMiddleware(mux))
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If auth is disabled, allow all
		if !s.authMgr.IsAuthEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path

		// Publicly accessible endpoints:
		// 1. Static SPA files (/ or /assets/..., /favicon.svg, etc.)
		// 2. /api/auth/status
		// 3. /api/auth/login
		// 4. /api/gdrive/oauth/callback (Google redirects the user here)
		if !strings.HasPrefix(path, "/api/") ||
			path == "/api/auth/status" ||
			path == "/api/auth/login" ||
			path == "/api/gdrive/oauth/callback" {
			next.ServeHTTP(w, r)
			return
		}

		token := extractSessionToken(r)
		if token != "" {
			if _, ok := s.authMgr.ValidateSession(token); ok {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Unauthorized for API endpoints
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized","auth_required":true}`))
	})
}

func extractSessionToken(r *http.Request) string {
	if cookie, err := r.Cookie("gd_session"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	authEnabled := s.authMgr.IsAuthEnabled()
	var authenticated bool
	var username string

	if !authEnabled {
		authenticated = true
		username = s.authMgr.GetConfig().Username
	} else {
		token := extractSessionToken(r)
		if u, ok := s.authMgr.ValidateSession(token); ok {
			authenticated = true
			username = u
		}
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

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "gd_session",
		Value:    token,
		Path:     "/",
		MaxAge:   7 * 24 * 3600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"token":    token,
		"username": req.Username,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := extractSessionToken(r)
	if token != "" {
		s.authMgr.RevokeSession(token)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "gd_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "updated",
		"auth_enabled": req.Enabled,
	})
}

func (s *Server) handleResolveFolder(w http.ResponseWriter, r *http.Request) {
	var req ResolveFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request payload"}`, http.StatusBadRequest)
		return
	}

	url := strings.TrimSpace(req.URL)
	folderID, isFolder := gdrive.IsFolderURL(url)
	if !isFolder {
		if len(url) >= 25 && !strings.Contains(url, "/") && !strings.Contains(url, " ") {
			folderID = url
			isFolder = true
		}
	}

	if !isFolder {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"is_folder": false,
		})
		return
	}

	info, err := s.manager.Downloader().FetchFolderInfo(r.Context(), folderID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"is_folder": true,
			"error":     fmt.Sprintf("Failed to inspect Google Drive folder: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"is_folder":   true,
		"folder_id":   info.FolderID,
		"title":       info.Title,
		"files_count": len(info.Files),
		"files":       info.Files,
	})
}

func (s *Server) handlePrecheckDownloads(w http.ResponseWriter, r *http.Request) {
	var req AddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request payload"}`, http.StatusBadRequest)
		return
	}

	if len(req.Links) == 0 {
		http.Error(w, `{"error":"no links provided"}`, http.StatusBadRequest)
		return
	}

	target := req.TargetFolder
	if target == "" {
		target = s.manager.TargetFolder
	}

	zipMode := true
	if req.ZipMode != nil {
		zipMode = *req.ZipMode
	}

	conflicts, err := s.manager.PrecheckDownloads(req.Links, target, zipMode)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"has_conflicts": len(conflicts) > 0,
		"conflicts":     conflicts,
	})
}

func (s *Server) handleAddDownloads(w http.ResponseWriter, r *http.Request) {
	var req AddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request payload"}`, http.StatusBadRequest)
		return
	}

	if len(req.Links) == 0 {
		http.Error(w, `{"error":"no links provided"}`, http.StatusBadRequest)
		return
	}

	target := req.TargetFolder
	if target == "" {
		target = s.manager.TargetFolder
	}

	zipMode := true
	if req.ZipMode != nil {
		zipMode = *req.ZipMode
	}

	added, err := s.manager.AddWithResolutions(req.Links, target, zipMode, req.ConflictResolutions)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"added": len(added),
		"items": added,
	})
}

func (s *Server) handleGetDownloads(w http.ResponseWriter, r *http.Request) {
	items := s.manager.GetList()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func (s *Server) handleDeleteDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}

	deleteFile := r.URL.Query().Get("delete_file") == "true" || r.URL.Query().Get("with_file") == "true"
	if err := s.manager.Delete(id, deleteFile); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"deleted"}`))
}

func (s *Server) handlePauseDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}

	if err := s.manager.Pause(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"paused"}`))
}

func (s *Server) handleStartDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}

	if err := s.manager.Start(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"started"}`))
}

func (s *Server) handlePauseAllDownloads(w http.ResponseWriter, r *http.Request) {
	pausedCount := s.manager.PauseAll()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "paused_all",
		"count":  pausedCount,
	})
}

func (s *Server) handleResumeAllDownloads(w http.ResponseWriter, r *http.Request) {
	resumedCount := s.manager.ResumeAll()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "resumed_all",
		"count":  resumedCount,
	})
}

func (s *Server) handleRestartDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}

	if err := s.manager.Restart(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"restarted"}`))
}

func (s *Server) handleUpdateDownloadTargetFolder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		TargetFolder string `json:"target_folder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
		return
	}

	if err := s.manager.SetItemTargetFolder(id, req.TargetFolder); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated", "target_folder": req.TargetFolder})
}

func (s *Server) handleCheckDownloadFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}

	exists, err := s.manager.CheckFileExistence(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":     id,
		"exists": exists,
	})
}

func (s *Server) handleCheckAllFiles(w http.ResponseWriter, r *http.Request) {
	s.manager.CheckAllFiles()
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"checked"}`))
}

func (s *Server) handleClearCompleted(w http.ResponseWriter, r *http.Request) {
	deleteFile := r.URL.Query().Get("delete_file") == "true" || r.URL.Query().Get("with_file") == "true"
	s.manager.ClearCompleted(deleteFile)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "cleared",
	})
}

func (s *Server) handleGetUnfinishedDiscordItems(w http.ResponseWriter, r *http.Request) {
	items := s.manager.GetUnfinishedDiscordItems()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func (s *Server) handleRefreshDiscordURLs(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024*10)) // up to 10MB input
	if err != nil {
		http.Error(w, `{"error":"failed reading request"}`, http.StatusBadRequest)
		return
	}

	var urls []string

	// 1. Try parsing as JSON object { "urls": [...] } or { "raw": "..." }
	var objReq struct {
		URLs []string `json:"urls"`
		Raw  string   `json:"raw"`
	}
	if err := json.Unmarshal(bodyBytes, &objReq); err == nil {
		if len(objReq.URLs) > 0 {
			urls = append(urls, objReq.URLs...)
		}
		if objReq.Raw != "" {
			for _, line := range strings.Split(objReq.Raw, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					urls = append(urls, line)
				}
			}
		}
	} else {
		// 2. Try parsing as JSON array of strings ["https://...", ...]
		var arrReq []string
		if err := json.Unmarshal(bodyBytes, &arrReq); err == nil {
			urls = append(urls, arrReq...)
		} else {
			// 3. Try parsing as JSON array of objects [{"url": "...", "filename": "..."}, ...]
			var objArr []struct {
				URL string `json:"url"`
			}
			if err := json.Unmarshal(bodyBytes, &objArr); err == nil && len(objArr) > 0 {
				for _, item := range objArr {
					if item.URL != "" {
						urls = append(urls, item.URL)
					}
				}
			} else {
				// 4. Fallback: Parse raw plain text lines
				lines := strings.Split(string(bodyBytes), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" {
						urls = append(urls, line)
					}
				}
			}
		}
	}

	// Also extract any Discord URLs from lines if line contains extra text/markdown
	discordURLPattern := regexp.MustCompile(`https?://(?:cdn\.discordapp\.com|media\.discordapp\.net)/attachments/[^\s"'<>\\]+`)
	var cleanURLs []string
	seen := make(map[string]bool)

	for _, u := range urls {
		matches := discordURLPattern.FindAllString(u, -1)
		if len(matches) > 0 {
			for _, m := range matches {
				if !seen[m] {
					seen[m] = true
					cleanURLs = append(cleanURLs, m)
				}
			}
		} else if strings.TrimSpace(u) != "" && !seen[u] {
			seen[u] = true
			cleanURLs = append(cleanURLs, strings.TrimSpace(u))
		}
	}

	if len(cleanURLs) == 0 {
		http.Error(w, `{"error":"no valid Discord URLs provided"}`, http.StatusBadRequest)
		return
	}

	updatedCount, notFoundCount, err := s.manager.BatchUpdateDiscordURLs(cleanURLs)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"updated":    updatedCount,
		"not_found":  notFoundCount,
		"total_urls": len(cleanURLs),
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, unsubscribe := s.manager.Subscribe()
	defer unsubscribe()

	// Send immediate initial snapshot
	initialList := s.manager.GetList()
	if data, err := json.Marshal(initialList); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case items, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(items)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cookie := s.manager.Downloader().GetGoogleCookie()
	cfg := s.authMgr.GetConfig()
	res := ConfigData{
		DownloadFolder:    s.manager.TargetFolder,
		MaxConcurrency:    s.manager.MaxConcurrency,
		ChunksPerDownload: s.authMgr.GetChunksPerDownload(),
		HasLogin:          cookie != "",
		AuthEnabled:       cfg.AuthEnabled,
		Username:          cfg.Username,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
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

	// Persist to disk via authMgr
	_ = s.authMgr.UpdateConfig(cfg.DownloadFolder, cfg.MaxConcurrency, cfg.ChunksPerDownload, cfg.GoogleCookie)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

func (s *Server) handleSetGoogleCookie(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Cookie string `json:"cookie"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
		return
	}

	s.manager.Downloader().SetGoogleCookie(req.Cookie)
	_ = s.authMgr.SetGoogleCookie(req.Cookie)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

func (s *Server) handleClearGoogleCookie(w http.ResponseWriter, r *http.Request) {
	s.manager.Downloader().SetGoogleCookie("")
	_ = s.authMgr.SetGoogleCookie("")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}

type CookieResponseItem struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	MaskedCookie string `json:"masked_cookie"`
	IsExhausted  bool   `json:"is_exhausted"`
	CooldownLeft string `json:"cooldown_left,omitempty"`
}

func (s *Server) handleGetGoogleCookies(w http.ResponseWriter, r *http.Request) {
	cookies := s.authMgr.GetCookies()
	now := time.Now()
	res := make([]CookieResponseItem, len(cookies))
	for i, c := range cookies {
		masked := "configured"
		if len(c.Cookie) > 16 {
			masked = c.Cookie[:8] + "..." + c.Cookie[len(c.Cookie)-6:]
		}
		isEx := false
		cooldown := ""
		if c.ExhaustedUntil != nil && now.Before(*c.ExhaustedUntil) {
			isEx = true
			rem := c.ExhaustedUntil.Sub(now)
			hrs := int(rem.Hours())
			mins := int(rem.Minutes()) % 60
			cooldown = fmt.Sprintf("%dh %dm", hrs, mins)
		}
		res[i] = CookieResponseItem{
			ID:           c.ID,
			Label:        c.Label,
			MaskedCookie: masked,
			IsExhausted:  isEx,
			CooldownLeft: cooldown,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (s *Server) handleAddGoogleCookie(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label  string `json:"label"`
		Cookie string `json:"cookie"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
		return
	}
	entry, err := s.authMgr.AddCookie(req.Label, req.Cookie)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func (s *Server) handleDeleteGoogleCookie(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	if err := s.authMgr.RemoveCookie(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleResetGoogleCookie(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	if err := s.authMgr.ResetCookieCooldown(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleResetAllGoogleCookies(w http.ResponseWriter, r *http.Request) {
	if err := s.authMgr.ResetAllCookieCooldowns(); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleBrowseFS(w http.ResponseWriter, r *http.Request) {
	reqPath := strings.TrimSpace(r.URL.Query().Get("path"))

	drives := getAvailableDrives()

	if reqPath == "" {
		reqPath = s.manager.TargetFolder
		if reqPath == "" {
			reqPath, _ = os.UserHomeDir()
		}
	}

	cleanPath := filepath.Clean(reqPath)
	if len(cleanPath) == 2 && cleanPath[1] == ':' {
		cleanPath += `\`
	}

	var parent string
	parentDir := filepath.Dir(cleanPath)
	if parentDir != cleanPath && parentDir != "" {
		parent = parentDir
	}

	entries, err := os.ReadDir(cleanPath)
	var folders []FolderItem
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				name := e.Name()
				if strings.HasPrefix(name, "$") || strings.HasPrefix(name, ".") {
					continue
				}
				folders = append(folders, FolderItem{
					Name: name,
					Path: filepath.Join(cleanPath, name),
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(BrowseResponse{
		Current: cleanPath,
		Parent:  parent,
		Drives:  drives,
		Folders: folders,
	})
}

func (s *Server) handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Parent string `json:"parent"`
		Name   string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		http.Error(w, `{"error":"folder name cannot be empty"}`, http.StatusBadRequest)
		return
	}

	parent := strings.TrimSpace(req.Parent)
	if parent == "" {
		parent = s.manager.TargetFolder
	}

	targetPath := filepath.Join(parent, name)
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create folder: %v"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "created",
		"path":   targetPath,
		"name":   name,
	})
}

func getAvailableDrives() []FolderItem {
	var drives []FolderItem
	seen := make(map[string]bool)

	addDrive := func(name, path string) {
		clean := filepath.Clean(path)
		if runtime.GOOS != "windows" && clean == "" {
			clean = "/"
		}
		if seen[clean] {
			return
		}
		if info, err := os.Stat(clean); err == nil && info.IsDir() {
			seen[clean] = true
			drives = append(drives, FolderItem{
				Name: name,
				Path: clean,
			})
		}
	}

	if runtime.GOOS == "windows" {
		for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
			dPath := string(drive) + `:\`
			if _, err := os.Stat(dPath); err == nil {
				seen[dPath] = true
				drives = append(drives, FolderItem{
					Name: string(drive) + ":",
					Path: dPath,
				})
			}
		}
	} else {
		// 1. Root
		addDrive("Root (/)", "/")

		// 2. Common external / storage mount points
		commonMounts := []struct {
			namePrefix string
			path       string
		}{
			{"External", "/mnt"},
			{"Media", "/media"},
			{"Volumes", "/Volumes"},
			{"Volumes", "/volumes"},
			{"Storage", "/storage"},
			{"Data", "/data"},
			{"Shares", "/shares"},
			{"Srv", "/srv"},
		}

		for _, cm := range commonMounts {
			if entries, err := os.ReadDir(cm.path); err == nil {
				for _, e := range entries {
					if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
						subPath := filepath.Join(cm.path, e.Name())
						// e.g. /media/user/MyPassport -> also check 1 level down
						if subEntries, err := os.ReadDir(subPath); err == nil && cm.namePrefix == "Media" {
							for _, se := range subEntries {
								if se.IsDir() && !strings.HasPrefix(se.Name(), ".") {
									addDrive(se.Name(), filepath.Join(subPath, se.Name()))
								}
							}
						}
						addDrive(e.Name()+" ("+cm.namePrefix+")", subPath)
					}
				}
				addDrive(cm.namePrefix+" ("+cm.path+")", cm.path)
			}
		}

		// 3. Synology NAS Volume shares (/volume1, /volume2, ...)
		for i := 1; i <= 9; i++ {
			volPath := fmt.Sprintf("/volume%d", i)
			addDrive(fmt.Sprintf("Volume %d", i), volPath)
		}

		// 4. Linux /proc/mounts detection for real block devices & network shares
		if data, err := os.ReadFile("/proc/mounts"); err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			realFsTypes := map[string]bool{
				"ext4": true, "ext3": true, "ext2": true, "btrfs": true,
				"xfs": true, "zfs": true, "ntfs": true, "ntfs-3g": true,
				"exfat": true, "vfat": true, "cifs": true, "smbfs": true,
				"nfs": true, "nfs4": true, "fuseblk": true,
			}
			for scanner.Scan() {
				fields := strings.Fields(scanner.Text())
				if len(fields) >= 3 {
					dev := fields[0]
					mountPoint := fields[1]
					fsType := fields[2]

					// Ignore virtual filesystems, boot, and docker overlay internals
					if !realFsTypes[fsType] {
						continue
					}
					if strings.HasPrefix(mountPoint, "/boot") ||
						strings.HasPrefix(mountPoint, "/etc") ||
						strings.HasPrefix(mountPoint, "/var/lib/docker") ||
						mountPoint == "/" {
						continue
					}

					baseName := filepath.Base(mountPoint)
					if baseName == "/" || baseName == "." {
						baseName = dev
					}
					addDrive(fmt.Sprintf("%s (%s)", baseName, fsType), mountPoint)
				}
			}
		}
	}

	// 5. Check if user configured custom EXTRA_DRIVES or DOWNLOAD_DIR environment variable
	if extraDrives := os.Getenv("EXTRA_DRIVES"); extraDrives != "" {
		separator := string(os.PathListSeparator)
		if strings.Contains(extraDrives, ",") {
			separator = ","
		}
		for _, p := range strings.Split(extraDrives, separator) {
			p = strings.TrimSpace(p)
			if p != "" {
				addDrive(filepath.Base(p)+" (Custom)", p)
			}
		}
	}
	if dlDir := os.Getenv("DOWNLOAD_DIR"); dlDir != "" {
		addDrive("Downloads ("+filepath.Base(dlDir)+")", dlDir)
	}

	return drives
}

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 200
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
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

type SaveOAuthConfigRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
	AutoBypass   bool   `json:"auto_bypass"`
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

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	errParam := r.URL.Query().Get("error")
	if errParam != "" {
		errDesc := r.URL.Query().Get("error_description")
		if errDesc == "" {
			errDesc = errParam
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>Authentication Error</title>
<style>
body { font-family: system-ui, -apple-system, sans-serif; background: #0f172a; color: #f8fafc; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; }
.card { background: #1e293b; border: 1px solid #ef4444; border-radius: 12px; padding: 28px; max-width: 480px; text-align: center; }
h2 { color: #f87171; margin-top: 0; }
p { color: #94a3b8; font-size: 14px; }
button { margin-top: 16px; padding: 8px 20px; background: #334155; color: white; border: none; border-radius: 6px; cursor: pointer; }
</style>
</head>
<body>
<div class="card">
  <h2>Google Authorization Failed</h2>
  <p>%s</p>
  <button onclick="window.close()">Close Window</button>
</div>
</body>
</html>`, html.EscapeString(errDesc))
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code parameter", http.StatusBadRequest)
		return
	}

	redirectURI := s.getOAuthRedirectURI(r)
	clientID, clientSecret, _, _, _, _ := s.authMgr.GetOAuthSettings()
	s.oauthMgr.UpdateConfig(clientID, clientSecret, redirectURI)

	_, email, err := s.oauthMgr.ExchangeCode(r.Context(), code)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>Token Exchange Failed</title>
<style>
body { font-family: system-ui, -apple-system, sans-serif; background: #0f172a; color: #f8fafc; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; }
.card { background: #1e293b; border: 1px solid #ef4444; border-radius: 12px; padding: 28px; max-width: 480px; text-align: center; }
h2 { color: #f87171; margin-top: 0; }
p { color: #94a3b8; font-size: 14px; word-break: break-all; }
button { margin-top: 16px; padding: 8px 20px; background: #334155; color: white; border: none; border-radius: 6px; cursor: pointer; }
</style>
</head>
<body>
<div class="card">
  <h2>Token Exchange Failed</h2>
  <p>%s</p>
  <button onclick="window.close()">Close Window</button>
</div>
</body>
</html>`, html.EscapeString(err.Error()))
		return
	}

	scheme := s.getRequestScheme(r)
	fullURL := fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)

	htmlTmpl := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Google Drive Authenticated - GDDL</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      background: #090d16;
      color: #f1f5f9;
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      margin: 0;
      padding: 20px;
      box-sizing: border-box;
    }
    .card {
      background: #111827;
      border: 1px solid #10b981;
      border-radius: 16px;
      padding: 32px;
      max-width: 560px;
      width: 100%;
      box-shadow: 0 20px 40px rgba(0,0,0,0.6);
      text-align: center;
    }
    .icon {
      width: 56px;
      height: 56px;
      background: rgba(16, 185, 129, 0.15);
      color: #10b981;
      border-radius: 50%;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      font-size: 28px;
      margin-bottom: 16px;
    }
    h2 {
      margin: 0 0 8px 0;
      font-size: 22px;
      font-weight: 700;
      color: #ffffff;
    }
    .email {
      font-size: 15px;
      color: #10b981;
      font-weight: 600;
      background: rgba(16, 185, 129, 0.1);
      padding: 6px 14px;
      border-radius: 20px;
      display: inline-block;
      margin: 8px 0 16px 0;
    }
    p {
      color: #94a3b8;
      font-size: 14px;
      line-height: 1.5;
      margin: 0 0 20px 0;
    }
    .manual-box {
      background: #1e293b;
      border: 1px solid #334155;
      border-radius: 10px;
      padding: 16px;
      text-align: left;
      margin-top: 16px;
    }
    .manual-label {
      font-size: 11px;
      color: #94a3b8;
      font-weight: 600;
      text-transform: uppercase;
      margin-bottom: 8px;
      display: block;
      letter-spacing: 0.05em;
    }
    .code-input {
      width: 100%;
      background: #0f172a;
      border: 1px solid #334155;
      border-radius: 6px;
      color: #38bdf8;
      padding: 8px 10px;
      font-family: monospace;
      font-size: 12px;
      box-sizing: border-box;
      margin-bottom: 8px;
    }
    .btn-row {
      display: flex;
      gap: 10px;
      justify-content: center;
      margin-top: 20px;
    }
    .btn {
      padding: 9px 18px;
      border-radius: 8px;
      border: none;
      font-weight: 600;
      font-size: 13px;
      cursor: pointer;
      transition: all 0.2s;
    }
    .btn-primary {
      background: #10b981;
      color: #0f172a;
    }
    .btn-primary:hover {
      background: #059669;
    }
    .btn-secondary {
      background: #334155;
      color: #f1f5f9;
    }
    .btn-secondary:hover {
      background: #475569;
    }
  </style>
</head>
<body>
  <div class="card">
    <div class="icon">✓</div>
    <h2>Google Drive Connected!</h2>
    <div class="email">{{EMAIL}}</div>
    <p>Authentication was successful. GDDL can now automatically clone and bypass quota-exceeded files into <b>ggdl_temp</b>.</p>

    <div class="manual-box">
      <span class="manual-label">Callback URL / Authorization Code (rclone style)</span>
      <input type="text" id="urlInput" class="code-input" readonly value="{{FULL_URL}}" />
      <button class="btn btn-secondary" style="width: 100%;" onclick="copyURL()">Copy Callback URL</button>
    </div>

    <div class="btn-row">
      <button class="btn btn-primary" onclick="closeTab()">Close Window</button>
    </div>
  </div>

  <script>
    function copyURL() {
      const el = document.getElementById('urlInput');
      el.select();
      navigator.clipboard.writeText(el.value);
      alert('Callback URL copied to clipboard!');
    }
    function closeTab() {
      window.close();
    }
    try {
      if (window.opener) {
        window.opener.postMessage({ type: 'gdrive-oauth-success', email: '{{EMAIL}}' }, '*');
        setTimeout(function() { window.close(); }, 2500);
      }
    } catch(e) {}
  </script>
</body>
</html>`

	htmlOut := strings.ReplaceAll(htmlTmpl, "{{EMAIL}}", html.EscapeString(email))
	htmlOut = strings.ReplaceAll(htmlOut, "{{FULL_URL}}", html.EscapeString(fullURL))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(htmlOut))
}

type ManualCodeRequest struct {
	Code string `json:"code"`
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

type ImportTokenRequest struct {
	Token string `json:"token"`
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
