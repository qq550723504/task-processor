package listingkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskGenerationActionDelegationBoundary(t *testing.T) {
	t.Parallel()

	actionSource := readExecuteTaskGenerationActionSource(t)

	assertSourceOccurrenceCount(t, actionSource, "buildTaskGenerationActionExecutePhase(", 1)
	assertSourceOccurrenceCount(t, actionSource, "buildTaskGenerationActionRefreshPhase(", 1)
	assertSourceOccurrenceCount(t, actionSource, "buildTaskGenerationActionProjectionPhase()", 1)
	assertSourceContainsAll(t, actionSource, []string{
		"taskGenerationActionProjectionInput",
	})
}

func TestTaskGenerationActionServiceBoundary(t *testing.T) {
	t.Parallel()

	actionSource := readExecuteTaskGenerationActionSource(t)

	assertSourceOccurrenceCount(t, actionSource, "buildGenerationReviewSession(", 1)
	assertSourceExcludesAll(t, actionSource, []string{
		"RetryTaskGenerationTasks(ctx, taskID, cloneRetryGenerationTasksRequest(",
		"GetTaskGenerationQueue(ctx, taskID, cloneGenerationQueueQuery(",
		"switch target.InteractionMode {",
		"buildActionPlatformRenderPreviews(",
		"PlatformAssetRenderPreviews = append([]PlatformAssetRenderPreviews(nil),",
		"AssetRenderPreviews = append([]AssetRenderPreview(nil),",
		"buildGenerationReviewWorkflowResult(",
		"applyGenerationReviewWorkflow(",
		"buildGenerationReviewSessionPatch(",
		`"patch_only"`,
		"buildGenerationReviewDeltaToken(",
	})
}

func TestTaskGenerationActionPhaseOwnershipBoundary(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		path      string
		required  []string
		forbidden []string
	}{
		{
			name: "execute_phase",
			path: "task_generation_action_execute.go",
			required: []string{
				"buildTaskGenerationActionExecuteRequestHandoffPhase(p.service).run(",
				"buildGenerationReviewSession(",
				"handoff.persistenceQueue",
			},
			forbidden: []string{
				"RetryTaskGenerationTasks(",
				"GetTaskGenerationQueue(",
				"cloneGenerationQueueQuery(",
				"cloneRetryGenerationTasksRequest(",
				"switch target.InteractionMode {",
				"getCurrentListingKitResult(",
				"buildActionPlatformRenderPreviews(",
				"buildGenerationReviewWorkflowResult(",
				"applyGenerationReviewWorkflow(",
				"buildGenerationReviewSessionPatch(",
				`"patch_only"`,
			},
		},
		{
			name: "refresh_phase",
			path: "task_generation_action_refresh.go",
			required: []string{
				"buildTaskGenerationActionRefreshExtractPhase(",
				"PlatformAssetRenderPreviews",
				"AssetRenderPreviews",
				"baseResult",
			},
			forbidden: []string{
				"getCurrentListingKitResult(",
				"overview := currentResult.AssetGenerationOverview",
				"buildActionPlatformRenderPreviews(currentResult, query)",
				"RetryTaskGenerationTasks(",
				"GetTaskGenerationQueue(",
				"buildGenerationReviewWorkflowResult(",
				"applyGenerationReviewWorkflow(",
				"buildGenerationReviewSessionPatch(",
			},
		},
		{
			name: "projection_phase",
			path: "task_generation_action_projection.go",
			required: []string{
				"buildTaskGenerationActionProjectionSessionPhase().run(",
				"buildTaskGenerationActionProjectionFinalizePhase().run(",
			},
			forbidden: []string{
				"RetryTaskGenerationTasks(",
				"GetTaskGenerationQueue(",
				"getCurrentListingKitResult(",
				"buildActionPlatformRenderPreviews(",
				"buildGenerationReviewSession(",
				"buildGenerationReviewWorkflowResult(",
				"applyGenerationReviewWorkflow(",
				"buildGenerationReviewSessionPatch(",
				`"patch_only"`,
				"buildGenerationReviewDeltaToken(",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			source := readTaskGenerationSourceFile(t, tc.path)
			assertSourceContainsAll(t, source, tc.required)
			assertSourceExcludesAll(t, source, tc.forbidden)
		})
	}
}

