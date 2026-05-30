package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestTaskGenerationActionDelegationBoundary(t *testing.T) {
	t.Parallel()

	actionSource := readExecuteTaskGenerationActionSource(t)

	assertSourceContainsAll(t, actionSource, []string{
		"buildTaskGenerationActionExecutePhase(s).run(",
		"buildTaskGenerationActionRefreshPhase(s).run(",
		"buildTaskGenerationActionProjectionPhase().run(&taskGenerationActionProjectionInput{",
	})
}

func TestTaskGenerationActionServiceBoundary(t *testing.T) {
	t.Parallel()

	actionSource := readExecuteTaskGenerationActionSource(t)

	assertSourceExcludesAll(t, actionSource, []string{
		"RetryTaskGenerationTasks(ctx, taskID, cloneRetryGenerationTasksRequest(",
		"GetTaskGenerationQueue(ctx, taskID, cloneGenerationQueueQuery(",
		"switch target.InteractionMode {",
		"buildActionPlatformRenderPreviews(",
		"PlatformAssetRenderPreviews = append([]PlatformAssetRenderPreviews(nil),",
		"AssetRenderPreviews = append([]AssetRenderPreview(nil),",
		"result.ReviewSession = buildGenerationReviewSession(",
		"result.ReviewWorkflow = buildGenerationReviewWorkflowResult(",
		"applyGenerationReviewWorkflow(",
		"result.ReviewPatch = buildGenerationReviewSessionPatch(",
		`if result.ResponseMode == "patch_only" {`,
		"result.DeltaToken = buildGenerationReviewDeltaToken(",
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
				"getCurrentListingKitResult(",
				"buildActionPlatformRenderPreviews(",
				"PlatformAssetRenderPreviews = append([]PlatformAssetRenderPreviews(nil),",
				"AssetRenderPreviews = append([]AssetRenderPreview(nil),",
			},
			forbidden: []string{
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
				"buildGenerationReviewSession(",
				"buildGenerationReviewWorkflowResult(",
				"applyGenerationReviewWorkflow(",
				"buildGenerationReviewSessionPatch(",
				`if result.ResponseMode == "patch_only" {`,
			},
			forbidden: []string{
				"RetryTaskGenerationTasks(",
				"GetTaskGenerationQueue(",
				"getCurrentListingKitResult(",
				"buildActionPlatformRenderPreviews(",
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
	return readSourceSection(
		t,
		"task_generation_service.go",
		"func (s *taskGenerationService) ExecuteTaskGenerationAction(",
		"func (s *taskGenerationService) DispatchTaskGenerationNavigation(",
	)
}

func readTaskGenerationSourceFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(content)
}

func readSourceSection(t *testing.T, path, startMarker, endMarker string) string {
	t.Helper()

	source := readTaskGenerationSourceFile(t, path)
	start := strings.Index(source, startMarker)
	end := strings.Index(source, endMarker)
	if start == -1 || end == -1 || end <= start {
		t.Fatalf("%s should contain source section between %q and %q", path, startMarker, endMarker)
	}
	return source[start:end]
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
