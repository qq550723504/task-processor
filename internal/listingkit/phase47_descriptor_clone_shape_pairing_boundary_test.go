package listingkit

import "testing"

func TestGenerationNavigationDescriptorCloneShapePairingBoundary(t *testing.T) {
	t.Parallel()

	t.Run("descriptor_clone_shape_pairing_home_owns_only_local_home_pairing", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_clone_shape_pairing.go", "applyGenerationNavigationDescriptorCloneShapePairing")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_clone_shape_pairing.go", "applyGenerationNavigationDescriptorCloneShapePairing")

		assertSourceContainsAll(t, source, []string{
			"applyGenerationNavigationDescriptorResidualCloneShape(descriptor, cloned)",
			"applyGenerationNavigationDescriptorFollowUpReadCloneRouting(descriptor, cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloneGenerationConditionalState(",
			"cloneGenerationNavigationDispatchPlan(",
			"cloneGenerationNavigationFollowUpRead(",
			"cloned.FollowUpReads = make([]GenerationNavigationFollowUpRead, 0, len(descriptor.FollowUpReads))",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorResidualCloneShape",
			"applyGenerationNavigationDescriptorFollowUpReadCloneRouting",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationConditionalState",
			"cloneGenerationNavigationDispatchPlan",
			"cloneGenerationNavigationFollowUpRead",
		})
	})

	t.Run("descriptor_clone_shape_home_routes_through_pairing_home", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_clone_shape.go", "run")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_clone_shape.go", "run")

		assertSourceContainsAll(t, source, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing(descriptor, cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"applyGenerationNavigationDescriptorResidualCloneShape(descriptor, cloned)",
			"applyGenerationNavigationDescriptorFollowUpReadCloneRouting(descriptor, cloned)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorResidualCloneShape",
			"applyGenerationNavigationDescriptorFollowUpReadCloneRouting",
		})
	})
}
