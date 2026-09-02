package runtimepath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallationNamespaceIsStableAcrossRestarts(t *testing.T) {
	first := NamespaceForDirectory(`C:\checkout\one`)
	restarted := NamespaceForDirectory(`C:\checkout\one`)
	otherInstallation := NamespaceForDirectory(`C:\checkout\two`)

	if first != restarted {
		t.Fatalf("namespace changed across restarts: first=%q restarted=%q", first, restarted)
	}
	if first == otherInstallation {
		t.Fatal("different installations must not share a namespace")
	}
	if first == "" || strings.ContainsAny(first, `/\\:`) {
		t.Fatalf("namespace = %q, want a non-empty path-safe value", first)
	}
}

func TestNamespacedTempPathUsesSystemTempDirectory(t *testing.T) {
	got := NamespacedTempPath("productimage")
	root := filepath.Join(os.TempDir(), "task-processor", "productimage")

	if !strings.HasPrefix(got, root+string(filepath.Separator)) {
		t.Fatalf("NamespacedTempPath() = %q, want a namespaced path under %q", got, root)
	}
}
