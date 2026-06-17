package resources

import (
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

func TestSharedResourcesDoNotImportLegacyCrawlerAmazon(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "shared_resources.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse shared resources imports: %v", err)
	}
	for _, imported := range file.Imports {
		importPath, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatalf("unquote import %s: %v", imported.Path.Value, err)
		}
		if importPath == "task-processor/internal/crawler/amazon" {
			t.Fatalf("shared_resources.go imports legacy crawler directly: %s", importPath)
		}
	}
}
