package listingkit

import "testing"

func TestTaskGenerationSharedCloneHelperBoundary(t *testing.T) {
	t.Parallel()

	t.Run("shared_clone_home_keeps_only_queue_and_retry_clone_helpers", func(t *testing.T) {
		t.Parallel()

		source := readTaskGenerationSourceFile(t, "task_generation_shared_clone.go")

		assertSourceContainsAll(t, source, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})
		assertSourceExcludesAll(t, source, []string{
			"func ExecuteTaskGenerationAction(",
			"func resolveLayerTemporalPlatform(",
			"func cloneAssetGenerationActionTarget(",
			"func cloneAssetGenerationActionImpact(",
			"func buildTaskGenerationActionExecuteRequestHandoffRetryPhase(",
			"func buildTaskGenerationActionExecuteRequestHandoffQueuePhase(",
		})
	})

	t.Run("direct_consumers_keep_using_shared_clone_home", func(t *testing.T) {
		t.Parallel()

		actionTargetCloneSource := readTaskGenerationSourceFile(t, "task_generation_action_target_clone.go")
		reviewNavigationSource := readTaskGenerationSourceFile(t, "generation_review_navigation_target.go")
		retryRequestSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_retry_request.go")
		queueRequestSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_queue_request.go")

		assertSourceContainsAll(t, actionTargetCloneSource, []string{
			"cloneGenerationQueueQuery(target.QueueQuery)",
			"cloneRetryGenerationTasksRequest(target.RetryRequest)",
		})
		assertSourceExcludesAll(t, actionTargetCloneSource, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})

		assertSourceContainsAll(t, reviewNavigationSource, []string{
			"cloneGenerationQueueQuery(target.QueueQuery)",
		})
		assertSourceExcludesAll(t, reviewNavigationSource, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})

		assertSourceContainsAll(t, retryRequestSource, []string{
			"return cloneRetryGenerationTasksRequest(target.RetryRequest)",
		})
		assertSourceExcludesAll(t, retryRequestSource, []string{
			"func cloneRetryGenerationTasksRequest(",
			"RetryTaskGenerationTasks(",
		})

		assertSourceContainsAll(t, queueRequestSource, []string{
			"return cloneGenerationQueueQuery(target.QueueQuery)",
		})
		assertSourceExcludesAll(t, queueRequestSource, []string{
			"func cloneGenerationQueueQuery(",
			"GetTaskGenerationQueue(",
		})
	})
}
