package listingkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

func TestStudioBatchCandidateBoundary(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "task_studio_batch_candidate_support.go", nil, parser.ParseComments)
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
		t.Fatalf("task_studio_batch_candidate_support.go should import %q", importPath)
	}

	var candidateBuilder *ast.FuncDecl
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Name == nil {
			continue
		}
		switch funcDecl.Name.Name {
		case "buildStudioBatchTaskCandidatesForDesign":
			candidateBuilder = funcDecl
		case "normalizeStudioBatchTaskGroupedSelections", "normalizeStudioBatchTaskGroupedSelection", "normalizeStudioBatchTaskDesignType":
			t.Fatalf("task_studio_batch_candidate_support.go should not declare retired %s", funcDecl.Name.Name)
		}
	}
	if candidateBuilder == nil {
		t.Fatal("task_studio_batch_candidate_support.go should retain the root candidate adapter")
	}

	delegates := false
	ast.Inspect(candidateBuilder.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel == nil || selector.Sel.Name != "Evaluate" {
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
		t.Fatal("root candidate adapter should call studiobatch.Evaluate")
	}
}
