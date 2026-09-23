package gdrive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gdrive-downloader/pkg/logger"
)

const (
	TempFolderName = "ggdl_temp"
)

// BypassManager handles cloning quota-exceeded Google Drive files to a temporary
// 'ggdl_temp' folder in the user's personal Google Drive, downloading them cleanly,
// and permanently purging the cloned copy after completion.
type BypassManager struct {
	oauthMgr     *OAuthManager
	httpClient   *http.Client
	folderMu     sync.RWMutex
	tempFolderID string
	autoBypass   bool
}

// NewBypassManager creates a new BypassManager with the provided OAuthManager.
func NewBypassManager(oauthMgr *OAuthManager, autoBypass bool) *BypassManager {
	return &BypassManager{
		oauthMgr:   oauthMgr,
		autoBypass: autoBypass,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// IsAvailable returns true if OAuth is connected.
func (bm *BypassManager) IsAvailable() bool {
	if bm == nil || bm.oauthMgr == nil {
		return false
	}
	return bm.oauthMgr.IsConnected()
}

// IsAutoBypass returns whether auto-bypass on quota exceeded is enabled.
func (bm *BypassManager) IsAutoBypass() bool {
	if bm == nil {
		return false
	}
	bm.folderMu.RLock()
	defer bm.folderMu.RUnlock()
	return bm.autoBypass
}

// SetAutoBypass enables or disables auto-bypass.
func (bm *BypassManager) SetAutoBypass(enabled bool) {
	if bm == nil {
		return
	}
	bm.folderMu.Lock()
	defer bm.folderMu.Unlock()
	bm.autoBypass = enabled
}

// EnsureTempFolder locates or creates the 'ggdl_temp' folder in the user's Google Drive root.
func (bm *BypassManager) EnsureTempFolder(ctx context.Context) (string, error) {
	bm.folderMu.RLock()
	cachedID := bm.tempFolderID
	bm.folderMu.RUnlock()

	token, err := bm.oauthMgr.GetValidAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("OAuth authentication required: %w", err)
	}

	// 1. If we have a cached folder ID, verify it still exists
	if cachedID != "" {
		verifyURL := fmt.Sprintf("%s/files/%s?fields=id,name,trashed", GoogleDriveAPIEndpoint, cachedID)
		req, err := http.NewRequestWithContext(ctx, "GET", verifyURL, nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := bm.httpClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var item struct {
						ID      string `json:"id"`
						Name    string `json:"name"`
						Trashed bool   `json:"trashed"`
					}
					if err := json.NewDecoder(resp.Body).Decode(&item); err == nil && !item.Trashed {
						return item.ID, nil
					}
				}
			}
		}
	}

	// 2. Search for existing 'ggdl_temp' folder
	query := fmt.Sprintf("name = '%s' and mimeType = 'application/vnd.google-apps.folder' and trashed = false and 'root' in parents", TempFolderName)
	searchURL := fmt.Sprintf("%s/files?q=%s&fields=files(id,name)&spaces=drive", GoogleDriveAPIEndpoint, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := bm.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed searching for %s folder: %w", TempFolderName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Google Drive API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var searchRes struct {
		Files []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchRes); err != nil {
		return "", fmt.Errorf("failed decoding folder search response: %w", err)
	}

	if len(searchRes.Files) > 0 {
		folderID := searchRes.Files[0].ID
		bm.folderMu.Lock()
		bm.tempFolderID = folderID
		bm.folderMu.Unlock()
		logger.Infof("Bypass", "Found existing '%s' folder in Google Drive (ID: %s)", TempFolderName, folderID)
		return folderID, nil
	}

	// 3. Create 'ggdl_temp' folder if not found
	logger.Infof("Bypass", "Creating '%s' folder in Google Drive root...", TempFolderName)
	createBody := map[string]interface{}{
		"name":     TempFolderName,
		"mimeType": "application/vnd.google-apps.folder",
		"parents":  []string{"root"},
	}
	bodyBytes, _ := json.Marshal(createBody)

	createReq, err := http.NewRequestWithContext(ctx, "POST", GoogleDriveAPIEndpoint+"/files?fields=id,name", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := bm.httpClient.Do(createReq)
	if err != nil {
		return "", fmt.Errorf("failed creating %s folder: %w", TempFolderName, err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode < 200 || createResp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(createResp.Body)
		return "", fmt.Errorf("failed to create %s folder (HTTP %d): %s", TempFolderName, createResp.StatusCode, string(respBody))
	}

	var createRes struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createRes); err != nil {
		return "", fmt.Errorf("failed decoding create folder response: %w", err)
	}

	bm.folderMu.Lock()
	bm.tempFolderID = createRes.ID
	bm.folderMu.Unlock()

	logger.Successf("Bypass", "Created '%s' folder in Google Drive (ID: %s)", TempFolderName, createRes.ID)
	return createRes.ID, nil
}

