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
	"time"

	"gdrive-downloader/pkg/chunked"
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
	destPath := filepath.Join(targetFolder, filename)
	if desiredFilename == "" {
		destPath = UniqueFilePath(destPath)
		filename = filepath.Base(destPath)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", defaultUserAgent)

	var startOffset int64
	if fi, statErr := os.Stat(destPath); statErr == nil && fi.Size() > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", fi.Size()))
		startOffset = fi.Size()
	}

	resp, err := bm.streamClient.Do(req)
	if err != nil {
		logger.Errorf("Bypass", "Failed initiating streaming download for '%s' (cloned ID: %s): %v", filename, clonedID, err)
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*64))
		err := fmt.Errorf("failed downloading cloned file (HTTP %d): %s", resp.StatusCode, string(body))
		logger.Errorf("Bypass", "Download failed for '%s': %v", filename, err)
		return "", 0, err
	}

	totalSize := resp.ContentLength
	if resp.StatusCode == http.StatusPartialContent {
		totalSize += startOffset
	}

	chunks := bm.GetChunksPerDownload()
	if chunks > 1 && totalSize > 10*1024*1024 && (startOffset == 0 || (totalSize-startOffset) > 5*1024*1024 || chunked.HasChunkMeta(destPath)) {
		resp.Body.Close()
		logger.Infof("Bypass", "Downloading '%s' (%d bytes) with %d parallel chunk streams", filename, totalSize, chunks)
		headers := http.Header{}
		headers.Set("Authorization", "Bearer "+token)
		headers.Set("User-Agent", defaultUserAgent)

		copied, err := bm.chunkedDownloader.DownloadSegmented(ctx, destPath, apiURL, totalSize, headers, chunks, onProgress)
		if err != nil {
			logger.Errorf("Bypass", "Parallel download failed for '%s': %v", filename, err)
			return filename, copied, err
		}
		if integrityErr := VerifyFileIntegrity(destPath, totalSize); integrityErr != nil {
			logger.Errorf("Integrity", "Integrity check failed for '%s': %v", filename, integrityErr)
			return filename, copied, fmt.Errorf("CORRUPT: %w", integrityErr)
		}
		logger.Successf("Bypass", "Saved '%s' successfully (%d bytes) via %d streams", filename, copied, chunks)
		return filename, copied, nil
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

	pr := &progressReader{ctx: ctx, reader: resp.Body, totalBytes: totalSize, downloaded: startOffset, lastDownloaded: startOffset, lastReport: time.Now(), onProgress: onProgress}
	copied, err := io.CopyBuffer(out, pr, make([]byte, 1024*1024))
	if err != nil {
		out.Close()
		_ = os.Remove(destPath)
		logger.Errorf("Bypass", "Streaming download failed for '%s' (cloned ID: %s): %v", filename, clonedID, err)
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
