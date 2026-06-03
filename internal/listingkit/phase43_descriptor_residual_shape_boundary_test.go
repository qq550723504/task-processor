package listingkit

import "testing"

func TestGenerationNavigationDescriptorResidualShapeBoundary(t *testing.T) {
	t.Parallel()

	t.Run("descriptor_residual_shape_home_owns_only_residual_shape", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_residual_shape.go", "applyGenerationNavigationDescriptorResidualCloneShape")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_residual_shape.go", "applyGenerationNavigationDescriptorResidualCloneShape")

		assertSourceContainsAll(t, source, []string{
			"applyGenerationNavigationDescriptorResidualClonePairing(descriptor, cloned)",
			"applyGenerationNavigationDescriptorDispatchPlanCloneDelegation(descriptor, cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloneGenerationNavigationFollowUpRead(",
			"cloneGenerationQueueQuery(",
			"func cloneGenerationNavigationDescriptor(",
			"cloned.Conditional = cloneGenerationConditionalState(descriptor.Conditional)",
			"cloned.DispatchPlan = cloneGenerationNavigationDispatchPlan(descriptor.DispatchPlan)",
			"cloned.Invalidates = append([]string(nil), descriptor.Invalidates...)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorResidualClonePairing",
			"applyGenerationNavigationDescriptorDispatchPlanCloneDelegation",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationConditionalState",
			"cloneGenerationNavigationDispatchPlan",
			"cloneGenerationNavigationFollowUpRead",
			"cloneGenerationQueueQuery",
		})
	})

	t.Run("descriptor_shape_home_routes_residual_shape_through_local_home", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_clone_shape.go", "run")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_clone_shape.go", "run")

		assertSourceContainsAll(t, source, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing(descriptor, cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"applyGenerationNavigationDescriptorResidualCloneShape(descriptor, cloned)",
			"cloned.FollowUpReads = make([]GenerationNavigationFollowUpRead, 0, len(descriptor.FollowUpReads))",
			"cloned.FollowUpReads = append(cloned.FollowUpReads, cloneGenerationNavigationFollowUpRead(item))",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorResidualCloneShape",
			"cloneGenerationNavigationFollowUpRead",
		})
	})
}
