package listingkit

import "testing"

func TestGenerationNavigationDescriptorCloneAggregateBoundary(t *testing.T) {
	t.Parallel()

	t.Run("aggregate_descriptor_clone_owner_delegates_nested_clone_shaping", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "generation_navigation_target_identity.go")
		source := readNamedFunctionSource(t, "generation_navigation_target_identity.go", "cloneGenerationNavigationDescriptor")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_target_identity.go", "cloneGenerationNavigationDescriptor")

		assertSourceContainsAll(t, fileSource, []string{
			"func cloneGenerationNavigationDescriptor(",
			"func applyGenerationNavigationDescriptorCloneShapePairing(",
		})
		assertSourceContainsAll(t, source, []string{
			"if descriptor == nil {",
			"cloned := *descriptor",
			"applyGenerationNavigationDescriptorCloneShapePairing(descriptor, &cloned)",
			"return &cloned",
		})
		assertSourceExcludesAll(t, source, []string{
			"buildGenerationReviewNavigationTarget(",
			"cloneGenerationReviewNavigationTarget(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationConditionalState",
			"cloneGenerationNavigationDispatchPlan",
			"cloneGenerationQueueQuery",
			"buildGenerationReviewNavigationTarget",
			"cloneGenerationReviewNavigationTarget",
		})
	})
}
