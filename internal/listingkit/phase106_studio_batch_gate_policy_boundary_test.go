package listingkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

func TestStudioBatchGatePolicyBoundary(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "studio_batch_task_gate.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	const importPath = "task-processor/internal/listingkit/studiobatch"
	imported := false
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if path == importPath {
			imported = true
		}
	}
	if !imported {
		t.Fatalf("studio_batch_task_gate.go should import %q", importPath)
	}

	var evaluate *ast.FuncDecl
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Name == nil {
			continue
		}
		switch funcDecl.Name.Name {
		case "Evaluate":
			if funcDecl.Recv != nil {
				evaluate = funcDecl
			}
		case "evaluateDesign", "evaluateSelection", "evaluateCompatibility":
			t.Fatalf("studio_batch_task_gate.go should not retain root %s policy", funcDecl.Name.Name)
		}
	}
	if evaluate == nil {
		t.Fatal("studioBatchTaskGate should retain Evaluate")
	}

	delegates := false
	ast.Inspect(evaluate.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel == nil || selector.Sel.Name != "EvaluateGate" {
			return true
		}
		identifier, ok := selector.X.(*ast.Ident)
		if ok && identifier.Name == "studiobatch" {
			delegates = true
			return false
		}
		return true
	})
	if !delegates {
		t.Fatal("root task gate should call studiobatch.EvaluateGate")
	}
}
