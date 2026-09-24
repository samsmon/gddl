package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

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

type CookieResponseItem struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	MaskedCookie string `json:"masked_cookie"`
	IsExhausted  bool   `json:"is_exhausted"`
	CooldownLeft string `json:"cooldown_left,omitempty"`
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

func (s *Server) handleGetGoogleCookies(w http.ResponseWriter, r *http.Request) {
	cookies := s.authMgr.GetCookies()
	now := time.Now()
	res := make([]CookieResponseItem, len(cookies))
	for i, c := range cookies {
		masked := "configured"
		if len(c.Cookie) > 16 {
			masked = c.Cookie[:8] + "..." + c.Cookie[len(c.Cookie)-6:]
		}
		isEx, cooldown := false, ""
		if c.ExhaustedUntil != nil && now.Before(*c.ExhaustedUntil) {
			isEx = true
			rem := c.ExhaustedUntil.Sub(now)
			cooldown = fmt.Sprintf("%dh %dm", int(rem.Hours()), int(rem.Minutes())%60)
		}
		res[i] = CookieResponseItem{
			ID: c.ID, Label: c.Label, MaskedCookie: masked,
			IsExhausted: isEx, CooldownLeft: cooldown,
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
		if reqPath = s.manager.TargetFolder; reqPath == "" {
			reqPath, _ = os.UserHomeDir()
		}
	}

	cleanPath := filepath.Clean(reqPath)
	if len(cleanPath) == 2 && cleanPath[1] == ':' {
		cleanPath += `\`
	}

	var parent string
	if parentDir := filepath.Dir(cleanPath); parentDir != cleanPath && parentDir != "" {
		parent = parentDir
	}

	var folders []FolderItem
	if entries, err := os.ReadDir(cleanPath); err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), "$") && !strings.HasPrefix(e.Name(), ".") {
				folders = append(folders, FolderItem{Name: e.Name(), Path: filepath.Join(cleanPath, e.Name())})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(BrowseResponse{Current: cleanPath, Parent: parent, Drives: drives, Folders: folders})
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
	json.NewEncoder(w).Encode(map[string]string{"status": "created", "path": targetPath, "name": name})
}

func getAvailableDrives() []FolderItem {
	var drives []FolderItem
	seen := make(map[string]bool)

	addDrive := func(name, path string) {
		clean := filepath.Clean(path)
		if runtime.GOOS != "windows" && clean == "" {
			clean = "/"
		}
		if !seen[clean] {
			if info, err := os.Stat(clean); err == nil && info.IsDir() {
				seen[clean] = true
				drives = append(drives, FolderItem{Name: name, Path: clean})
			}
		}
	}

	if runtime.GOOS == "windows" {
		for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
			dPath := string(drive) + `:\`
			if _, err := os.Stat(dPath); err == nil {
				seen[dPath] = true
				drives = append(drives, FolderItem{Name: string(drive) + ":", Path: dPath})
			}
		}
	} else {
		addDrive("Root (/)", "/")
		commonMounts := []struct{ namePrefix, path string }{
			{"External", "/mnt"}, {"Media", "/media"}, {"Volumes", "/Volumes"}, {"Volumes", "/volumes"},
			{"Storage", "/storage"}, {"Data", "/data"}, {"Shares", "/shares"}, {"Srv", "/srv"},
		}
		for _, cm := range commonMounts {
			if entries, err := os.ReadDir(cm.path); err == nil {
				for _, e := range entries {
					if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
						subPath := filepath.Join(cm.path, e.Name())
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
		for i := 1; i <= 9; i++ {
			addDrive(fmt.Sprintf("Volume %d", i), fmt.Sprintf("/volume%d", i))
		}
		if data, err := os.ReadFile("/proc/mounts"); err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			realFsTypes := map[string]bool{
				"ext4": true, "ext3": true, "ext2": true, "btrfs": true, "xfs": true, "zfs": true,
				"ntfs": true, "ntfs-3g": true, "exfat": true, "vfat": true, "cifs": true,
				"smbfs": true, "nfs": true, "nfs4": true, "fuseblk": true,
			}
			for scanner.Scan() {
				if fields := strings.Fields(scanner.Text()); len(fields) >= 3 {
					dev, mountPoint, fsType := fields[0], fields[1], fields[2]
					if !realFsTypes[fsType] || strings.HasPrefix(mountPoint, "/boot") || strings.HasPrefix(mountPoint, "/etc") || strings.HasPrefix(mountPoint, "/var/lib/docker") || mountPoint == "/" {
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

	if extraDrives := os.Getenv("EXTRA_DRIVES"); extraDrives != "" {
		separator := string(os.PathListSeparator)
		if strings.Contains(extraDrives, ",") {
			separator = ","
		}
		for _, p := range strings.Split(extraDrives, separator) {
			if p = strings.TrimSpace(p); p != "" {
				addDrive(filepath.Base(p)+" (Custom)", p)
			}
		}
	}
	if dlDir := os.Getenv("DOWNLOAD_DIR"); dlDir != "" {
		addDrive("Downloads ("+filepath.Base(dlDir)+")", dlDir)
	}

	return drives
}