func TestReadNamedFunctionSourceHandlesBracesInsideStrings(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "named_function_source.go")
	source := "package listingkit\n\ntype namedFunctionSourceFixture struct{}\n\nfunc (f *namedFunctionSourceFixture) run() string {\n\ttext := \"}\"\n\treturn text\n}\n"
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}

	funcSource := readNamedFunctionSource(t, path, "run")

	assertSourceContainsAll(t, funcSource, []string{
		`text := "}"`,
		"return text",
	})
}

func readExecuteTaskGenerationActionSource(t *testing.T) string {
	t.Helper()
	return readNamedFunctionSource(t, "task_generation_service.go", "ExecuteTaskGenerationAction")
}

func readTaskGenerationSourceFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(content)
}

func readNamedFunctionSource(t *testing.T, path, funcName string) string {
	t.Helper()

	return readFunctionSourceMatching(t, path, "function "+funcName, func(decl *ast.FuncDecl) bool {
		return decl.Name != nil && decl.Name.Name == funcName
	})
}

func readFunctionSourceMatching(t *testing.T, path, description string, match func(*ast.FuncDecl) bool) string {
	t.Helper()

	source := readTaskGenerationSourceFile(t, path)
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, source, parser.ParseComments)
	if err != nil {
		t.Fatalf("ParseFile(%s) error = %v", path, err)
	}

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || !match(funcDecl) {
			continue
		}
		start := fileSet.PositionFor(funcDecl.Pos(), false).Offset
		end := fileSet.PositionFor(funcDecl.End(), false).Offset
		if start < 0 || end < start || end > len(source) {
			t.Fatalf("%s should contain valid source offsets for %s", path, description)
		}
		return source[start:end]
	}

	t.Fatalf("%s should contain %s", path, description)
	return ""
}

func assertSourceContainsAll(t *testing.T, source string, required []string) {
	t.Helper()

	for _, needle := range required {
		if !strings.Contains(source, needle) {
			t.Fatalf("source should contain %q", needle)
		}
	}
}

func assertSourceExcludesAll(t *testing.T, source string, forbidden []string) {
	t.Helper()

	for _, needle := range forbidden {
		if strings.Contains(source, needle) {
			t.Fatalf("source should not contain %q", needle)
		}
	}
}

func assertSourceOccurrenceCount(t *testing.T, source, needle string, want int) {
	t.Helper()

	if got := strings.Count(source, needle); got != want {
		t.Fatalf("source should contain %q %d time(s), got %d", needle, want, got)
	}
}

func readNamedFunctionCallNames(t *testing.T, path, funcName string) []string {
	t.Helper()

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("ParseFile(%s) error = %v", path, err)
	}

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Name == nil || funcDecl.Name.Name != funcName {
			continue
		}
		var names []string
		ast.Inspect(funcDecl.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if name := calledFunctionName(call.Fun); name != "" {
				names = append(names, name)
			}
			return true
		})
		return names
	}

	t.Fatalf("%s should contain function %q", path, funcName)
	return nil
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

func assertFunctionCallsContainAll(t *testing.T, callNames []string, required []string) {
	t.Helper()

	for _, want := range required {
		found := false
		for _, got := range callNames {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("function calls should contain %q; got %v", want, callNames)
		}
	}
}

func assertFunctionCallsExcludeAll(t *testing.T, callNames []string, forbidden []string) {
	t.Helper()

	for _, want := range forbidden {
		for _, got := range callNames {
			if got == want {
				t.Fatalf("function calls should not contain %q; got %v", want, callNames)
			}
		}
	}
}

func assertFunctionCallsAppearInOrder(t *testing.T, callNames []string, expected []string) {
	t.Helper()

	next := 0
	for _, got := range callNames {
		if next >= len(expected) {
			break
		}
		if got == expected[next] {
			next++
		}
	}

	if next != len(expected) {
		t.Fatalf("function calls should contain ordered subsequence %v; got %v", expected, callNames)
	}
}
