package listingkit

import (
	"os"
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
				"RetryTaskGenerationTasks(",
				"GetTaskGenerationQueue(",
				"buildGenerationReviewSession(",
			},
			forbidden: []string{
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

	source := readTaskGenerationSourceFile(t, path)
	nameIndex := strings.Index(source, funcName+"(")
	if nameIndex == -1 {
		t.Fatalf("%s should contain function %q", path, funcName)
	}
	start := strings.LastIndex(source[:nameIndex], "func ")
	if start == -1 {
		t.Fatalf("%s should declare function %q", path, funcName)
	}
	bodyStart := strings.Index(source[nameIndex:], "{")
	if bodyStart == -1 {
		t.Fatalf("%s should contain body for function %q", path, funcName)
	}
	bodyStart += nameIndex
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
	t.Fatalf("%s should contain a complete body for function %q", path, funcName)
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
