package httpapi

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTopUpSecretRejectsUnsafeFiles(t *testing.T) {
	t.Run("relative path", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if err := os.WriteFile("credential", []byte("fixture-secret"), 0600); err != nil {
			t.Fatal(err)
		}
		if value, err := readTopUpSecret(context.Background(), "credential"); err == nil || value != "" {
			t.Fatal("relative credential path was accepted")
		}
	})
	t.Run("symbolic link", func(t *testing.T) {
		dir := t.TempDir()
		target, link := filepath.Join(dir, "credential"), filepath.Join(dir, "link")
		if err := os.WriteFile(target, []byte("fixture-secret"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			if runtime.GOOS == "windows" {
				t.Skip("Windows symlink creation is unavailable")
			}
			t.Fatal(err)
		}
		if value, err := readTopUpSecret(context.Background(), link); err == nil || value != "" {
			t.Fatal("symlink credential was accepted")
		}
	})
	for _, mode := range []os.FileMode{0640, 0644} {
		t.Run(mode.String(), func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("Unix mode bits are not Windows ACLs")
			}
			path := filepath.Join(t.TempDir(), "credential")
			if err := os.WriteFile(path, []byte("fixture-secret"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}
			if value, err := readTopUpSecret(context.Background(), path); err == nil || value != "" {
				t.Fatal("shared credential file was accepted")
			}
		})
	}
}

func TestTopUpPrivateCredentialAndPublicKeyFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix private-file acceptance requires Unix permissions")
	}
	path := filepath.Join(t.TempDir(), "credential")
	if err := os.WriteFile(path, []byte("fixture-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if value, err := readTopUpSecret(context.Background(), path); err != nil || value != "fixture-secret" {
		t.Fatal("private credential was not loaded")
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if value, err := readTopUpKeyFile(path); err != nil || value != "fixture-secret" {
		t.Fatal("public verification key incorrectly required confidential permissions")
	}
}

func TestTopUpKeyFileReadIsBounded(t *testing.T) {
	for _, size := range []int{0, 16384, 16385} {
		path := filepath.Join(t.TempDir(), "key")
		if err := os.WriteFile(path, []byte(strings.Repeat("x", size)), 0600); err != nil {
			t.Fatal(err)
		}
		value, err := readTopUpKeyFile(path)
		if size == 16384 {
			if err != nil || len(value) != size {
				t.Fatal("valid bounded key was rejected")
			}
		} else if err == nil || value != "" {
			t.Fatal("empty or oversized key was accepted")
		}
	}
}