// CloneFileToTemp makes a copy of sourceFileID into the 'ggdl_temp' folder.
func (bm *BypassManager) CloneFileToTemp(ctx context.Context, sourceFileID string) (clonedID string, filename string, size int64, err error) {
	folderID, err := bm.EnsureTempFolder(ctx)
	if err != nil {
		return "", "", 0, err
	}

	token, err := bm.oauthMgr.GetValidAccessToken(ctx)
	if err != nil {
		return "", "", 0, fmt.Errorf("OAuth token error: %w", err)
	}

	copyURL := fmt.Sprintf("%s/files/%s/copy?supportsAllDrives=true&fields=id,name,size,mimeType", GoogleDriveAPIEndpoint, sourceFileID)

	copyBody := map[string]interface{}{
		"parents": []string{folderID},
	}
	bodyBytes, _ := json.Marshal(copyBody)

	req, err := http.NewRequestWithContext(ctx, "POST", copyURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := bm.httpClient.Do(req)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed sending copy request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed reading copy response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var gErr struct {
			Error struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &gErr)
		if gErr.Error.Message != "" {
			return "", "", 0, fmt.Errorf("Google Drive copy error (%d): %s", gErr.Error.Code, gErr.Error.Message)
		}
		return "", "", 0, fmt.Errorf("Google Drive copy failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		ID       string          `json:"id"`
		Name     string          `json:"name"`
		Size     json.RawMessage `json:"size"`
		MimeType string          `json:"mimeType"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", "", 0, fmt.Errorf("failed parsing copy result: %w", err)
	}

	if res.ID == "" {
		return "", "", 0, errors.New("Google Drive did not return a cloned file ID")
	}

	var parsedSize int64
	if len(res.Size) > 0 {
		sStr := strings.Trim(string(res.Size), "\"")
		if s, err := strconv.ParseInt(sStr, 10, 64); err == nil {
			parsedSize = s
		}
	}

	cleanName := res.Name
	// Strip "Copy of " or "Salinan dari " if Google prepended it
	for _, prefix := range []string{"Copy of ", "Salinan dari ", "Copia de "} {
		if strings.HasPrefix(cleanName, prefix) {
			cleanName = strings.TrimPrefix(cleanName, prefix)
			break
		}
	}

	logger.Successf("Bypass", "Cloned file %s into %s as '%s' (Cloned ID: %s, Size: %d bytes)", sourceFileID, TempFolderName, cleanName, res.ID, parsedSize)
	return res.ID, cleanName, parsedSize, nil
}

// DeleteFile permanently deletes a file from Google Drive (skips trash to keep drive clean).
func (bm *BypassManager) DeleteFile(ctx context.Context, fileID string) error {
	token, err := bm.oauthMgr.GetValidAccessToken(ctx)
	if err != nil {
		return err
	}

	delURL := fmt.Sprintf("%s/files/%s?supportsAllDrives=true", GoogleDriveAPIEndpoint, fileID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", delURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := bm.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete file %s (HTTP %d): %s", fileID, resp.StatusCode, string(body))
	}

	return nil
}

// EmptyTempFolder deletes all files currently inside the 'ggdl_temp' folder.
func (bm *BypassManager) EmptyTempFolder(ctx context.Context) (int, error) {
	folderID, err := bm.EnsureTempFolder(ctx)
	if err != nil {
		return 0, err
	}

	token, err := bm.oauthMgr.GetValidAccessToken(ctx)
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf("'%s' in parents and trashed = false", folderID)
	listURL := fmt.Sprintf("%s/files?q=%s&fields=files(id,name)&pageSize=100&supportsAllDrives=true", GoogleDriveAPIEndpoint, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", listURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := bm.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var listRes struct {
		Files []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listRes); err != nil {
		return 0, err
	}

	count := 0
	for _, f := range listRes.Files {
		if err := bm.DeleteFile(ctx, f.ID); err == nil {
			count++
		}
	}

	logger.Infof("Bypass", "Cleaned %d temporary files from Google Drive '%s'", count, TempFolderName)
	return count, nil
}

// DownloadClonedFile streams the cloned Google Drive file to targetFolder with resume support.
func (bm *BypassManager) DownloadClonedFile(
	ctx context.Context,
	clonedID string,
	targetFolder string,
	desiredFilename string,
	onProgress ProgressCallback,
) (string, int64, error) {
	if err := os.MkdirAll(targetFolder, 0755); err != nil {
		return "", 0, fmt.Errorf("failed to create target folder: %w", err)
	}

	token, err := bm.oauthMgr.GetValidAccessToken(ctx)
	if err != nil {
		return "", 0, fmt.Errorf("OAuth token error: %w", err)
	}

	apiURL := fmt.Sprintf("%s/files/%s?alt=media&supportsAllDrives=true&acknowledgeAbuse=true", GoogleDriveAPIEndpoint, clonedID)

	filename := SanitizeFilename(desiredFilename)
	if filename == "" {
		filename = fmt.Sprintf("gdrive_%s.bin", clonedID)
	}
	destPath := UniqueFilePath(filepath.Join(targetFolder, filename))
	filename = filepath.Base(destPath)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	// Check if partial file exists for resume
	var startOffset int64 = 0
	if fi, statErr := os.Stat(destPath); statErr == nil && fi.Size() > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", fi.Size()))
		startOffset = fi.Size()
	}

	resp, err := bm.httpClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*64))
		return "", 0, fmt.Errorf("failed downloading cloned file (HTTP %d): %s", resp.StatusCode, string(body))
	}

	totalSize := resp.ContentLength
	if resp.StatusCode == http.StatusPartialContent {
		totalSize += startOffset
	}

	var out *os.File
	if resp.StatusCode == http.StatusPartialContent && startOffset > 0 {
		out, err = os.OpenFile(destPath, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return "", 0, fmt.Errorf("failed opening file for resume: %w", err)
		}
		logger.Infof("Bypass", "Resuming '%s' from byte %d / %d", filename, startOffset, totalSize)
	} else {
		out, err = os.Create(destPath)
		if err != nil {
			return "", 0, fmt.Errorf("failed creating file: %w", err)
		}
		startOffset = 0
	}
	defer out.Close()

	pr := &progressReader{
		ctx:            ctx,
		reader:         resp.Body,
		totalBytes:     totalSize,
		downloaded:     startOffset,
		lastDownloaded: startOffset,
		lastReport:     time.Now(),
		onProgress:     onProgress,
	}

	buf := make([]byte, 1024*1024)
	copied, err := io.CopyBuffer(out, pr, buf)
	if err != nil {
		out.Close()
		_ = os.Remove(destPath)
		return filename, copied, err
	}
	out.Close()

	if totalSize > 0 {
		if integrityErr := VerifyFileIntegrity(destPath, totalSize); integrityErr != nil {
			logger.Errorf("Integrity", "Integrity failure on '%s': %v", filename, integrityErr)
			return filename, copied, fmt.Errorf("CORRUPT: %w", integrityErr)
		}
	}

	logger.Successf("Bypass", "Successfully downloaded cloned file '%s' (%d bytes)", filename, copied)
	if onProgress != nil {
		onProgress(copied+startOffset, totalSize, 0, 0, 100.0)
	}

	return filename, copied + startOffset, nil
}

// ExecuteBypass executes the full automatic clone -> download -> cleanup cycle.
func (bm *BypassManager) ExecuteBypass(
	ctx context.Context,
	sourceFileID string,
	targetFolder string,
	desiredFilename string,
	onProgress ProgressCallback,
) (string, int64, error) {
	logger.Infof("Bypass", "Starting automated Google Drive quota bypass for file %s...", sourceFileID)

	clonedID, origName, _, err := bm.CloneFileToTemp(ctx, sourceFileID)
	if err != nil {
		return "", 0, fmt.Errorf("quota bypass failed (could not copy to ggdl_temp): %w", err)
	}

	// Ensure cleanup is always attempted, even if context was canceled
	defer func() {
		cleanCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if delErr := bm.DeleteFile(cleanCtx, clonedID); delErr != nil {
			logger.Warnf("Bypass", "Warning: Failed to delete cloned file %s from ggdl_temp: %v", clonedID, delErr)
		} else {
			logger.Infof("Bypass", "Cleaned up temporary cloned file %s from ggdl_temp", clonedID)
		}
	}()

	targetName := desiredFilename
	if targetName == "" {
		targetName = origName
	}

	return bm.DownloadClonedFile(ctx, clonedID, targetFolder, targetName, onProgress)
}
