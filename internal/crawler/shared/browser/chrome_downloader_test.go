package browser

import (
	"testing"
	"time"

	"task-processor/internal/pkg/timeout"
)

func TestNewChromeDownloaderWithTimeoutUsesProvidedTimeout(t *testing.T) {
	downloader := NewChromeDownloaderWithTimeout("144", t.TempDir(), timeout.DownloadLongTimeout)

	if downloader.downloadTimeout != timeout.DownloadLongTimeout {
		t.Fatalf("downloadTimeout = %v, want %v", downloader.downloadTimeout, timeout.DownloadLongTimeout)
	}

	if downloader.httpClient == nil {
		t.Fatal("httpClient should not be nil")
	}
}

func TestNewChromeDownloaderWithTimeoutFallsBackToDefault(t *testing.T) {
	downloader := NewChromeDownloaderWithTimeout("144", t.TempDir(), 0)

	if downloader.downloadTimeout != 10*time.Minute {
		t.Fatalf("downloadTimeout = %v, want %v", downloader.downloadTimeout, 10*time.Minute)
	}
}
