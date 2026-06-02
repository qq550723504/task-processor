package listingkit

import "testing"

func TestTaskGenerationActionServiceEntryBoundary(t *testing.T) {
	t.Parallel()

	source := readExecuteTaskGenerationActionSource(t)
	callNames := readNamedFunctionCallNames(t, "task_generation_service.go", "ExecuteTaskGenerationAction")

	assertSourceOccurrenceCount(t, source, "executeLayerTemporalAction(ctx, taskID, req)", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationActionEntryPhase(s).run(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationActionExecutePhase(s).run(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationActionPersistPhase(s).run(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationActionRefreshPhase(s).run(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationActionProjectionPhase().run(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationActionFinalizePhase().run(", 1)
	assertFunctionCallsContainAll(t, callNames, []string{
		"executeLayerTemporalAction",
		"buildTaskGenerationActionEntryPhase",
		"buildTaskGenerationActionExecutePhase",
		"buildTaskGenerationActionPersistPhase",
		"buildTaskGenerationActionRefreshPhase",
		"buildTaskGenerationActionProjectionPhase",
		"buildTaskGenerationActionFinalizePhase",
	})
	assertFunctionCallsAppearInOrder(t, callNames, []string{
		"executeLayerTemporalAction",
		"buildTaskGenerationActionEntryPhase",
		"buildTaskGenerationActionExecutePhase",
		"buildTaskGenerationActionPersistPhase",
		"buildTaskGenerationActionRefreshPhase",
		"buildTaskGenerationActionProjectionPhase",
		"buildTaskGenerationActionFinalizePhase",
	})
	assertFunctionCallsExcludeAll(t, callNames, []string{
		"getCurrentAssetGenerationQueue",
		"getCurrentListingKitResult",
		"buildAssetGenerationOverview",
		"resolveAssetGenerationActionTarget",
		"buildGenerationReviewSession",
		"RetryTaskGenerationTasks",
		"GetTaskGenerationQueue",
		"persistGenerationReviewDecision",
		"buildActionPlatformRenderPreviews",
		"buildGenerationReviewWorkflowResult",
		"applyGenerationReviewWorkflow",
		"buildGenerationReviewSessionPatch",
		"buildGenerationReviewDeltaToken",
		"applyGenerationConditionalStateToActionResult",
	})
}

func TestTaskGenerationActionPhaseOwnershipServiceEntryBoundary(t *testing.T) {
	t.Parallel()

	t.Run("service_orchestrates_entry_seams", func(t *testing.T) {
		t.Parallel()

		source := readExecuteTaskGenerationActionSource(t)
		callNames := readNamedFunctionCallNames(t, "task_generation_service.go", "ExecuteTaskGenerationAction")

		assertSourceContainsAll(t, source, []string{
			"executeLayerTemporalAction(ctx, taskID, req)",
			"buildTaskGenerationActionEntryPhase(s).run(",
			"buildTaskGenerationActionExecutePhase(s).run(",
			"buildTaskGenerationActionPersistPhase(s).run(",
			"buildTaskGenerationActionRefreshPhase(s).run(",
			"buildTaskGenerationActionProjectionPhase().run(",
			"buildTaskGenerationActionFinalizePhase().run(",
		})
		assertFunctionCallsAppearInOrder(t, callNames, []string{
			"executeLayerTemporalAction",
			"buildTaskGenerationActionEntryPhase",
			"buildTaskGenerationActionExecutePhase",
			"buildTaskGenerationActionPersistPhase",
			"buildTaskGenerationActionRefreshPhase",
			"buildTaskGenerationActionProjectionPhase",
			"buildTaskGenerationActionFinalizePhase",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"getCurrentAssetGenerationQueue",
			"getCurrentListingKitResult",
			"buildAssetGenerationOverview",
			"resolveAssetGenerationActionTarget",
			"buildGenerationReviewSession",
			"RetryTaskGenerationTasks",
			"GetTaskGenerationQueue",
			"persistGenerationReviewDecision",
			"buildActionPlatformRenderPreviews",
			"buildGenerationReviewWorkflowResult",
			"applyGenerationReviewWorkflow",
			"buildGenerationReviewSessionPatch",
			"buildGenerationReviewDeltaToken",
			"applyGenerationConditionalStateToActionResult",
		})
	})

	testCases := []struct {
		name      string
		path      string
		required  []string
		forbidden []string
	}{
		{
			name: "entry_phase_owns_bootstrap_and_resolution",
			path: "task_generation_action_entry.go",
			required: []string{
				"getCurrentAssetGenerationQueue(",
				"getCurrentListingKitResult(",
				"buildTaskGenerationActionTargetResolutionPhase().run(queue, req)",
				"buildAssetGenerationActionImpact(",
				"buildGenerationReviewSession(",
				"GenerationActionExecutionResult{",
			},
			forbidden: []string{
				"executeLayerTemporalAction(",
				"RetryTaskGenerationTasks(",
				"GetTaskGenerationQueue(",
				"persistGenerationReviewDecision(",
				"buildTaskGenerationActionRefreshPhase(",
				"buildTaskGenerationActionProjectionPhase(",
				"buildTaskGenerationActionFinalizePhase(",
				"buildGenerationReviewWorkflowResult(",
				"applyGenerationReviewWorkflow(",
				"buildGenerationReviewSessionPatch(",
				"buildGenerationReviewDeltaToken(",
				"applyGenerationConditionalStateToActionResult(",
			},
		},
		{
			name: "execute_phase_owns_local_action_execution",
			path: "task_generation_action_execute.go",
			required: []string{
				"RetryTaskGenerationTasks(",
				"GetTaskGenerationQueue(",
				"buildGenerationReviewSession(",
				"generationWorkQueueFromRetryPage(",
				"generationWorkQueueFromPage(",
			},
			forbidden: []string{
				"executeLayerTemporalAction(",
				"getCurrentAssetGenerationQueue(",
				"getCurrentListingKitResult(",
				"buildAssetGenerationOverview(",
				"resolveAssetGenerationActionTarget(",
				"persistGenerationReviewDecision(",
				"buildActionPlatformRenderPreviews(",
				"buildGenerationReviewWorkflowResult(",
				"applyGenerationReviewWorkflow(",
				"buildGenerationReviewSessionPatch(",
				"buildGenerationReviewDeltaToken(",
				"applyGenerationConditionalStateToActionResult(",
			},
		},
		{
			name: "persist_phase_owns_persisted_review_handoff",
			path: "task_generation_action_persist.go",
			required: []string{
				"isPersistedGenerationReviewAction(target.ActionKey)",
				"execution.persistenceSession",
				"persistGenerationReviewDecision(ctx, taskID, target.ActionKey, execution.persistenceSession, target)",
			},
			forbidden: []string{
				"executeLayerTemporalAction(",
				"getCurrentAssetGenerationQueue(",
				"getCurrentListingKitResult(",
				"buildAssetGenerationOverview(",
				"resolveAssetGenerationActionTarget(",
				"buildGenerationReviewSession(",
				"RetryTaskGenerationTasks(",
				"GetTaskGenerationQueue(",
				"buildActionPlatformRenderPreviews(",
				"buildGenerationReviewWorkflowResult(",
				"applyGenerationReviewWorkflow(",
				"buildGenerationReviewSessionPatch(",
				"buildGenerationReviewDeltaToken(",
				"applyGenerationConditionalStateToActionResult(",
			},
		},
		{
			name: "finalize_phase_owns_projection_copy_back",
			path: "task_generation_action_finalize.go",
			required: []string{
				"result.Overview = projection.Overview",
				"result.Queue = projection.Queue",
				"result.Retry = projection.Retry",
				"result.ReviewPatch = projection.ReviewPatch",
				"result.PlatformRenderPreviews = projection.PlatformRenderPreviews",
				"applyGenerationConditionalStateToActionResult(result)",
			},
			forbidden: []string{
				"executeLayerTemporalAction(",
				"getCurrentAssetGenerationQueue(",
				"getCurrentListingKitResult(",
				"buildAssetGenerationOverview(",
				"resolveAssetGenerationActionTarget(",
				"buildGenerationReviewSession(",
				"RetryTaskGenerationTasks(",
				"GetTaskGenerationQueue(",
				"persistGenerationReviewDecision(",
				"buildActionPlatformRenderPreviews(",
				"buildGenerationReviewWorkflowResult(",
				"applyGenerationReviewWorkflow(",
				"buildGenerationReviewSessionPatch(",
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
