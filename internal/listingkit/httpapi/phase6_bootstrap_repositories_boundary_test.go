package httpapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapRepositoryFamiliesOwnSeparatedResponsibilities(t *testing.T) {
	t.Parallel()

	dir := "."

	contractsFile := readBootstrapRepositoryFileContent(t, filepath.Join(dir, "bootstrap_repositories_contracts.go"))
	coreFile := readBootstrapRepositoryFileContent(t, filepath.Join(dir, "bootstrap_repositories_core.go"))
	adminFile := readBootstrapRepositoryFileContent(t, filepath.Join(dir, "bootstrap_repositories_admin.go"))
	mergeFile := readBootstrapRepositoryFileContent(t, filepath.Join(dir, "bootstrap_repositories_merge.go"))

	assertBootstrapRepositoryNoFile(t, filepath.Join(dir, "bootstrap_repositories.go"))

	assertBootstrapRepositoryContainsAll(t, contractsFile,
		"type builtRepositories struct {",
		"type builtCoreRepositories struct {",
		"type builtAdminRepositories struct {",
		"type repositoryAssembly struct {",
	)
	assertBootstrapRepositoryNotContainsAny(t, contractsFile,
		"func buildCoreRepositories(",
		"func buildAdminRepositories(",
		"func assembleRepositories(",
	)

	assertBootstrapRepositoryContainsAll(t, coreFile,
		"func buildCoreRepositories(",
		"func buildLateCoreRepositories(",
		"func buildSubscriptionService(",
	)
	assertBootstrapRepositoryNotContainsAny(t, coreFile,
		"type builtRepositories struct {",
		"func buildAdminRepositories(",
		"func assembleRepositories(",
	)

	assertBootstrapRepositoryContainsAll(t, adminFile,
		"func buildAdminRepositories(",
		"func buildAdminCatalogRepositories(",
		"func buildAdminRuleRepositories(",
	)
	assertBootstrapRepositoryNotContainsAny(t, adminFile,
		"type builtRepositories struct {",
		"func buildCoreRepositories(",
		"func assembleRepositories(",
	)

	assertBootstrapRepositoryContainsAll(t, mergeFile,
		"func applyCoreRepositories(",
		"func applyAdminRepositories(",
		"func mergeBuiltRepositories(",
		"func assembleRepositories(",
		"func buildRepositories(",
	)
	assertBootstrapRepositoryNotContainsAny(t, mergeFile,
		"type builtRepositories struct {",
		"func buildAdminCatalogRepositories(",
		"func buildSubscriptionService(",
	)
}

func assertBootstrapRepositoryNoFile(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %s to be removed", path)
	}
}

func readBootstrapRepositoryFileContent(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertBootstrapRepositoryContainsAll(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected content to contain %q", snippet)
		}
	}
}

func assertBootstrapRepositoryNotContainsAny(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected content to exclude %q", snippet)
		}
	}
}
