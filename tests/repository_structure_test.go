package tests

import (
	"os"
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

	for _, line := range trackedFiles(t, "cmd") {
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
	assertNoTrackedLocalArtifacts(t, "cmd")
}

func TestHackContainsOnlyManagedSupportAreas(t *testing.T) {
	allowed := map[string]struct{}{
		"debug": {},
		"k8s":   {},
	}

	for _, line := range trackedFiles(t, "hack") {
		parts := strings.Split(filepath.ToSlash(line), "/")
		if len(parts) < 2 || parts[0] != "hack" {
			continue
		}
		name := parts[1]
		if _, ok := allowed[name]; !ok {
			t.Errorf("hack/%s is not a managed support area; put debug experiments under hack/debug or promote long-lived tools to tools", name)
		}
	}
}

func TestTrackedLocalArtifactsStayOutOfTools(t *testing.T) {
	assertNoTrackedLocalArtifacts(t, "tools")
}

func TestPlatformRegistrationPackagesStayThin(t *testing.T) {
	allowedFiles := map[string]struct{}{
		"internal/platforms/modules.go":           {},
		"internal/platforms/amazon/module.go":     {},
		"internal/platforms/shein/doc.go":         {},
		"internal/platforms/shein/module.go":      {},
		"internal/platforms/shein/module_test.go": {},
		"internal/platforms/temu/module.go":       {},
	}

	for _, line := range trackedFiles(t, "internal/platforms") {
		path := filepath.ToSlash(line)
		if _, ok := allowedFiles[path]; !ok {
			t.Errorf("%s is a new platform registration file; keep internal/platforms limited to module descriptors and put platform business rules in marketplace or publishing packages", path)
		}
	}
}

func TestPlatformRegistrationPackagesContainNoLocalArtifacts(t *testing.T) {
	assertNoLocalArtifactPaths(t, filepath.Join("..", "internal", "platforms"))
}

func trackedFiles(t *testing.T, pathspec string) []string {
	t.Helper()

	out, err := exec.Command("git", "ls-files", pathspec).Output()
	if err != nil {
		t.Fatal(err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func assertNoTrackedLocalArtifacts(t *testing.T, pathspec string) {
	t.Helper()

	for _, line := range trackedFiles(t, pathspec) {
		if containsLocalArtifactPathPart(line) {
			t.Errorf("%s is a tracked local artifact path under %s; keep runtime files under .local instead", line, pathspec)
		}
	}
}

func assertNoLocalArtifactPaths(t *testing.T, pathspec string) {
	t.Helper()

	err := filepath.Walk(pathspec, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == pathspec {
			return nil
		}
		if containsLocalArtifactPathPart(path) {
			t.Errorf("%s is a local artifact path under %s; keep runtime files under .local instead", path, pathspec)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func containsLocalArtifactPathPart(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		switch part {
		case "logs", "tmp", "bin", "dev-logs", "playwright-cli":
			return true
		}
	}
	return false
}
