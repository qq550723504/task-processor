package listingkit

import "testing"

func TestTaskGenerationNavigationActionTargetCloneBoundary(t *testing.T) {
	t.Parallel()

	t.Run("navigation_clone_home_owns_only_navigation_specific_shaping", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_review_navigation_target.go", "cloneAssetGenerationActionTargetForNavigation")
		callNames := readNamedFunctionCallNames(t, "generation_review_navigation_target.go", "cloneAssetGenerationActionTargetForNavigation")

		assertSourceContainsAll(t, source, []string{
			"cloneAssetGenerationActionTarget(target)",
			"cloned.NavigationTarget = nil",
			"return cloned",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloneAssetGenerationFilters(",
			"cloneGenerationQueueQuery(",
			"cloneRetryGenerationTasksRequest(",
			"cloneAssetGenerationActionImpact(",
			"cloneGenerationReviewNavigationTarget(",
			"&AssetGenerationActionTarget{",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneAssetGenerationActionTarget",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneAssetGenerationFilters",
			"cloneGenerationQueueQuery",
			"cloneRetryGenerationTasksRequest",
			"cloneAssetGenerationActionImpact",
			"cloneGenerationReviewNavigationTarget",
		})
	})

	t.Run("navigation_builder_routes_through_navigation_clone_home", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_review_navigation_target.go", "buildGenerationReviewActionNavigationTarget")
		callNames := readNamedFunctionCallNames(t, "generation_review_navigation_target.go", "buildGenerationReviewActionNavigationTarget")

		assertSourceContainsAll(t, source, []string{
			"cloneAssetGenerationActionTargetForNavigation(target)",
			"DispatchKind: \"action\"",
			"target.QueueQuery != nil",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloneAssetGenerationActionTarget(target)",
			"cloneAssetGenerationFilters(",
			"cloneRetryGenerationTasksRequest(",
			"cloneAssetGenerationActionImpact(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneAssetGenerationActionTargetForNavigation",
			"applyIdentityToNavigationTarget",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneAssetGenerationActionTarget",
			"cloneAssetGenerationFilters",
			"cloneRetryGenerationTasksRequest",
			"cloneAssetGenerationActionImpact",
		})
	})

	t.Run("common_clone_home_keeps_shared_action_target_clone_semantics", func(t *testing.T) {
		t.Parallel()

		source := readTaskGenerationSourceFile(t, "task_generation_action_target_clone.go")

		assertSourceContainsAll(t, source, []string{
			"func cloneAssetGenerationActionTarget(",
			"buildTaskGenerationActionTargetCloneShapePhase().run(target, &cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"func cloneAssetGenerationActionTargetForNavigation(",
			"NavigationTarget = nil",
			"func buildGenerationReviewActionNavigationTarget(",
		})
	})
}
