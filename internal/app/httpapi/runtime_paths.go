package httpapi

import (
	"path/filepath"

	"task-processor/internal/core/config"
	"task-processor/internal/pkg/runtimepath"
)

func resolveImageWorkDir(cfg *config.Config) string {
	if cfg == nil {
		return runtimepath.NamespacedTempPath("productimage")
	}

	workDir := filepath.Clean(cfg.ProductImage.WorkDir)
	if workDir == "" || workDir == "." {
		return runtimepath.NamespacedTempPath("productimage")
	}

	return workDir
}
