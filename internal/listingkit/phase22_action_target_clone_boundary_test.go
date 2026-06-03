package listingkit

import "testing"

func TestTaskGenerationActionTargetCloneOwnershipBoundary(t *testing.T) {
	t.Parallel()

	t.Run("local_clone_home_owns_action_target_clone_shape", func(t *testing.T) {
		t.Parallel()

		source := readTaskGenerationSourceFile(t, "task_generation_action_target_clone.go")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_target_clone.go", "cloneAssetGenerationActionTarget")

		assertSourceContainsAll(t, source, []string{
			"func cloneAssetGenerationActionTarget(",
			"func cloneAssetGenerationActionImpact(",
			"cloneAssetGenerationFilters(target.Filters)",
			"cloneGenerationQueueQuery(target.QueueQuery)",
			"cloneRetryGenerationTasksRequest(target.RetryRequest)",
			"cloneAssetGenerationActionImpact(target.ExpectedImpact)",
			"cloneGenerationReviewNavigationTarget(target.NavigationTarget)",
		})
		assertSourceExcludesAll(t, source, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
			"func resolveAssetGenerationActionTarget(",
			"func requestedAssetGenerationActionKey(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneAssetGenerationFilters",
			"cloneGenerationQueueQuery",
			"cloneRetryGenerationTasksRequest",
			"cloneAssetGenerationActionImpact",
			"cloneGenerationReviewNavigationTarget",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"resolveAssetGenerationActionTarget",
			"requestedAssetGenerationActionKey",
		})
	})

	t.Run("service_helper_home_keeps_only_shared_queue_and_retry_clones", func(t *testing.T) {
		t.Parallel()

		source := readTaskGenerationSourceFile(t, "task_generation_shared_clone.go")

		assertSourceContainsAll(t, source, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})
		assertSourceExcludesAll(t, source, []string{
			"func cloneAssetGenerationActionTarget(",
			"func cloneAssetGenerationActionImpact(",
		})
	})
}
