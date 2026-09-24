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
	"strconv"
	"strings"

	"gdrive-downloader/pkg/logger"
)

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
	bodyBytes, _ := json.Marshal(map[string]interface{}{"parents": []string{folderID}})

	req, err := http.NewRequestWithContext(ctx, "POST", copyURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := bm.apiClient.Do(req)
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
	for _, prefix := range []string{"Copy of ", "Salinan dari ", "Copia de "} {
		if strings.HasPrefix(cleanName, prefix) {
			cleanName = strings.TrimPrefix(cleanName, prefix)
			break
		}
	}

	logger.Successf("Bypass", "Cloned file %s into %s as '%s' (Cloned ID: %s, Size: %d bytes)", sourceFileID, TempFolderName, cleanName, res.ID, parsedSize)
	return res.ID, cleanName, parsedSize, nil
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

	resp, err := bm.apiClient.Do(req)
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
