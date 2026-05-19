package sheinlogin

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveProfileDirReturnsAbsolutePath(t *testing.T) {
	dir, err := resolveProfileDir("./.local/tmp/shein-login/profiles", 1, 2)
	if err != nil {
		t.Fatalf("resolveProfileDir: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("expected absolute path, got %s", dir)
	}
	if filepath.Base(dir) != "2" || filepath.Base(filepath.Dir(dir)) != "1" {
		t.Fatalf("unexpected profile dir structure: %s", dir)
	}
}

func TestClearProfileLockFiles(t *testing.T) {
	root := t.TempDir()
	for _, name := range profileLockFiles {
		if err := os.WriteFile(filepath.Join(root, name), []byte("lock"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if !clearProfileLockFiles(root) {
		t.Fatal("expected lock files to be cleared")
	}
	for _, name := range profileLockFiles {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, err=%v", name, err)
		}
	}
}

func TestTrimProfileDirRemovesCacheDirectories(t *testing.T) {
	root := t.TempDir()
	targets := []string{
		filepath.Join(root, "Default", "Cache"),
		filepath.Join(root, "Crashpad"),
	}
	for _, target := range targets {
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", target, err)
		}
	}
	if removed := trimProfileDir(root); removed < len(targets) {
		t.Fatalf("expected at least %d directories removed, got %d", len(targets), removed)
	}
	for _, target := range targets {
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, err=%v", target, err)
		}
	}
}

func TestIsProfileInUseError(t *testing.T) {
	cases := []string{
		"profile appears to be in use",
		"ProcessSingleton already held",
		"profile directory is locked",
		"SingletonLock exists",
	}
	for _, message := range cases {
		if !isProfileInUseError(errors.New(message)) {
			t.Fatalf("expected profile-in-use for %q", message)
		}
	}
	if isProfileInUseError(errors.New("other error")) {
		t.Fatal("did not expect unrelated error to match profile-in-use")
	}
}
