package main

import (
	"testing"
	"time"

	"task-processor/internal/pkg/timeout"
)

func TestConfigFromEnvDefaultsToAppChrome(t *testing.T) {
	cfg, err := configFromEnv(func(string) string { return "" })
	if err != nil {
		t.Fatalf("configFromEnv() error = %v", err)
	}
	if cfg.Version != "144" {
		t.Fatalf("Version = %q, want 144", cfg.Version)
	}
	if cfg.DownloadDir != "/app/chrome" {
		t.Fatalf("DownloadDir = %q, want /app/chrome", cfg.DownloadDir)
	}
	if cfg.DownloadTimeout != timeout.DownloadLongTimeout {
		t.Fatalf("DownloadTimeout = %v, want %v", cfg.DownloadTimeout, timeout.DownloadLongTimeout)
	}
}

func TestConfigFromEnvUsesOverrides(t *testing.T) {
	values := map[string]string{
		"FINGERPRINT_CHROME_VERSION":          "145",
		"FINGERPRINT_CHROME_DIR":              "/tmp/chrome",
		"FINGERPRINT_CHROME_DOWNLOAD_TIMEOUT": "45m",
	}
	cfg, err := configFromEnv(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("configFromEnv() error = %v", err)
	}
	if cfg.Version != "145" {
		t.Fatalf("Version = %q, want 145", cfg.Version)
	}
	if cfg.DownloadDir != "/tmp/chrome" {
		t.Fatalf("DownloadDir = %q, want /tmp/chrome", cfg.DownloadDir)
	}
	if cfg.DownloadTimeout != 45*time.Minute {
		t.Fatalf("DownloadTimeout = %v, want 45m", cfg.DownloadTimeout)
	}
}

func TestConfigFromEnvAcceptsBareSecondsTimeout(t *testing.T) {
	values := map[string]string{
		"FINGERPRINT_CHROME_DOWNLOAD_TIMEOUT": "60",
	}
	cfg, err := configFromEnv(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("configFromEnv() error = %v", err)
	}
	if cfg.DownloadTimeout != time.Minute {
		t.Fatalf("DownloadTimeout = %v, want 1m", cfg.DownloadTimeout)
	}
}

func TestConfigFromEnvRejectsInvalidTimeout(t *testing.T) {
	values := map[string]string{
		"FINGERPRINT_CHROME_DOWNLOAD_TIMEOUT": "nope",
	}
	if _, err := configFromEnv(func(key string) string { return values[key] }); err == nil {
		t.Fatal("expected invalid timeout error")
	}
}
