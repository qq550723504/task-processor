package httpapi

import (
	"os"
	"path/filepath"
	"testing"

	"task-processor/internal/core/config"
)

func TestResolveImageWorkDirKeepsEmptyConfigRuntimeStateOutsideSourceTree(t *testing.T) {
	want := filepath.Join(os.TempDir(), "task-processor", "productimage")
	for _, cfg := range []*config.Config{nil, &config.Config{}} {
		if got := resolveImageWorkDir(cfg); got != want {
			t.Fatalf("resolveImageWorkDir(%v) = %q, want %q", cfg, got, want)
		}
	}
}
