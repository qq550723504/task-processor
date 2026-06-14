package crawler

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestCrawlerIntegrationsDoNotDependOnListingMarketplaceOrProductSourcing(t *testing.T) {
	t.Parallel()

	forbiddenPrefixes := []string{
		"task-processor/internal/listingkit",
		"task-processor/internal/marketplace",
		"task-processor/internal/publishing",
		"task-processor/internal/workspace",
		"task-processor/internal/product/sourcing",
		"task-processor/internal/app",
		"task-processor/internal/httpbootstrap",
		"task-processor/internal/httproute",
	}
	assertCrawlerIntegrationsDoNotImportPrefixes(t, crawlerIntegrationRootDir(t), forbiddenPrefixes)
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
					t.Fatalf("%s imports forbidden dependency %q", path, importPath)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk crawler integration imports: %v", err)
	}
}
