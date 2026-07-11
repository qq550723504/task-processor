package sdspod

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestPackageImportsStayPlatformNeutral(t *testing.T) {
	allowed := map[string]struct{}{
		"task-processor/internal/catalog/canonical": {},
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(".", entry.Name())
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := allowed[importPath]; ok {
				continue
			}
			if strings.Contains(importPath, ".") ||
				strings.HasPrefix(importPath, "task-processor/") {
				t.Errorf("%s imports forbidden package %s", path, importPath)
			}
		}
	}
}
