package browser

import (
	"path/filepath"
	"testing"
)

func TestResolveUserDataDirReturnsAbsolutePath(t *testing.T) {
	got, err := resolveUserDataDir(filepath.Join(".", "tmp", "browser-profiles", "1688"))
	if err != nil {
		t.Fatalf("resolveUserDataDir returned error: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("expected absolute path, got %q", got)
	}
}

func TestResolveUserDataDirKeepsAbsolutePath(t *testing.T) {
	want := filepath.Clean(filepath.Join(t.TempDir(), "profile"))
	got, err := resolveUserDataDir(want)
	if err != nil {
		t.Fatalf("resolveUserDataDir returned error: %v", err)
	}
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveUserDataDirAllowsEmpty(t *testing.T) {
	got, err := resolveUserDataDir(" ")
	if err != nil {
		t.Fatalf("resolveUserDataDir returned error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty path, got %q", got)
	}
}
