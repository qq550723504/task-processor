package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyTaskEntrypointIsRemovedFromPrimarySurfaces(t *testing.T) {
	t.Helper()

	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	assertPathMissing(t, filepath.Join(repoRoot, "cmd", "task"))
	assertPathMissing(t, filepath.Join(repoRoot, "deployments", "docker", "Dockerfile.task"))
	assertPathMissing(t, filepath.Join(repoRoot, "deployments", "docker", "docker-compose.task.yml"))

	assertFileDoesNotContain(t, filepath.Join(repoRoot, "Makefile"), "build-task")
	assertFileDoesNotContain(t, filepath.Join(repoRoot, "README.md"), "cmd/task")
	assertFileDoesNotContain(t, filepath.Join(repoRoot, "README.md"), "config-task.yaml")
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected path to be removed: %s", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat path %s: %v", path, err)
	}
}

func assertFileDoesNotContain(t *testing.T, path, needle string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	if strings.Contains(string(content), needle) {
		t.Fatalf("expected %s not to contain %q", path, needle)
	}
}
