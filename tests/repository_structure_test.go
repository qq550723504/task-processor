package tests

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdContainsOnlyOfficialEntrypoints(t *testing.T) {
	allowed := map[string]struct{}{
		"product-listing-api": {},
		"shein-listing":       {},
		"temu-listing":        {},
	}

	out, err := exec.Command("git", "ls-files", "cmd").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(filepath.ToSlash(line), "/")
		if len(parts) < 2 || parts[0] != "cmd" {
			continue
		}
		name := parts[1]
		if _, ok := allowed[name]; !ok {
			t.Errorf("cmd/%s is not an official entrypoint; put one-off debug programs under hack/debug or long-lived developer tools under tools", name)
		}
	}
}

func TestTrackedLocalArtifactsStayOutOfProductionEntrypoints(t *testing.T) {
	out, err := exec.Command("git", "ls-files", "cmd").Output()
	if err != nil {
		t.Fatal(err)
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(filepath.ToSlash(line), "/")
		for _, part := range parts {
			switch part {
			case "logs", "tmp", "bin", "dev-logs", "playwright-cli":
				t.Errorf("%s is a tracked local artifact path under cmd; keep runtime files under .local instead", line)
			}
		}
	}
}
