package local

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"task-processor/internal/integration/httpimage"
)

type ImageDownloader struct {
	client *http.Client
}

func NewImageDownloader(timeout time.Duration) *ImageDownloader {
	client := httpimage.NewPublicImageHTTPClient()
	client.Timeout = timeout
	return &ImageDownloader{
		client: client,
	}
}

func (d *ImageDownloader) DownloadImage(url string) ([]byte, error) {
	if d == nil || d.client == nil {
		return nil, fmt.Errorf("image downloader is not configured")
	}
	data, err := httpimage.Download(context.Background(), d.client, url, httpimage.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("download image %s: %w", url, err)
	}
	return data, nil
}
