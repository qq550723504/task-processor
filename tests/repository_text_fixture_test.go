package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func normalizeRepositoryText(raw []byte) string {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	return strings.ReplaceAll(text, "\r", "\n")
}

func readRepositoryText(t *testing.T, path ...string) string {
	t.Helper()
	fixturePath := filepath.Join(path...)
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read repository text %s: %v", fixturePath, err)
	}
	return normalizeRepositoryText(raw)
}

func TestNormalizeRepositoryTextHandlesWindowsAndBareCR(t *testing.T) {
	got := normalizeRepositoryText([]byte("alpha\r\nbeta\rgamma\n"))
	want := "alpha\nbeta\ngamma\n"
	if got != want {
		t.Fatalf("normalize repository text = %q, want %q", got, want)
	}
}
