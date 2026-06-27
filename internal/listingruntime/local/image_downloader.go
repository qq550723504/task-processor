package local

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ImageDownloader struct {
	client *http.Client
}

func NewImageDownloader(timeout time.Duration, insecureSkipVerify bool) *ImageDownloader {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if insecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return &ImageDownloader{
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
}

func (d *ImageDownloader) DownloadImage(url string) ([]byte, error) {
	if d == nil || d.client == nil {
		return nil, fmt.Errorf("image downloader is not configured")
	}
	resp, err := d.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download image %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
