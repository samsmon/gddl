package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"gdrive-downloader/pkg/auth"
	"gdrive-downloader/pkg/gdrive"
	"gdrive-downloader/pkg/logger"
	"gdrive-downloader/pkg/queue"
)

type Server struct {
	manager  *queue.Manager
	authMgr  *auth.Manager
	distPath string
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
	DownloadFolder string `json:"download_folder"`
	MaxConcurrency int    `json:"max_concurrency"`
	GoogleCookie   string `json:"google_cookie,omitempty"`
	HasLogin       bool   `json:"has_login"`
	AuthEnabled    bool   `json:"auth_enabled"`
	Username       string `json:"username"`
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
	return &Server{
		manager:  manager,
		authMgr:  authMgr,
		distPath: distPath,
	}
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
	mux.HandleFunc("POST /api/downloads/{id}/check", s.handleCheckDownloadFile)
	mux.HandleFunc("POST /api/downloads/check-all", s.handleCheckAllFiles)
	mux.HandleFunc("POST /api/downloads/clear", s.handleClearCompleted)
	mux.HandleFunc("GET /api/events", s.handleEvents)

	// Protected Config & File system
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("POST /api/config", s.handleSaveConfig)
	mux.HandleFunc("POST /api/gdrive/cookie", s.handleSetGoogleCookie)
	mux.HandleFunc("POST /api/gdrive/logout", s.handleClearGoogleCookie)
	mux.HandleFunc("GET /api/fs/browse", s.handleBrowseFS)

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
		if !strings.HasPrefix(path, "/api/") ||
			path == "/api/auth/status" ||
			path == "/api/auth/login" {
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
		DownloadFolder: s.manager.TargetFolder,
		MaxConcurrency: s.manager.MaxConcurrency,
		HasLogin:       cookie != "",
		AuthEnabled:    cfg.AuthEnabled,
		Username:       cfg.Username,
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
	if cfg.GoogleCookie != "" {
		s.manager.Downloader().SetGoogleCookie(cfg.GoogleCookie)
	}

	// Persist to disk via authMgr
	_ = s.authMgr.UpdateConfig(cfg.DownloadFolder, cfg.MaxConcurrency, cfg.GoogleCookie)

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
