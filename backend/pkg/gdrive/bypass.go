package gdrive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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
	oauthMgr          *OAuthManager
	apiClient         *http.Client
	streamClient      *http.Client
	chunkedDownloader *ChunkedDownloader
	folderMu          sync.RWMutex
	tempFolderID      string
	autoBypass        bool
	chunksPerDownload int
}

// NewBypassManager creates a new BypassManager with the provided OAuthManager.
func NewBypassManager(oauthMgr *OAuthManager, autoBypass bool) *BypassManager {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	streamTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dialer.DialContext(ctx, "tcp4", addr)
			if err != nil {
				return dialer.DialContext(ctx, network, addr)
			}
			return conn, nil
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ReadBufferSize:        1024 * 1024, // 1MB buffer for fast streaming
		WriteBufferSize:       1024 * 1024,
	}

	streamClient := &http.Client{
		Transport: streamTransport,
		Timeout:   0, // NO TIMEOUT for streaming file downloads!
	}

	return &BypassManager{
		oauthMgr:          oauthMgr,
		autoBypass:        autoBypass,
		chunksPerDownload: 4,
		apiClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		streamClient:      streamClient,
		chunkedDownloader: NewChunkedDownloader(streamClient),
	}
}

// SetChunksPerDownload configures the parallel download streams per file.
func (bm *BypassManager) SetChunksPerDownload(n int) {
	if bm == nil {
		return
	}
	bm.folderMu.Lock()
	defer bm.folderMu.Unlock()
	if n <= 0 {
		n = 4
	}
	bm.chunksPerDownload = n
}

// GetChunksPerDownload returns the configured parallel download streams.
func (bm *BypassManager) GetChunksPerDownload() int {
	if bm == nil {
		return 4
	}
	bm.folderMu.RLock()
	defer bm.folderMu.RUnlock()
	if bm.chunksPerDownload <= 0 {
		return 4
	}
	return bm.chunksPerDownload
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
			resp, err := bm.apiClient.Do(req)
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

	resp, err := bm.apiClient.Do(req)
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

	createResp, err := bm.apiClient.Do(createReq)
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

	resp, err := bm.apiClient.Do(req)
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
