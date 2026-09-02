package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestBootstrapRepositoryFamiliesUseTypedValuesAndOnePersistentConstructor(t *testing.T) {
	t.Parallel()

	contracts := readBootstrapRepositoryFileContent(t, "bootstrap_contracts.go")
	assembly := readBootstrapRepositoryFileContent(t, "bootstrap_repositories_merge.go")
	persistent := readBootstrapRepositoryFileContent(t, "builders_db_repository_support.go")

	for _, retired := range []string{"bootstrap_repositories_core.go", "bootstrap_repositories_admin.go", "builders_repositories.go"} {
		if _, err := os.Stat(retired); !os.IsNotExist(err) {
			t.Fatalf("retired repository factory file %s still exists: %v", retired, err)
		}
	}
	assertBootstrapRepositoryContainsAll(t, contracts,
		"type CoreRepositories struct {",
		"type AdminRepositories struct {",
		"type BuildServiceRepositories struct {",
	)
	assertBootstrapRepositoryNotContainsAny(t, contracts, "CoreRepositoryBuilders", "AdminRepositoryBuilders")
	assertBootstrapRepositoryContainsAll(t, assembly, "func buildRepositories(input BuildServiceInput)")
	assertBootstrapRepositoryNotContainsAny(t, assembly, "buildNamedWithClosers", "input.Config.Database")
	assertBootstrapRepositoryContainsAll(t, persistent,
		"func BuildPersistentRepositories(",
		"func NewPersistentRepositories(",
		"openListingKitRepositoryDB",
	)
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
