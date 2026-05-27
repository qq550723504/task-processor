package main

import (
	"log"
	"os"
	"time"

	sharedbrowser "task-processor/internal/crawler/shared/browser"
	timeoutpkg "task-processor/internal/pkg/timeout"
)

func main() {
	version := os.Getenv("FINGERPRINT_CHROME_VERSION")
	if version == "" {
		version = "144"
	}

	downloadDir := os.Getenv("FINGERPRINT_CHROME_DIR")
	if downloadDir == "" {
		downloadDir = "/opt/fingerprint-chrome"
	}

	downloadTimeout := timeoutpkg.DownloadLongTimeout
	if raw := os.Getenv("FINGERPRINT_CHROME_DOWNLOAD_TIMEOUT"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			log.Fatalf("invalid FINGERPRINT_CHROME_DOWNLOAD_TIMEOUT %q: %v", raw, err)
		}
		downloadTimeout = parsed
	}

	downloader := sharedbrowser.NewChromeDownloaderWithTimeout(version, downloadDir, downloadTimeout)
	chromePath, err := downloader.CheckAndDownload("")
	if err != nil {
		log.Fatalf("install fingerprint browser failed: %v", err)
	}

	log.Printf("fingerprint browser installed at %s", chromePath)
}
