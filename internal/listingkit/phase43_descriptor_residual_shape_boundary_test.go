package listingkit

import "testing"

func TestGenerationNavigationDescriptorResidualShapeBoundary(t *testing.T) {
	t.Parallel()

	t.Run("descriptor_clone_shape_pairing_home_routes_residual_and_dispatch_plan_through_local_homes", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_clone_shape_pairing.go", "applyGenerationNavigationDescriptorCloneShapePairing")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_clone_shape_pairing.go", "applyGenerationNavigationDescriptorCloneShapePairing")

		assertSourceContainsAll(t, source, []string{
			"cloned.Conditional = cloneGenerationConditionalState(descriptor.Conditional)",
			"cloned.Invalidates = append([]string(nil), descriptor.Invalidates...)",
			"cloned.DispatchPlan = cloneGenerationNavigationDispatchPlan(descriptor.DispatchPlan)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloneGenerationNavigationFollowUpRead(",
			"cloneGenerationQueueQuery(",
			"func cloneGenerationNavigationDescriptor(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneGenerationConditionalState",
			"cloneGenerationNavigationDispatchPlan",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationNavigationFollowUpRead",
			"cloneGenerationQueueQuery",
		})
	})

	t.Run("descriptor_shape_home_routes_residual_shape_through_local_home", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_target_identity.go", "run")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_target_identity.go", "run")

		assertSourceContainsAll(t, source, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing(descriptor, cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.Conditional = cloneGenerationConditionalState(descriptor.Conditional)",
			"cloned.Invalidates = append([]string(nil), descriptor.Invalidates...)",
			"cloned.DispatchPlan = cloneGenerationNavigationDispatchPlan(descriptor.DispatchPlan)",
			"cloned.FollowUpReads = make([]GenerationNavigationFollowUpRead, 0, len(descriptor.FollowUpReads))",
			"cloned.FollowUpReads = append(cloned.FollowUpReads, cloneGenerationNavigationFollowUpRead(item))",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationConditionalState",
			"cloneGenerationNavigationDispatchPlan",
			"cloneGenerationNavigationFollowUpRead",
		})
	})
}
