package main

import (
	"strings"
	"testing"
)

func TestSeedCommandRequiresLocalFilesAndPublicSourceURL(t *testing.T) {
	err := run([]string{
		"-runtime-file", "", "-token-file", "", "-source-url", "http://localhost/a.png",
	})
	if err == nil || !strings.Contains(err.Error(), "-runtime-file") {
		t.Fatalf("run() error = %v, want required local-file flags", err)
	}
}
