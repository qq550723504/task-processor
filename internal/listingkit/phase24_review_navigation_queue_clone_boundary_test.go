package listingkit

import "testing"

func TestGenerationReviewNavigationQueueCloneBoundary(t *testing.T) {
	t.Parallel()

	t.Run("review_navigation_builder_routes_shared_queue_clone_through_common_home", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_review_navigation_target.go", "buildGenerationReviewActionNavigationTarget")
		callNames := readNamedFunctionCallNames(t, "generation_review_navigation_target.go", "buildGenerationReviewActionNavigationTarget")

		assertSourceContainsAll(t, source, []string{
			"cloneAssetGenerationActionTargetForNavigation(target)",
			"cloneGenerationQueueQuery(target.QueueQuery)",
			"applyIdentityToNavigationTarget(navigation)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloneAssetGenerationActionTarget(target)",
			"cloned := *target.QueueQuery",
			"navigation.QueueQuery = &cloned",
			"cloneAssetGenerationFilters(",
			"cloneRetryGenerationTasksRequest(",
			"cloneAssetGenerationActionImpact(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneAssetGenerationActionTargetForNavigation",
			"cloneGenerationQueueQuery",
			"applyIdentityToNavigationTarget",
		})
		assertFunctionCallsAppearInOrder(t, callNames, []string{
			"cloneAssetGenerationActionTargetForNavigation",
			"cloneGenerationQueueQuery",
			"applyIdentityToNavigationTarget",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneAssetGenerationActionTarget",
			"cloneAssetGenerationFilters",
			"cloneRetryGenerationTasksRequest",
			"cloneAssetGenerationActionImpact",
		})
	})

	t.Run("shared_queue_clone_home_stays_outside_review_navigation_local_home", func(t *testing.T) {
		t.Parallel()

		navigationSource := readTaskGenerationSourceFile(t, "generation_review_navigation_target.go")
		sharedCloneSource := readTaskGenerationSourceFile(t, "task_generation_shared_clone.go")

		assertSourceContainsAll(t, navigationSource, []string{
			"func buildGenerationReviewActionNavigationTarget(",
			"func cloneAssetGenerationActionTargetForNavigation(",
			"cloneGenerationQueueQuery(target.QueueQuery)",
		})
		assertSourceExcludesAll(t, navigationSource, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})

		assertSourceContainsAll(t, sharedCloneSource, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})
		assertSourceExcludesAll(t, sharedCloneSource, []string{
			"func buildGenerationReviewActionNavigationTarget(",
			"func cloneAssetGenerationActionTargetForNavigation(",
		})
	})
}
