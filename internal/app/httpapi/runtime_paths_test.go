package httpapi

import (
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/pkg/runtimepath"
)

func TestResolveImageWorkDirKeepsEmptyConfigRuntimeStateOutsideSourceTree(t *testing.T) {
	want := runtimepath.NamespacedTempPath("productimage")
	for _, cfg := range []*config.Config{nil, &config.Config{}} {
		if got := resolveImageWorkDir(cfg); got != want {
			t.Fatalf("resolveImageWorkDir(%v) = %q, want %q", cfg, got, want)
		}
	}
}
