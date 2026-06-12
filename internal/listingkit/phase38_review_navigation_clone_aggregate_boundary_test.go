package listingkit

import "testing"

func TestGenerationReviewNavigationCloneAggregateBoundary(t *testing.T) {
	t.Parallel()

	t.Run("aggregate_navigation_clone_owner_delegates_nested_clone_shaping", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "generation_review_navigation_target.go")
		source := readNamedFunctionSource(t, "generation_review_navigation_target.go", "cloneGenerationReviewNavigationTarget")
		callNames := readNamedFunctionCallNames(t, "generation_review_navigation_target.go", "cloneGenerationReviewNavigationTarget")

		assertSourceContainsAll(t, fileSource, []string{
			"func cloneGenerationReviewNavigationTarget(",
		})
		assertSourceContainsAll(t, source, []string{
			"if target == nil {",
			"cloned := *target",
			"applyGenerationReviewNavigationTargetCloneShape(target, &cloned)",
			"return applyIdentityToNavigationTarget(&cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.Conditional = cloneGenerationConditionalState(target.Conditional)",
			"cloned.Descriptor = cloneGenerationNavigationDescriptor(target.Descriptor)",
			"cloned.QueueQuery = cloneGenerationQueueQuery(target.QueueQuery)",
			"cloned.SessionQuery = cloneGenerationQueueQuery(target.SessionQuery)",
			"cloned.PreviewQuery = cloneGenerationQueueQuery(target.PreviewQuery)",
			"cloned.ActionTarget = cloneAssetGenerationActionTarget(target.ActionTarget)",
			"buildGenerationReviewActionNavigationTarget(",
			"cloneAssetGenerationActionTargetForNavigation(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationReviewNavigationTargetCloneShape",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationConditionalState",
			"cloneGenerationNavigationDescriptor",
			"cloneGenerationQueueQuery",
			"cloneAssetGenerationActionTarget",
			"buildGenerationReviewActionNavigationTarget",
			"cloneAssetGenerationActionTargetForNavigation",
		})
	})
}
