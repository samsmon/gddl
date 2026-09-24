package gdrive

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gdrive-downloader/pkg/chunked"
	"gdrive-downloader/pkg/logger"
)

type ProgressCallback func(downloadedBytes, totalBytes, speed int64, etaSeconds int64, percentage float64)

type progressReader struct {
	ctx            context.Context
	reader         io.Reader
	totalBytes     int64
	downloaded     int64
	lastDownloaded int64
	lastReport     time.Time
	onProgress     ProgressCallback
}

func (pr *progressReader) Read(p []byte) (int, error) {
	if err := pr.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.downloaded += int64(n)
		now := time.Now()
		if elapsed := now.Sub(pr.lastReport).Seconds(); elapsed >= 0.15 {
			speed := int64(float64(pr.downloaded-pr.lastDownloaded) / elapsed)
			pr.lastDownloaded = pr.downloaded
			pr.lastReport = now
			var percentage float64
			var eta int64
			if pr.totalBytes > 0 {
				percentage = float64(pr.downloaded) / float64(pr.totalBytes) * 100
				if speed > 0 {
					eta = (pr.totalBytes - pr.downloaded) / speed
				}
			}
			if pr.onProgress != nil {
				pr.onProgress(pr.downloaded, pr.totalBytes, speed, eta, percentage)
			}
		}
	}
	return n, err
}

func (d *Downloader) downloadStream(ctx context.Context, finalResp *http.Response, fileID, targetFolder, fallbackHTMLFilename, currentCookie string, onProgress ProgressCallback, desiredFilename ...string) (string, int64, error) {
	defer finalResp.Body.Close()
	var filename, destPath string
	if len(desiredFilename) > 0 && desiredFilename[0] != "" {
		filename = sanitizeFilename(desiredFilename[0])
		destPath = filepath.Join(targetFolder, filename)
	} else {
		filename = extractFilename(finalResp.Header.Get("Content-Disposition"))
		if filename == "" && fallbackHTMLFilename != "" {
			filename = fallbackHTMLFilename
		}
		if filename == "" {
			filename = fmt.Sprintf("gdrive_%s.bin", fileID)
		}
		filename = sanitizeFilename(filename)
		destPath = uniqueFilePath(filepath.Join(targetFolder, filename))
		filename = filepath.Base(destPath)
	}
	totalSize := finalResp.ContentLength
	if totalSize < 0 {
		totalSize = 0
	}
	chunks := d.GetChunksPerDownload()
	downloadURL := ""
	if finalResp.Request != nil && finalResp.Request.URL != nil {
		downloadURL = finalResp.Request.URL.String()
	}
	var existingSize int64
	if fi, statErr := os.Stat(destPath); statErr == nil {
		existingSize = fi.Size()
	}
	if chunks > 1 && downloadURL != "" && totalSize > 10*1024*1024 && (existingSize == 0 || (totalSize-existingSize) > 5*1024*1024 || chunked.HasChunkMeta(destPath)) {
		finalResp.Body.Close()
		logger.Infof("Download", "Downloading '%s' (%d bytes) with %d parallel chunk streams", filename, totalSize, chunks)
		headers := http.Header{}
		headers.Set("User-Agent", defaultUserAgent)
		if currentCookie != "" {
			headers.Set("Cookie", currentCookie)
		}
		copied, err := d.chunkedDownloader.DownloadSegmented(ctx, destPath, downloadURL, totalSize, headers, chunks, onProgress)
		if err != nil {
			if ctx.Err() == nil && !strings.Contains(err.Error(), "context canceled") {
				logger.Errorf("Download", "Parallel download failed for '%s': %v", filename, err)
			}
			return filename, copied, err
		}
		if integrityErr := VerifyFileIntegrity(destPath, totalSize); integrityErr != nil {
			logger.Errorf("Integrity", "Integrity failure on '%s': %v", filename, integrityErr)
			return filename, copied, fmt.Errorf("CORRUPT: %w", integrityErr)
		}
		logger.Successf("Download", "Saved '%s' successfully (%d bytes) via %d streams", filename, copied, chunks)
		return filename, copied, nil
	}

	var out *os.File
	var startOffset int64
	if fi, statErr := os.Stat(destPath); statErr == nil && fi.Size() > 0 && totalSize > 0 && fi.Size() < totalSize {
		if rangeReq, err := http.NewRequestWithContext(ctx, "GET", finalResp.Request.URL.String(), nil); err == nil {
			rangeReq.Header.Set("Range", fmt.Sprintf("bytes=%d-", fi.Size()))
			rangeReq.Header.Set("User-Agent", defaultUserAgent)
			if currentCookie != "" {
				rangeReq.Header.Set("Cookie", currentCookie)
			}
			if rangeResp, err := d.client.Do(rangeReq); err == nil && rangeResp.StatusCode == http.StatusPartialContent {
				finalResp.Body.Close()
				finalResp = rangeResp
				if out, err = os.OpenFile(destPath, os.O_WRONLY|os.O_APPEND, 0644); err == nil {
					startOffset = fi.Size()
					logger.Infof("Download", "Resuming '%s' from byte %d / %d", filename, startOffset, totalSize)
				}
			} else if rangeResp != nil {
				rangeResp.Body.Close()
			}
		}
	}
	if out == nil {
		var err error
		if out, err = os.Create(destPath); err != nil {
			return "", 0, fmt.Errorf("failed to create destination file: %w", err)
		}
	}
	defer out.Close()
	logger.Infof("Download", "Receiving '%s' (Length: %d bytes, Starting: %d bytes)", filename, totalSize, startOffset)
	pr := &progressReader{ctx: ctx, reader: finalResp.Body, totalBytes: totalSize, downloaded: startOffset, lastDownloaded: startOffset, lastReport: time.Now(), onProgress: onProgress}
	copied, err := io.CopyBuffer(out, pr, make([]byte, 1024*1024))
	if err != nil {
		out.Close()
		_ = os.Remove(destPath)
		if ctx.Err() == nil && !strings.Contains(err.Error(), "context canceled") {
			logger.Errorf("Download", "Interrupted/failed downloading '%s': %v", filename, err)
		}
		return filename, copied, err
	}
	out.Close()
	if integrityErr := VerifyFileIntegrity(destPath, totalSize); integrityErr != nil {
		logger.Errorf("Integrity", "Integrity failure on '%s': %v", filename, integrityErr)
		return filename, copied, fmt.Errorf("CORRUPT: %w", integrityErr)
	}
	logger.Successf("Download", "Saved '%s' successfully (%d bytes)", filename, copied)
	if onProgress != nil {
		onProgress(copied, copied, 0, 0, 100.0)
	}
	return filename, copied, nil
}
