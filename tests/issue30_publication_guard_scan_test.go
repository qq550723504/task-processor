package tests

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
)

const issue30SourcingPackage = "task-processor/internal/product/sourcing"

func issue30ProductionGoSource(path string) bool {
	path = filepath.ToSlash(path)
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") && !strings.Contains("/"+path, "/testdata/")
}

// Temporary single-symbol gate, not a permanent restriction on Product ownership.
// Only an independently approved publication cutover PR may replace this gate.
// Parse tracked text rather than loading host-buildable packages: no GOOS or tag
// can hide a maintained source file. Parser object resolution distinguishes local
// shadows from package references; imports are resolved by their decoded path.
func issue30PublicationIdentityViolations(sources []listingKitImageBoundarySource) ([]string, error) {
	var violations []string
	for _, source := range sources {
		if !issue30ProductionGoSource(source.path) {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, source.path, source.text, 0)
		if err != nil {
			return nil, err
		}
		owner := filepath.ToSlash(filepath.Dir(source.path)) == "internal/product/sourcing" && file.Name.Name == "sourcing"
		aliases := make(map[string]bool)
		shadowImport := false
		dot := false
		for _, spec := range file.Imports {
			path, err := decodeGoImportPath(spec.Path.Value)
			if err != nil {
				return nil, err
			}
			if spec.Name != nil && spec.Name.Name == "PublicationIdentity" {
				shadowImport = true
			}
			if path != issue30SourcingPackage {
				continue
			}
			alias := "sourcing"
			if spec.Name != nil {
				alias = spec.Name.Name
			}
			if alias == "." {
				dot = true
			} else if alias != "_" {
				aliases[alias] = true
			}
		}
		parents := providerSyntaxParents([]*ast.File{file})
		ast.Inspect(file, func(node ast.Node) bool {
			id, ok := node.(*ast.Ident)
			if !ok || id.Name != "PublicationIdentity" {
				return true
			}
			if sel, ok := parents[id].(*ast.SelectorExpr); ok && sel.Sel == id {
				qualifier, ok := sel.X.(*ast.Ident)
				if ok && qualifier.Obj == nil && aliases[qualifier.Name] {
					violations = append(violations, fmt.Sprintf("%s references %s.PublicationIdentity before cutover approval", fset.Position(id.Pos()), issue30SourcingPackage))
				}
				return true
			}
			if !owner && !dot {
				return true
			}
			if shadowImport {
				return true
			}
			// Names in declarations, fields and keyed struct literals are not uses.
			switch parent := parents[id].(type) {
			case *ast.ImportSpec:
				return true
			case *ast.FuncDecl:
				if parent.Name == id {
					return true
				}
			case *ast.Field:
				for _, name := range parent.Names {
					if name == id {
						return true
					}
				}
			case *ast.KeyValueExpr:
				if parent.Key == id {
					return true
				}
			}
			if id.Obj != nil {
				decl, ok := id.Obj.Decl.(*ast.FuncDecl)
				if !owner || !ok || decl.Recv != nil {
					return true
				}
			}
			// An unresolved unqualified name in the owner refers across files;
			// with a dot import it refers to the imported symbol. Local bindings
			// have an Obj and were excluded above. Invalid source fails CI compile.
			violations = append(violations, fmt.Sprintf("%s references %s.PublicationIdentity before cutover approval", fset.Position(id.Pos()), issue30SourcingPackage))
			return true
		})
	}
	sort.Strings(violations)
	return violations, nil
}
