package listingkit

import "testing"

func TestGenerationNavigationDescriptorResidualPairingBoundary(t *testing.T) {
	t.Parallel()

	t.Run("descriptor_residual_pairing_home_owns_only_conditional_invalidates_pairing", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_residual_pairing.go", "applyGenerationNavigationDescriptorResidualClonePairing")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_residual_pairing.go", "applyGenerationNavigationDescriptorResidualClonePairing")

		assertSourceContainsAll(t, source, []string{
			"cloned.Conditional = cloneGenerationConditionalState(descriptor.Conditional)",
			"cloned.Invalidates = append([]string(nil), descriptor.Invalidates...)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloneGenerationNavigationDispatchPlan(",
			"cloneGenerationNavigationFollowUpRead(",
			"cloneGenerationQueueQuery(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneGenerationConditionalState",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationNavigationDispatchPlan",
			"cloneGenerationNavigationFollowUpRead",
			"cloneGenerationQueueQuery",
		})
	})

	t.Run("descriptor_residual_shape_home_routes_pairing_through_local_home", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_residual_shape.go", "applyGenerationNavigationDescriptorResidualCloneShape")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_residual_shape.go", "applyGenerationNavigationDescriptorResidualCloneShape")

		assertSourceContainsAll(t, source, []string{
			"applyGenerationNavigationDescriptorResidualClonePairing(descriptor, cloned)",
			"applyGenerationNavigationDescriptorDispatchPlanCloneDelegation(descriptor, cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.Conditional = cloneGenerationConditionalState(descriptor.Conditional)",
			"cloned.Invalidates = append([]string(nil), descriptor.Invalidates...)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorResidualClonePairing",
			"applyGenerationNavigationDescriptorDispatchPlanCloneDelegation",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationConditionalState",
		})
	})
}
