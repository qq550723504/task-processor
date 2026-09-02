package httpapi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionRepositoryCompositionHasNoMemoryOrGenericFallbackCalls(t *testing.T) {
	t.Parallel()

	repoRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	directories := []string{
		filepath.Join(repoRoot, "internal", "amazonlisting", "httpapi"),
		filepath.Join(repoRoot, "internal", "listingkit", "httpapi"),
		filepath.Join(repoRoot, "internal", "app", "httpapi"),
	}
	var violations []string
	for _, directory := range directories {
		packages, err := parser.ParseDir(token.NewFileSet(), directory, func(info os.FileInfo) bool {
			return !strings.HasSuffix(info.Name(), "_test.go")
		}, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", directory, err)
		}
		for _, pkg := range packages {
			for filename, file := range pkg.Files {
				ast.Inspect(file, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					name := calledFunctionName(call.Fun)
					if name == "buildRepositoryWithFallback" || strings.HasPrefix(name, "NewMem") {
						violations = append(violations, filepath.Base(filename)+":"+name)
					}
					return true
				})
			}
		}
	}
	if len(violations) > 0 {
		t.Fatalf("production repository composition calls forbidden fallbacks: %v", violations)
	}
}

func calledFunctionName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return typed.Sel.Name
	default:
		return ""
	}
}
