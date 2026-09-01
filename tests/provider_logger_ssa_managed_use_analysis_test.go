package tests

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"
)

type managedFunctionSpec struct {
	packagePath string
	name        string
}

func managedFunctionValueViolations(loaded []*packages.Package, providerRules []providerLoggerRule, nilRules []typedNilCallRule) []string {
	specifications := make(map[managedFunctionSpec]struct{}, len(providerRules)+len(nilRules))
	for _, rule := range providerRules {
		specifications[managedFunctionSpec{packagePath: rule.PackagePath, name: rule.ConstructorName}] = struct{}{}
	}
	for _, rule := range nilRules {
		specifications[managedFunctionSpec{packagePath: rule.PackagePath, name: rule.FunctionName}] = struct{}{}
	}

	managed := make(map[*types.Func]managedFunctionSpec, len(specifications))
	for _, loadedPackage := range providerPackageGraph(loaded) {
		for specification := range specifications {
			if loadedPackage.PkgPath != specification.packagePath || loadedPackage.Types == nil {
				continue
			}
			object, ok := loadedPackage.Types.Scope().Lookup(specification.name).(*types.Func)
			if !ok {
				continue
			}
			managed[object] = specification
		}
	}

	var violations []string
	for _, loadedPackage := range loaded {
		parents := providerSyntaxParents(loadedPackage.Syntax)
		for identifier, object := range loadedPackage.TypesInfo.Uses {
			functionObject, ok := object.(*types.Func)
			if !ok {
				continue
			}
			specification, isManaged := managed[functionObject]
			if !isManaged || providerFunctionUseIsDirectCall(identifier, parents) {
				continue
			}
			position := loadedPackage.Fset.Position(identifier.Pos())
			violations = append(violations, fmt.Sprintf("%s: %s.%s must be used only as a direct static call", position, specification.packagePath, specification.name))
		}
	}
	return violations
}

func providerPackageGraph(roots []*packages.Package) []*packages.Package {
	seen := make(map[*packages.Package]struct{})
	var packagesInGraph []*packages.Package
	var visit func(*packages.Package)
	visit = func(current *packages.Package) {
		if current == nil {
			return
		}
		if _, exists := seen[current]; exists {
			return
		}
		seen[current] = struct{}{}
		packagesInGraph = append(packagesInGraph, current)
		for _, imported := range current.Imports {
			visit(imported)
		}
	}
	for _, root := range roots {
		visit(root)
	}
	return packagesInGraph
}

func providerSyntaxParents(files []*ast.File) map[ast.Node]ast.Node {
	parents := make(map[ast.Node]ast.Node)
	for _, file := range files {
		var stack []ast.Node
		ast.Inspect(file, func(node ast.Node) bool {
			if node == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if len(stack) > 0 {
				parents[node] = stack[len(stack)-1]
			}
			stack = append(stack, node)
			return true
		})
	}
	return parents
}

func providerFunctionUseIsDirectCall(identifier *ast.Ident, parents map[ast.Node]ast.Node) bool {
	var expression ast.Expr = identifier
	if selector, ok := parents[identifier].(*ast.SelectorExpr); ok && selector.Sel == identifier {
		expression = selector
	}
	for {
		parenthesized, ok := parents[expression].(*ast.ParenExpr)
		if !ok || parenthesized.X != expression {
			break
		}
		expression = parenthesized
	}
	call, ok := parents[expression].(*ast.CallExpr)
	return ok && call.Fun == expression
}
