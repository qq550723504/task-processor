package listingkit

import (
	"go/ast"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskGenerationActionProjectionPhaseBoundary(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "task_generation_action_projection.go", "run")

	assertSourceOccurrenceCount(t, source, "buildTaskGenerationActionProjectionSessionPhase().run(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationActionProjectionFinalizePhase().run(", 1)
	assertSourceExcludesAll(t, source, []string{
		"buildGenerationReviewSession(",
		"buildGenerationReviewWorkflowResult(",
		"applyGenerationReviewWorkflow(",
		"buildGenerationReviewSessionPatch(",
		`"patch_only"`,
		"buildGenerationReviewDeltaToken(",
	})
	assertSourceContainsInOrder(
		t,
		source,
		"buildTaskGenerationActionProjectionSessionPhase().run(",
		"buildTaskGenerationActionProjectionFinalizePhase().run(",
	)
}

func TestTaskGenerationActionProjectionSessionBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_action_projection.go", "func (p *taskGenerationActionProjectionSessionPhase) run(")

	assertSourceOccurrenceCount(t, source, "taskGenerationActionProjectionReviewQueue(", 1)
	assertSourceOccurrenceCount(t, source, "buildGenerationReviewSession(", 1)
	assertSourceOccurrenceCount(t, source, "projectionQueueQuery(", 1)
	assertSourceExcludesAll(t, source, []string{
		"buildGenerationReviewWorkflowResult(",
		"applyGenerationReviewWorkflow(",
		"buildGenerationReviewSessionPatch(",
		`"patch_only"`,
		"buildGenerationReviewDeltaToken(",
	})
}

func TestTaskGenerationActionProjectionSessionQueueBoundary(t *testing.T) {
	t.Parallel()

	source := readDeclaredFunctionSource(t, "task_generation_action_projection.go", "taskGenerationActionProjectionReviewQueue(")

	assertSourceOccurrenceCount(t, source, "generationWorkQueueFromRetryPage(", 1)
	assertSourceOccurrenceCount(t, source, "generationWorkQueueFromPage(", 1)
	assertSourceExcludesAll(t, source, []string{
		"buildGenerationReviewSession(",
		"buildGenerationReviewWorkflowResult(",
		"applyGenerationReviewWorkflow(",
		"buildGenerationReviewSessionPatch(",
		`"patch_only"`,
		"buildGenerationReviewDeltaToken(",
	})
}

func TestTaskGenerationActionProjectionFinalizeBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_action_projection.go", "func (p *taskGenerationActionProjectionFinalizePhase) run(")

	assertSourceOccurrenceCount(t, source, "buildGenerationReviewWorkflowResult(", 1)
	assertSourceOccurrenceCount(t, source, "applyGenerationReviewWorkflow(", 1)
	assertSourceOccurrenceCount(t, source, "buildGenerationReviewSessionPatch(", 1)
	assertSourceOccurrenceCount(t, source, "buildGenerationReviewDeltaToken(", 1)
	assertSourceOccurrenceCount(t, source, `"patch_only"`, 1)
	assertSourceExcludesAll(t, source, []string{
		"taskGenerationActionProjectionReviewQueue(",
		"generationWorkQueueFromRetryPage(",
		"generationWorkQueueFromPage(",
		"buildGenerationReviewSession(",
		"projectionQueueQuery(",
	})
}

func TestTaskGenerationActionProjectionPhaseOwnershipBoundary(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		path      string
		required  []string
		forbidden []string
	}{
		{
			name: "projection_phase_file",
			path: "task_generation_action_projection.go",
			required: []string{
				"buildTaskGenerationActionProjectionSessionPhase().run(",
				"buildTaskGenerationActionProjectionFinalizePhase().run(",
			},
			forbidden: []string{},
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

	t.Run("session_phase_method", func(t *testing.T) {
		t.Parallel()

		source := readExactMethodSource(t, "task_generation_action_projection.go", "func (p *taskGenerationActionProjectionSessionPhase) run(")
		assertSourceContainsAll(t, source, []string{
			"taskGenerationActionProjectionReviewQueue(",
			"buildGenerationReviewSession(",
			"projectionQueueQuery(",
		})
		assertSourceExcludesAll(t, source, []string{
			"buildGenerationReviewWorkflowResult(",
			"applyGenerationReviewWorkflow(",
			"buildGenerationReviewSessionPatch(",
			`"patch_only"`,
			"buildGenerationReviewDeltaToken(",
		})
	})

	t.Run("finalize_phase_method", func(t *testing.T) {
		t.Parallel()

		source := readExactMethodSource(t, "task_generation_action_projection.go", "func (p *taskGenerationActionProjectionFinalizePhase) run(")
		assertSourceContainsAll(t, source, []string{
			"reviewSession *GenerationReviewSession",
			"buildGenerationReviewWorkflowResult(",
			"applyGenerationReviewWorkflow(",
			"buildGenerationReviewSessionPatch(",
			"buildGenerationReviewDeltaToken(",
		})
		assertSourceExcludesAll(t, source, []string{
			"taskGenerationActionProjectionSessionResult",
			"taskGenerationActionProjectionReviewQueue(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
			"buildGenerationReviewSession(",
			"projectionQueueQuery(",
		})
	})
}

func TestReadDeclaredFunctionSourceHandlesBracesInsideStrings(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "declared_function_source.go")
	source := "package listingkit\n\nfunc trickyDeclaredFunction() string {\n\ttext := \"}\"\n\treturn text\n}\n"
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}

	funcSource := readDeclaredFunctionSource(t, path, "trickyDeclaredFunction(")

	assertSourceContainsAll(t, funcSource, []string{
		`text := "}"`,
		"return text",
	})
}

func assertSourceContainsInOrder(t *testing.T, source, first, second string) {
	t.Helper()

	firstIndex := strings.Index(source, first)
	if firstIndex == -1 {
		t.Fatalf("source should contain %q", first)
	}
	secondIndex := strings.Index(source, second)
	if secondIndex == -1 {
		t.Fatalf("source should contain %q", second)
	}
	if firstIndex >= secondIndex {
		t.Fatalf("source should contain %q before %q", first, second)
	}
}

func readDeclaredFunctionSource(t *testing.T, path, decl string) string {
	t.Helper()

	funcName, _, _ := strings.Cut(decl, "(")
	funcName = strings.TrimSpace(funcName)
	return readFunctionSourceMatching(t, path, "function declaration "+decl, func(decl *ast.FuncDecl) bool {
		return decl.Name != nil && decl.Name.Name == funcName
	})
}
