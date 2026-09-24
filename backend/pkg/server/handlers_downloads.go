package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gdrive-downloader/pkg/gdrive"
)

type AddRequest struct {
	Links               []string          `json:"links"`
	TargetFolder        string            `json:"target_folder"`
	ZipMode             *bool             `json:"zip_mode"`
	ConflictResolutions map[string]string `json:"conflict_resolutions"`
}

type ResolveFolderRequest struct {
	URL string `json:"url"`
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
