package studiobatch

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestPackageImportsStayDomainFocused(t *testing.T) {
	allowed := map[string]struct{}{
		"task-processor/internal/listing/studio": {},
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(".", entry.Name())
		file, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
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
			if strings.Contains(importPath, ".") || strings.HasPrefix(importPath, "task-processor/") {
				t.Errorf("%s imports forbidden package %s", path, importPath)
			}
		}
	}
}
