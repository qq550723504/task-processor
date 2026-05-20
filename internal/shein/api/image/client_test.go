package image

import (
	"testing"
	"time"

	"task-processor/internal/infra/clients/management"
)

func TestNewClientWithImageDownloaderUsesProvidedDownloader(t *testing.T) {
	downloader := management.NewImageDownloader(3*time.Second, true)

	client := NewClientWithImageDownloader(nil, downloader)

	if client.imageDownloader != downloader {
		t.Fatal("expected client to reuse provided image downloader")
	}
}

func TestNewClientCreatesDefaultDownloaderWhenMissing(t *testing.T) {
	client := NewClientWithImageDownloader(nil, nil)

	if client.imageDownloader == nil {
		t.Fatal("expected default image downloader to be created")
	}
}
