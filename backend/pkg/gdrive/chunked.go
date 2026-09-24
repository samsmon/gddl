package gdrive

import (
	"context"
	"net/http"

	"gdrive-downloader/pkg/chunked"
)

// ChunkSegment represents a byte range [Start, End] of a file.
type ChunkSegment = chunked.ChunkSegment

// ChunkedDownloader manages multi-stream segmented downloads with parallel HTTP Range requests.
type ChunkedDownloader struct {
	dl *chunked.Downloader
}

// NewChunkedDownloader creates a new ChunkedDownloader with the provided client.
func NewChunkedDownloader(client *http.Client) *ChunkedDownloader {
	return &ChunkedDownloader{
		dl: chunked.NewDownloader(client),
	}
}

// Transport returns the underlying HTTP transport for dynamic proxy wiring.
func (cd *ChunkedDownloader) Transport() *http.Transport {
	return cd.dl.Transport()
}

// SetEpochProvider forwards the rotation epoch provider to the underlying chunked downloader.
func (cd *ChunkedDownloader) SetEpochProvider(fn func() uint64) {
	cd.dl.SetEpochProvider(fn)
}

// SetRateLimitCallback forwards the rate limit callback to the underlying chunked downloader.
func (cd *ChunkedDownloader) SetRateLimitCallback(fn func(reason string)) {
	cd.dl.SetRateLimitCallback(fn)
}

// DownloadSegmented downloads a file in parallel chunks using HTTP Range requests.
func (cd *ChunkedDownloader) DownloadSegmented(
	ctx context.Context,
	destPath string,
	downloadURL string,
	totalSize int64,
	headers http.Header,
	numChunks int,
	onProgress ProgressCallback,
) (int64, error) {
	var cb chunked.ProgressCallback
	if onProgress != nil {
		cb = chunked.ProgressCallback(onProgress)
	}
	return cd.dl.DownloadSegmented(ctx, destPath, downloadURL, totalSize, headers, numChunks, cb)
}
