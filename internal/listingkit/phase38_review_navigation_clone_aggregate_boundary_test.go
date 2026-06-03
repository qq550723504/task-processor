package listingkit

import "testing"

func TestGenerationReviewNavigationCloneAggregateBoundary(t *testing.T) {
	t.Parallel()

	t.Run("aggregate_navigation_clone_shape_owner_delegates_nested_clone_shaping", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "generation_review_navigation_target_clone_shape.go")
		buildSource := readNamedFunctionSource(t, "generation_review_navigation_target_clone_shape.go", "buildGenerationReviewNavigationTargetCloneShapePhase")
		source := readNamedFunctionSource(t, "generation_review_navigation_target_clone_shape.go", "run")
		callNames := readNamedFunctionCallNames(t, "generation_review_navigation_target_clone_shape.go", "run")

		assertSourceContainsAll(t, fileSource, []string{
			"type generationReviewNavigationTargetCloneShapePhase struct{}",
			"func buildGenerationReviewNavigationTargetCloneShapePhase()",
			"func (p *generationReviewNavigationTargetCloneShapePhase) run(",
		})
		assertSourceContainsAll(t, buildSource, []string{
			"return &generationReviewNavigationTargetCloneShapePhase{}",
		})
		assertSourceContainsAll(t, source, []string{
			"if target == nil || cloned == nil {",
			"cloned.Conditional = cloneGenerationConditionalState(target.Conditional)",
			"cloned.Descriptor = cloneGenerationNavigationDescriptor(target.Descriptor)",
			"cloned.QueueQuery = cloneGenerationQueueQuery(target.QueueQuery)",
			"cloned.SessionQuery = cloneGenerationQueueQuery(target.SessionQuery)",
			"cloned.PreviewQuery = cloneGenerationQueueQuery(target.PreviewQuery)",
			"cloned.ActionTarget = cloneAssetGenerationActionTarget(target.ActionTarget)",
		})
		assertSourceExcludesAll(t, source, []string{
			"func cloneGenerationReviewNavigationTarget(",
			"buildGenerationReviewActionNavigationTarget(",
			"cloneAssetGenerationActionTargetForNavigation(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneGenerationConditionalState",
			"cloneGenerationNavigationDescriptor",
			"cloneGenerationQueueQuery",
			"cloneAssetGenerationActionTarget",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"buildGenerationReviewActionNavigationTarget",
			"cloneAssetGenerationActionTargetForNavigation",
		})
	})
}
