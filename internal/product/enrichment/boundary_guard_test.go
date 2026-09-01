package enrichment

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestProductEnrichmentHasOnlyPureDomainDependencies(t *testing.T) {
	t.Parallel()

	allowedModuleImports := map[string]struct{}{
		"task-processor/internal/product/catalog":  {},
		"task-processor/internal/product/sourcing": {},
	}
	for _, file := range enrichmentProductionFiles(t) {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse imports from %s: %v", file, err)
		}
		for _, imported := range parsed.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("unquote import %s: %v", imported.Path.Value, err)
			}
			if !strings.HasPrefix(path, "task-processor/") {
				if isBannedExternalImport(path) {
					t.Errorf("product enrichment file %s imports banned runtime/provider dependency %q", file, path)
				}
				continue
			}
			if _, ok := allowedModuleImports[path]; !ok {
				t.Errorf("product enrichment file %s imports non-domain dependency %q", file, path)
			}
		}
	}
}

func TestProductEnrichmentDeclaresNoRuntimeSemantics(t *testing.T) {
	t.Parallel()

	for _, file := range enrichmentProductionFiles(t) {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatalf("parse declarations from %s: %v", file, err)
		}
		for _, declaration := range parsed.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, specification := range general.Specs {
				typeSpec := specification.(*ast.TypeSpec)
				lower := strings.ToLower(typeSpec.Name.Name)
				for _, forbidden := range []string{"task", "repository", "retry", "queue", "provider"} {
					if strings.Contains(lower, forbidden) {
						t.Errorf("product enrichment file %s declares forbidden runtime type %s", file, typeSpec.Name.Name)
					}
				}
			}
		}
	}
}

func enrichmentProductionFiles(t *testing.T) []string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve product enrichment package directory")
	}
	directory := filepath.Dir(currentFile)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read product enrichment package: %v", err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		files = append(files, filepath.Join(directory, entry.Name()))
	}
	return files
}

func isBannedExternalImport(path string) bool {
	for _, prefix := range []string{
		"gorm.io/",
		"go.temporal.io/",
		"github.com/gin-gonic/",
		"github.com/sirupsen/logrus",
		"github.com/sashabaranov/go-openai",
		"github.com/aws/aws-sdk-go-v2",
		"github.com/redis/",
		"github.com/rabbitmq/",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return strings.Contains(strings.ToLower(path), "/queue")
}
