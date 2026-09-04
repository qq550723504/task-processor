package crawler

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestCrawlerIntegrationsDoNotDependOnListingMarketplaceOrRuntime(t *testing.T) {
	t.Parallel()

	forbiddenPrefixes := []string{
		"task-processor/internal/listingkit",
		"task-processor/internal/marketplace",
		"task-processor/internal/publishing",
		"task-processor/internal/workspace",
		"task-processor/internal/app",
		"task-processor/internal/httpbootstrap",
		"task-processor/internal/httproute",
	}
	assertCrawlerIntegrationsDoNotImportPrefixes(t, crawlerIntegrationRootDir(t), forbiddenPrefixes)
}

func TestCrawlerIntegrationBoundaryAllowsSourcingAndRejectsRuntime(t *testing.T) {
	root := t.TempDir()
	allowedPath := filepath.Join(root, "allowed.go")
	if err := os.WriteFile(allowedPath, []byte("package fixture\nimport _ \"task-processor/internal/product/sourcing\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	forbidden := []string{"task-processor/internal/app"}
	violations, err := crawlerIntegrationForbiddenImports(root, forbidden)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("pure sourcing contract was rejected: %v", violations)
	}

	forbiddenPath := filepath.Join(root, "forbidden.go")
	if err := os.WriteFile(forbiddenPath, []byte("package fixture\nimport _ \"task-processor/internal/app\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	violations, err = crawlerIntegrationForbiddenImports(root, forbidden)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 {
		t.Fatalf("runtime violations = %v, want one internal/app violation", violations)
	}
}

func crawlerIntegrationRootDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(filename)
}

func assertCrawlerIntegrationsDoNotImportPrefixes(t *testing.T, root string, forbiddenPrefixes []string) {
	t.Helper()
	violations, err := crawlerIntegrationForbiddenImports(root, forbiddenPrefixes)
	if err != nil {
		t.Fatalf("walk crawler integration imports: %v", err)
	}
	if len(violations) > 0 {
		t.Fatal(strings.Join(violations, "\n"))
	}
}

func crawlerIntegrationForbiddenImports(root string, forbiddenPrefixes []string) ([]string, error) {
	violations := []string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			for _, forbidden := range forbiddenPrefixes {
				if strings.HasPrefix(importPath, forbidden) {
					violations = append(violations, fmt.Sprintf("%s imports forbidden dependency %q", path, importPath))
				}
			}
		}
		return nil
	})
	return violations, err
}
