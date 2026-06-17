package sourcing

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

func TestProductSourcingDoesNotDependOnListingKitMarketplaceOrRuntime(t *testing.T) {
	t.Parallel()

	forbiddenPrefixes := []string{
		"task-processor/internal/listingkit",
		"task-processor/internal/marketplace",
		"task-processor/internal/publishing/shein",
		"task-processor/internal/workspace/shein",
		"task-processor/internal/app",
		"task-processor/internal/httpbootstrap",
		"task-processor/internal/httproute",
		"task-processor/internal/infra",
		"task-processor/internal/platform",
	}
	assertProductSourcingDoesNotImportPrefixes(t, productSourcingPackageDir(t), forbiddenPrefixes)
}

func TestProductSourcingDoesNotDependOnLegacyCrawlerRuntime(t *testing.T) {
	t.Parallel()

	forbiddenPrefixes := []string{
		"task-processor/internal/crawler/amazon",
		"task-processor/internal/crawler/alibaba1688",
	}
	allowedImports := map[string]struct{}{
		"task-processor/internal/crawler/alibaba1688/model": {},
	}
	assertProductSourcingDoesNotImportPrefixesExcept(t, productSourcingPackageDir(t), forbiddenPrefixes, allowedImports)
}

func productSourcingPackageDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(filename)
}

func assertProductSourcingDoesNotImportPrefixes(t *testing.T, dir string, forbiddenPrefixes []string) {
	t.Helper()
	assertProductSourcingDoesNotImportPrefixesExcept(t, dir, forbiddenPrefixes, nil)
}

func assertProductSourcingDoesNotImportPrefixesExcept(t *testing.T, dir string, forbiddenPrefixes []string, allowedImports map[string]struct{}) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse imports for %s: %v", path, err)
		}
		for _, imported := range file.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("unquote import %s: %v", imported.Path.Value, err)
			}
			if _, allowed := allowedImports[importPath]; allowed {
				continue
			}
			for _, forbidden := range forbiddenPrefixes {
				if strings.HasPrefix(importPath, forbidden) {
					t.Fatalf("%s imports forbidden boundary dependency %q", path, importPath)
				}
			}
		}
	}
}
