package client

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"task-processor/internal/core/logger"
)

func TestMain(m *testing.M) {
	logDir, err := os.MkdirTemp("", "sds-client-test-logs-*")
	if err != nil {
		panic(err)
	}
	logger.InitGlobalLogger(&logger.LogConfig{
		Level:      "error",
		Format:     "json",
		OutputFile: filepath.Join(logDir, "app.log"),
		Console:    false,
	})
	code := m.Run()
	if manager := logger.GetGlobalLogManager(); manager != nil {
		_ = manager.Close()
	}
	_ = os.RemoveAll(logDir)
	os.Exit(code)
}

func TestDefaultConfigKeepsRuntimeStateUnderLocalDirectory(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.AuthFile != ".local/sds/auth_state.json" {
		t.Fatalf("AuthFile = %q, want runtime auth state under .local/sds", cfg.AuthFile)
	}
	if cfg.CookieFile != ".local/sds/session_cookies.json" {
		t.Fatalf("CookieFile = %q, want runtime cookies under .local/sds", cfg.CookieFile)
	}
}

func TestRetryIntervalWithJitterUsesBoundedExponentialBackoff(t *testing.T) {
	t.Parallel()

	base := 1500 * time.Millisecond
	for _, tc := range []struct {
		attempt int
		min     time.Duration
		max     time.Duration
	}{
		{attempt: 1, min: base, max: 2 * base},
		{attempt: 2, min: 2 * base, max: 4 * base},
		{attempt: 3, min: 4 * base, max: 8 * base},
		{attempt: 10, min: sdsRetryMaxInterval, max: sdsRetryMaxInterval},
	} {
		got := retryIntervalWithJitter(base, tc.attempt)
		if tc.min == tc.max {
			if got != tc.min {
				t.Fatalf("attempt %d delay = %s, want %s", tc.attempt, got, tc.min)
			}
			continue
		}
		if got < tc.min || got >= tc.max {
			t.Fatalf("attempt %d delay = %s, want [%s, %s)", tc.attempt, got, tc.min, tc.max)
		}
	}
}
