package listingkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

func TestSDSBaselinePolicyBoundary(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "sds_baseline_readiness_support.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	imported := false
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if path == "task-processor/internal/product/sourcing/sdspod" {
			imported = true
		}
	}
	if !imported {
		t.Fatal("baseline readiness support should import sdspod")
	}

	delegates := false
	for _, decl := range file.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok || function.Name == nil || function.Name.Name != "evaluateSDSBaselineReusableReadiness" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel == nil || selector.Sel.Name != "EvaluateBaseline" {
				return true
			}
			identifier, ok := selector.X.(*ast.Ident)
			if ok && identifier.Name == "sdspod" {
				delegates = true
				return false
			}
			return true
		})
	}
	if !delegates {
		t.Fatal("baseline reusable policy should call sdspod.EvaluateBaseline")
	}
}
