package httpapi

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListingKitProductionOwnsItsImageUploadConfiguration(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	var violations []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Clean(entry.Name())
		fileset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fileset, path, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}

		configAliases := map[string]struct{}{}
		for _, spec := range file.Imports {
			if strings.Trim(spec.Path.Value, `"`) != "task-processor/internal/core/config" {
				continue
			}
			alias := "config"
			if spec.Name != nil {
				alias = spec.Name.Name
			}
			configAliases[alias] = struct{}{}
		}

		configVariables := map[string]struct{}{}
		ast.Inspect(file, func(node ast.Node) bool {
			field, ok := node.(*ast.Field)
			if !ok || len(field.Names) == 0 || !isConfigPointer(field.Type, configAliases) {
				return true
			}
			for _, name := range field.Names {
				configVariables[name.Name] = struct{}{}
			}
			return true
		})

		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			receiver, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			_, configVariable := configVariables[receiver.Name]
			_, configAlias := configAliases[receiver.Name]
			forbiddenField := selector.Sel.Name == "ProductImage" || selector.Sel.Name == "ImageAgent"
			forbiddenType := strings.HasPrefix(selector.Sel.Name, "ProductImage") || strings.HasPrefix(selector.Sel.Name, "ImageAgent")
			if (configVariable && forbiddenField) || (configAlias && forbiddenType) {
				violations = append(violations, fmt.Sprintf("%s: ListingKit cannot access %s.%s", fileset.Position(selector.Pos()), receiver.Name, selector.Sel.Name))
			}
			return true
		})
	}

	if len(violations) > 0 {
		t.Fatalf("ListingKit image uploads must use Config.ListingKit only:\n%s", strings.Join(violations, "\n"))
	}
}

func isConfigPointer(expr ast.Expr, aliases map[string]struct{}) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := star.X.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Config" {
		return false
	}
	receiver, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	_, ok = aliases[receiver.Name]
	return ok
}
