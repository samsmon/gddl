package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gdrive-downloader/pkg/auth"
	"gdrive-downloader/pkg/gdrive"
	"gdrive-downloader/pkg/queue"
	"gdrive-downloader/pkg/warp"
)

type Server struct {
	manager   *queue.Manager
	authMgr   *auth.Manager
	oauthMgr  *gdrive.OAuthManager
	bypassMgr *gdrive.BypassManager
	warpCtrl  *warp.Controller
	distPath  string
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

	// Initialize Cloudflare WARP & Anti-Throttle controller
	autoWarp, minSpeedMB, warpPort, customProxy := authMgr.GetWarpSettings()
	warpCtrl := warp.NewController(autoWarp, minSpeedMB, warpPort, customProxy)
	manager.SetWarpController(warpCtrl)

	s := &Server{
		manager:   manager,
		authMgr:   authMgr,
		oauthMgr:  oauthMgr,
		bypassMgr: bypassMgr,
		warpCtrl:  warpCtrl,
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

	// Cloudflare WARP & Anti-Throttle Auto-Bypass endpoints
	mux.HandleFunc("GET /api/warp/status", s.handleGetWarpStatus)
	mux.HandleFunc("POST /api/warp/toggle", s.handleToggleWarpProxy)
	mux.HandleFunc("POST /api/warp/rotate", s.handleRotateWarpIP)
	mux.HandleFunc("POST /api/warp/config", s.handleSaveWarpConfig)

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
