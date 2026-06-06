package listingkit

import "testing"

func TestTaskGenerationActionTargetCloneOwnershipBoundary(t *testing.T) {
	t.Parallel()

	t.Run("local_clone_home_owns_action_target_clone_shape", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_target_clone.go")
		source := readNamedFunctionSource(t, "task_generation_action_target_clone.go", "cloneAssetGenerationActionTarget")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_target_clone.go", "cloneAssetGenerationActionTarget")

		assertSourceContainsAll(t, fileSource, []string{
			"func cloneAssetGenerationActionTarget(",
			"func cloneAssetGenerationActionImpact(",
		})
		assertSourceExcludesAll(t, fileSource, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
			"func resolveAssetGenerationActionTarget(",
			"func requestedAssetGenerationActionKey(",
		})
		assertSourceContainsAll(t, source, []string{
			"buildTaskGenerationActionTargetCloneShapePhase().run(target, &cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloneAssetGenerationFilters(target.Filters)",
			"cloneGenerationQueueQuery(target.QueueQuery)",
			"cloneRetryGenerationTasksRequest(target.RetryRequest)",
			"cloneAssetGenerationActionImpact(target.ExpectedImpact)",
			"cloneGenerationReviewNavigationTarget(target.NavigationTarget)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildTaskGenerationActionTargetCloneShapePhase",
			"run",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneAssetGenerationFilters",
			"cloneGenerationQueueQuery",
			"cloneRetryGenerationTasksRequest",
			"cloneAssetGenerationActionImpact",
			"cloneGenerationReviewNavigationTarget",
			"resolveAssetGenerationActionTarget",
			"requestedAssetGenerationActionKey",
		})
	})

	t.Run("service_helper_home_keeps_only_shared_queue_and_retry_clones", func(t *testing.T) {
		t.Parallel()

		sharedCloneSource := readTaskGenerationSourceFile(t, "task_generation_shared_clone.go")

		assertSourceContainsAll(t, sharedCloneSource, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})
		assertSourceExcludesAll(t, sharedCloneSource, []string{
			"func cloneAssetGenerationActionTarget(",
			"func cloneAssetGenerationActionImpact(",
		})
	})
}
