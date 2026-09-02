package httpapi

import (
	"os"
	"path/filepath"

	"task-processor/internal/core/config"
)

func resolveImageWorkDir(cfg *config.Config) string {
	if cfg == nil {
		return filepath.Join(os.TempDir(), "task-processor", "productimage")
	}

	workDir := filepath.Clean(cfg.ProductImage.WorkDir)
	if workDir == "" || workDir == "." {
		return filepath.Join(os.TempDir(), "task-processor", "productimage")
	}

	return workDir
}
