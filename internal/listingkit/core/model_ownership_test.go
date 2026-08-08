package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestRootPackageDoesNotDeclareCoreTaskLifecycleSymbols(t *testing.T) {
	t.Parallel()

	forbidden := map[string]struct{}{
		"ErrTaskNotFound": {}, "ErrTaskNotPending": {},
		"ErrTaskNotRecoverable": {}, "ErrTaskRecoveryUnavailable": {},
		"ErrTaskRequeueUnavailable": {}, "ErrTaskRequeueInvalidRequest": {},
		"ErrGenerationTaskNotFound": {}, "ErrGenerationTaskNotRetryable": {},
		"ErrGenerationActionNotFound": {}, "ErrChildTaskRetryInvalidRequest": {},
		"ErrChildTaskNotFound": {}, "ErrChildTaskNotRetryable": {},
		"ErrChildTaskRetryConflict": {}, "TaskStatus": {},
		"TaskStatusPending": {}, "TaskStatusProcessing": {},
		"TaskStatusCompleted": {}, "TaskStatusNeedsReview": {},
		"TaskStatusFailed": {}, "TaskStatusBlockedRetryable": {},
	}

	paths, err := filepath.Glob(filepath.Join("..", "*.go"))
	if err != nil {
		t.Fatalf("glob root listingkit files: %v", err)
	}
	fset := token.NewFileSet()
	var duplicates []string
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				switch typed := spec.(type) {
				case *ast.TypeSpec:
					if _, found := forbidden[typed.Name.Name]; found {
						duplicates = append(duplicates, filepath.Base(path)+":"+typed.Name.Name)
					}
				case *ast.ValueSpec:
					for _, name := range typed.Names {
						if _, found := forbidden[name.Name]; found {
							duplicates = append(duplicates, filepath.Base(path)+":"+name.Name)
						}
					}
				}
			}
		}
	}
	if len(duplicates) > 0 {
		sort.Strings(duplicates)
		t.Fatalf("root listingkit package redeclares core task lifecycle symbols: %s", strings.Join(duplicates, ", "))
	}
}
