package listingkit

import "testing"

func TestTaskGenerationSharedCloneHelperBoundary(t *testing.T) {
	t.Parallel()

	t.Run("shared_clone_homes_keep_queue_and_retry_clone_helpers_separate", func(t *testing.T) {
		t.Parallel()

		queueCloneSource := readTaskGenerationSourceFile(t, "generation_queue_query_clone.go")
		retryCloneSource := readTaskGenerationSourceFile(t, "task_generation_shared_clone.go")

		assertSourceContainsAll(t, queueCloneSource, []string{
			"func cloneGenerationQueueQuery(",
		})
		assertSourceExcludesAll(t, queueCloneSource, []string{
			"func cloneRetryGenerationTasksRequest(",
			"func ExecuteTaskGenerationAction(",
			"func resolveLayerTemporalPlatform(",
			"func cloneAssetGenerationActionTarget(",
		})

		assertSourceContainsAll(t, retryCloneSource, []string{
			"func cloneRetryGenerationTasksRequest(",
		})
		assertSourceExcludesAll(t, retryCloneSource, []string{
			"func cloneGenerationQueueQuery(",
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

		actionTargetCloneShapeSource := readTaskGenerationSourceFile(t, "task_generation_action_target_clone_shape.go")
		reviewNavigationSource := readTaskGenerationSourceFile(t, "generation_review_navigation_target.go")
		retryRequestSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_retry_request.go")
		queueRequestSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_queue_request.go")

		assertSourceContainsAll(t, actionTargetCloneShapeSource, []string{
			"cloneGenerationQueueQuery(target.QueueQuery)",
			"cloneRetryGenerationTasksRequest(target.RetryRequest)",
		})
		assertSourceExcludesAll(t, actionTargetCloneShapeSource, []string{
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
