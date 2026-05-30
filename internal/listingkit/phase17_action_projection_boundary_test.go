package listingkit

import (
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

	source := readNamedFunctionSource(t, "task_generation_action_projection_session.go", "run")

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

	source := readDeclaredFunctionSource(t, "task_generation_action_projection_session.go", "taskGenerationActionProjectionReviewQueue(")

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

	source := readNamedFunctionSource(t, "task_generation_action_projection_finalize.go", "run")

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

	source := readTaskGenerationSourceFile(t, path)
	start := strings.Index(source, "func "+decl)
	if start == -1 {
		t.Fatalf("%s should contain function declaration %q", path, decl)
	}
	bodyStart := strings.Index(source[start:], "{")
	if bodyStart == -1 {
		t.Fatalf("%s should contain body for function declaration %q", path, decl)
	}
	bodyStart += start
	depth := 0
	for index := bodyStart; index < len(source); index++ {
		switch source[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[start : index+1]
			}
		}
	}
	t.Fatalf("%s should contain a complete body for function declaration %q", path, decl)
	return ""
}
