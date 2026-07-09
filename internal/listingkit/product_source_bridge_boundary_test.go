package listingkit

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestSourceFactsBridgeImportsOnlyNeutralFacts(t *testing.T) {
	t.Parallel()

	allowedImports := map[string]struct{}{
		"sort":                          {},
		"strings":                       {},
		"task-processor/internal/asset":  {},
		"task-processor/internal/catalog": {},
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(filename), "product_source_bridge.go")
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
		if _, ok := allowedImports[importPath]; !ok {
			t.Fatalf("product_source_bridge.go imports %q; bridge should only depend on stdlib plus neutral catalog/asset facts", importPath)
		}
	}
}
