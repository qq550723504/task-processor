package listingkit

import "testing"

func TestGenerationNavigationDescriptorResidualPairingBoundary(t *testing.T) {
	t.Parallel()

	t.Run("descriptor_residual_pairing_home_owns_only_conditional_invalidates_pairing", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_target_identity.go", "applyGenerationNavigationDescriptorCloneShapePairing")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_target_identity.go", "applyGenerationNavigationDescriptorCloneShapePairing")

		assertSourceContainsAll(t, source, []string{
			"cloned.Conditional = cloneGenerationConditionalState(descriptor.Conditional)",
			"cloned.Invalidates = append([]string(nil), descriptor.Invalidates...)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloneGenerationNavigationFollowUpRead(",
			"cloneGenerationQueueQuery(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneGenerationConditionalState",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationNavigationFollowUpRead",
			"cloneGenerationQueueQuery",
		})
	})

	t.Run("descriptor_clone_shape_pairing_home_routes_pairing_through_local_home", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_target_identity.go", "cloneGenerationNavigationDescriptor")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_target_identity.go", "cloneGenerationNavigationDescriptor")

		assertSourceContainsAll(t, source, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing(descriptor, &cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.Conditional = cloneGenerationConditionalState(descriptor.Conditional)",
			"cloned.Invalidates = append([]string(nil), descriptor.Invalidates...)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationConditionalState",
		})
	})
}
