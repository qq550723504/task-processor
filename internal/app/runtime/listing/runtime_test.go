package listing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"task-processor/internal/core/config"
	loggerPkg "task-processor/internal/core/logger"
	"task-processor/internal/pkg/appenv"
)

func TestApplyLoggingConfigFromConfig_WritesToConfiguredFile(t *testing.T) {
	log := appenv.SetupLoggerWithLevel("info")
	logPath := filepath.Join(t.TempDir(), "app.log")
	cfg := &config.Config{
		Logging: config.LoggingConfig{
			Level:  "debug",
			Format: "text",
			File:   logPath,
		},
	}

	if err := applyLoggingConfigFromConfig(log, cfg); err != nil {
		t.Fatalf("apply logging config: %v", err)
	}

	log.Info("runtime logging test")

	if manager := loggerPkg.GetGlobalLogManager(); manager != nil {
		_ = manager.Close()
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(data), "runtime logging test") {
		t.Fatalf("expected log file %s to contain runtime log entry, got %q", logPath, string(data))
	}
}
