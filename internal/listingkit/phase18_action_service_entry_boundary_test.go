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

	t.Run("entry_phase_owns_bootstrap_and_resolution", func(t *testing.T) {
		t.Parallel()

		source := readTaskGenerationSourceFile(t, "task_generation_action_entry.go")
		assertSourceContainsAll(t, source, []string{
			"getCurrentAssetGenerationQueue(",
			"getCurrentListingKitResult(",
			"buildTaskGenerationActionTargetResolutionPhase().run(queue, req)",
			"buildAssetGenerationActionImpact(",
			"buildGenerationReviewSession(",
			"GenerationActionExecutionResult{",
		})
		assertSourceExcludesAll(t, source, []string{
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
		})
	})

	t.Run("execute_phase_owns_local_action_execution", func(t *testing.T) {
		t.Parallel()

		source := readExactMethodSource(t, "task_generation_action_execute.go", "func (p *taskGenerationActionExecutePhase) run(")
		assertSourceContainsAll(t, source, []string{
			"buildTaskGenerationActionExecuteRequestHandoffPhase(p.service).run(",
			"buildGenerationReviewSession(",
			"handoff.persistenceQueue",
		})
		assertSourceExcludesAll(t, source, []string{
			"executeLayerTemporalAction(",
			"getCurrentAssetGenerationQueue(",
			"getCurrentListingKitResult(",
			"buildAssetGenerationOverview(",
			"resolveAssetGenerationActionTarget(",
			"RetryTaskGenerationTasks(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"cloneRetryGenerationTasksRequest(",
			"switch target.InteractionMode {",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
			"persistGenerationReviewDecision(",
			"buildActionPlatformRenderPreviews(",
			"buildGenerationReviewWorkflowResult(",
			"applyGenerationReviewWorkflow(",
			"buildGenerationReviewSessionPatch(",
			"buildGenerationReviewDeltaToken(",
			"applyGenerationConditionalStateToActionResult(",
		})
	})

	t.Run("persist_phase_owns_persisted_review_handoff", func(t *testing.T) {
		t.Parallel()

		source := readExactMethodSource(t, "task_generation_action_execute.go", "func (p *taskGenerationActionPersistPhase) run(")
		assertSourceContainsAll(t, source, []string{
			"isPersistedGenerationReviewAction(target.ActionKey)",
			"execution.persistenceSession",
			"persistGenerationReviewDecision(ctx, taskID, target.ActionKey, execution.persistenceSession, target)",
		})
		assertSourceExcludesAll(t, source, []string{
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
		})
	})

	t.Run("finalize_phase_owns_projection_copy_back", func(t *testing.T) {
		t.Parallel()

		source := readExactMethodSource(t, "task_generation_action_execute.go", "func (p *taskGenerationActionFinalizePhase) run(")
		assertSourceContainsAll(t, source, []string{
			"applyGenerationActionProjectionToResult(result, projection)",
			"applyGenerationConditionalStateToActionResult(result)",
		})
		assertSourceExcludesAll(t, source, []string{
			"result.Overview = projection.Overview",
			"result.Queue = projection.Queue",
			"result.Retry = projection.Retry",
			"result.ReviewPatch = projection.ReviewPatch",
			"result.PlatformRenderPreviews = projection.PlatformRenderPreviews",
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
		})
	})
}
