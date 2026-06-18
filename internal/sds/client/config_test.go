package client

import (
	"os"
	"path/filepath"
	"testing"

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
