package listingkit

import "testing"

func TestTaskGenerationActionTargetCloneAggregateBoundary(t *testing.T) {
	t.Parallel()

	t.Run("aggregate_clone_owner_delegates_all_nested_clone_shaping", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_target_clone.go")
		source := readNamedFunctionSource(t, "task_generation_action_target_clone.go", "cloneAssetGenerationActionTarget")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_target_clone.go", "cloneAssetGenerationActionTarget")

		assertSourceContainsAll(t, fileSource, []string{
			"func cloneAssetGenerationActionTarget(",
		})
		assertSourceContainsAll(t, source, []string{
			"if target == nil {",
			"cloned := *target",
			"cloned.Filters = cloneAssetGenerationFilters(target.Filters)",
			"cloned.QueueQuery = cloneGenerationQueueQuery(target.QueueQuery)",
			"cloned.RetryRequest = cloneRetryGenerationTasksRequest(target.RetryRequest)",
			"cloned.ExpectedImpact = cloneAssetGenerationActionImpact(target.ExpectedImpact)",
			"cloned.NavigationTarget = cloneGenerationReviewNavigationTarget(target.NavigationTarget)",
		})
		assertSourceExcludesAll(t, source, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
			"resolveAssetGenerationActionTarget(",
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
		})
	})
}
