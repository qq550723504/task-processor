package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/crawler/shared/browser"
	"task-processor/internal/pkg/timeout"
)

const (
	defaultFingerprintChromeVersion = "144"
	defaultFingerprintChromeDir     = "/app/chrome"
)

type installConfig struct {
	Version         string
	DownloadDir     string
	DownloadTimeout time.Duration
}

func configFromEnv(getenv func(string) string) (installConfig, error) {
	cfg := installConfig{
		Version:         strings.TrimSpace(getenv("FINGERPRINT_CHROME_VERSION")),
		DownloadDir:     strings.TrimSpace(getenv("FINGERPRINT_CHROME_DIR")),
		DownloadTimeout: timeout.DownloadLongTimeout,
	}
	if cfg.Version == "" {
		cfg.Version = defaultFingerprintChromeVersion
	}
	if cfg.DownloadDir == "" {
		cfg.DownloadDir = defaultFingerprintChromeDir
	}

	rawTimeout := strings.TrimSpace(getenv("FINGERPRINT_CHROME_DOWNLOAD_TIMEOUT"))
	if rawTimeout != "" {
		parsed, err := parseDuration(rawTimeout)
		if err != nil {
			return installConfig{}, fmt.Errorf("parse FINGERPRINT_CHROME_DOWNLOAD_TIMEOUT: %w", err)
		}
		cfg.DownloadTimeout = parsed
	}

	return cfg, nil
}

func parseDuration(raw string) (time.Duration, error) {
	if seconds, err := strconv.Atoi(raw); err == nil {
		if seconds <= 0 {
			return 0, fmt.Errorf("duration must be positive")
		}
		return time.Duration(seconds) * time.Second, nil
	}
	duration, err := time.ParseDuration(raw)
	if err != nil {
		return 0, err
	}
	if duration <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	return duration, nil
}

func run() error {
	cfg, err := configFromEnv(os.Getenv)
	if err != nil {
		return err
	}

	downloader := browser.NewChromeDownloaderWithTimeout(cfg.Version, cfg.DownloadDir, cfg.DownloadTimeout)
	chromePath, err := downloader.CheckAndDownload("")
	if err != nil {
		return fmt.Errorf("install fingerprint chrome: %w", err)
	}
	fmt.Printf("fingerprint chrome installed at %s\n", chromePath)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
