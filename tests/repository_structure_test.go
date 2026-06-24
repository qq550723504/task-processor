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
		"listing-control-plane": {},
		"product-listing-api":   {},
		"shein-listing":         {},
		"temu-listing":          {},
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

func TestProductionEntrypointsContainNoLocalArtifacts(t *testing.T) {
	assertNoLocalArtifactPaths(t, filepath.Join("..", "cmd"))
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

func TestHackSupportAreasContainNoLocalArtifacts(t *testing.T) {
	assertNoLocalArtifactPaths(t, filepath.Join("..", "hack"))
}

func TestTrackedLocalArtifactsStayOutOfTools(t *testing.T) {
	assertNoTrackedLocalArtifacts(t, "tools")
}

func TestToolsContainNoLocalArtifacts(t *testing.T) {
	assertNoLocalArtifactPaths(t, filepath.Join("..", "tools"))
}

func TestInternalPackagesContainNoLocalArtifacts(t *testing.T) {
	assertNoLocalArtifactPaths(t, filepath.Join("..", "internal"))
}

func TestSDSLoginRuntimeStateStaysOutOfInternalPackages(t *testing.T) {
	path := filepath.Join("..", "internal", "sdslogin", "data")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s is SDS login runtime state; keep browser/auth state under .local or another ignored runtime directory outside internal packages", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
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

func TestLocalArtifactPathDetectionCoversLocalRuntimeDirectories(t *testing.T) {
	if !containsLocalArtifactPathPart(filepath.Join("internal", "sdslogin", ".local", "auth_state.json")) {
		t.Fatal("expected .local runtime directory to be detected as a local artifact path")
	}
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

	for _, path := range trackedFiles(t, normalizeRepoRelativePath(pathspec)) {
		if !containsLocalArtifactPathPart(path) {
			continue
		}
		t.Errorf("%s is a local artifact path under %s; keep runtime files under .local instead", path, pathspec)
	}
}

func normalizeRepoRelativePath(path string) string {
	normalized := filepath.ToSlash(filepath.Clean(path))
	for strings.HasPrefix(normalized, "../") {
		normalized = strings.TrimPrefix(normalized, "../")
	}
	if normalized == "." {
		return ""
	}
	return normalized
}

func containsLocalArtifactPathPart(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "__debug_bin") {
			return true
		}
		if strings.HasSuffix(strings.ToLower(part), ".exe") {
			return true
		}
		switch part {
		case ".local", "logs", "tmp", "bin", "dev-logs", "playwright-cli", "node_modules", "result":
			return true
		}
	}
	return false
}
