package main

import (
	"log"
	"os"

	sharedbrowser "task-processor/internal/crawler/shared/browser"
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

	downloader := sharedbrowser.NewChromeDownloader(version, downloadDir)
	chromePath, err := downloader.CheckAndDownload("")
	if err != nil {
		log.Fatalf("install fingerprint browser failed: %v", err)
	}

	log.Printf("fingerprint browser installed at %s", chromePath)
}
