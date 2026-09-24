package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type VersionInfo struct {
	Version   string   `json:"version"`
	Commit    string   `json:"commit"`
	Date      string   `json:"date"`
	Title     string   `json:"title"`
	Message   string   `json:"message"`
	Changelog []string `json:"changelog"`
}

type UpdateCheckResult struct {
	UpdateAvailable bool     `json:"update_available"`
	CurrentVersion  string   `json:"current_version"`
	CurrentCommit   string   `json:"current_commit"`
	LatestVersion   string   `json:"latest_version"`
	LatestCommit    string   `json:"latest_commit"`
	LatestTitle     string   `json:"latest_title"`
	LatestMessage   string   `json:"latest_message"`
	Changelog       []string `json:"changelog"`
	PullCommand     string   `json:"pull_command"`
	RebuildCommand  string   `json:"rebuild_command"`
	Error           string   `json:"error,omitempty"`
}

func getLocalVersion() VersionInfo {
	info := VersionInfo{
		Version: "1.3.0",
		Commit:  "1e382c5",
		Date:    "2026-09-24",
		Title:   "IDM-Style Categories & Streamlined Toolbar",
		Message: "feat(ui): add IDM-style categories (Unfinished, Finished, Sources) and streamline toolbar",
		Changelog: []string{
			"Added IDM-style Unfinished and Finished categories in sidebar",
			"Added Google Drive and Discord CDN source filter categories",
			"Removed redundant Hide Completed button from toolbar for cleaner UI",
			"Categorized Settings Modal with internal sidebar (General, WARP, Cookies, Security)",
			"Added live real-time update checker with commit changelog and git pull instructions",
		},
	}

	// Try reading version.json from relative or executable locations
	candidates := []string{
		"version.json",
		"../version.json",
		"../../version.json",
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "version.json"))
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", "version.json"))
	}

	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			var parsed VersionInfo
			if err := json.Unmarshal(data, &parsed); err == nil {
				info = parsed
				break
			}
		}
	}

	// If git is available, query actual local HEAD commit hash
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD")
	if out, err := cmd.Output(); err == nil {
		gitCommit := strings.TrimSpace(string(out))
		if gitCommit != "" {
			info.Commit = gitCommit
		}
	}

	return info
}

func (s *Server) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(getLocalVersion())
}

func (s *Server) handleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	local := getLocalVersion()

	res := UpdateCheckResult{
		UpdateAvailable: false,
		CurrentVersion:  local.Version,
		CurrentCommit:   local.Commit,
		LatestVersion:   local.Version,
		LatestCommit:    local.Commit,
		LatestTitle:     local.Title,
		LatestMessage:   local.Message,
		Changelog:       local.Changelog,
		PullCommand:     "git pull origin main",
		RebuildCommand:  "npm run build --prefix frontend && go build -o gddl.exe ./backend",
	}

	// 1. Fetch remote version.json from GitHub raw content
	client := &http.Client{Timeout: 6 * time.Second}
	req, err := http.NewRequestWithContext(r.Context(), "GET", "https://raw.githubusercontent.com/samsmon/gddl/main/version.json", nil)
	if err == nil {
		req.Header.Set("User-Agent", "GDDL-Updater/1.0")
		req.Header.Set("Cache-Control", "no-cache")
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var remote VersionInfo
			if err := json.NewDecoder(resp.Body).Decode(&remote); err == nil {
				res.LatestVersion = remote.Version
				res.LatestCommit = remote.Commit
				res.LatestTitle = remote.Title
				res.LatestMessage = remote.Message
				if len(remote.Changelog) > 0 {
					res.Changelog = remote.Changelog
				}

				// Check if remote commit or version is different/newer
				isDifferentCommit := remote.Commit != "" && local.Commit != "" && !strings.EqualFold(remote.Commit, local.Commit)
				isDifferentVersion := remote.Version != "" && local.Version != "" && remote.Version != local.Version
				if isDifferentCommit || isDifferentVersion {
					res.UpdateAvailable = true
				}
				_ = json.NewEncoder(w).Encode(res)
				return
			}
		}
	}

	// 2. Fallback: Query GitHub API commits
	ghReq, err := http.NewRequestWithContext(r.Context(), "GET", "https://api.github.com/repos/samsmon/gddl/commits/main", nil)
	if err == nil {
		ghReq.Header.Set("User-Agent", "GDDL-Updater/1.0")
		ghReq.Header.Set("Accept", "application/vnd.github.v3+json")
		resp, err := client.Do(ghReq)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			bodyBytes, _ := io.ReadAll(resp.Body)
			var ghCommit struct {
				Sha    string `json:"sha"`
				Commit struct {
					Message string `json:"message"`
				} `json:"commit"`
			}
			if err := json.Unmarshal(bodyBytes, &ghCommit); err == nil && ghCommit.Sha != "" {
				shortSha := ghCommit.Sha
				if len(shortSha) > 7 {
					shortSha = shortSha[:7]
				}
				res.LatestCommit = shortSha
				msg := strings.TrimSpace(ghCommit.Commit.Message)
				res.LatestMessage = msg
				firstLine := strings.Split(msg, "\n")[0]
				res.LatestTitle = firstLine

				if local.Commit != "" && !strings.EqualFold(shortSha, local.Commit) {
					res.UpdateAvailable = true
				}
				_ = json.NewEncoder(w).Encode(res)
				return
			}
		}
	}

	// Return local state if remote check couldn't reach GitHub
	_ = json.NewEncoder(w).Encode(res)
}
