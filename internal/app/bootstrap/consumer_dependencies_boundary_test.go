package bootstrap

import (
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

func TestConsumerDependenciesDoNotImportLegacyCrawlerAmazon(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "consumer_dependencies.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse consumer dependencies imports: %v", err)
	}
	for _, imported := range file.Imports {
		importPath, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatalf("unquote import %s: %v", imported.Path.Value, err)
		}
		if importPath == "task-processor/internal/crawler/amazon" {
			t.Fatalf("consumer_dependencies.go imports legacy crawler directly: %s", importPath)
		}
	}
}
