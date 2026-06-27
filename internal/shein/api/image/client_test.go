package image

import (
	"testing"
)

type fakeImageDownloader struct{}

func (fakeImageDownloader) DownloadImage(string) ([]byte, error) {
	return nil, nil
}

func TestNewClientWithImageDownloaderUsesProvidedDownloader(t *testing.T) {
	downloader := fakeImageDownloader{}

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
