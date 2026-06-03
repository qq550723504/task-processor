package listingkit

import "testing"

func TestGenerationNavigationDescriptorResidualShapeBoundary(t *testing.T) {
	t.Parallel()

	t.Run("descriptor_residual_shape_home_owns_only_residual_shape", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_residual_shape.go", "applyGenerationNavigationDescriptorResidualCloneShape")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_residual_shape.go", "applyGenerationNavigationDescriptorResidualCloneShape")

		assertSourceContainsAll(t, source, []string{
			"cloned.Conditional = cloneGenerationConditionalState(descriptor.Conditional)",
			"cloned.DispatchPlan = cloneGenerationNavigationDispatchPlan(descriptor.DispatchPlan)",
			"cloned.Invalidates = append([]string(nil), descriptor.Invalidates...)",
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

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_clone_shape.go", "run")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_clone_shape.go", "run")

		assertSourceContainsAll(t, source, []string{
			"applyGenerationNavigationDescriptorResidualCloneShape(descriptor, cloned)",
			"cloned.FollowUpReads = make([]GenerationNavigationFollowUpRead, 0, len(descriptor.FollowUpReads))",
			"cloned.FollowUpReads = append(cloned.FollowUpReads, cloneGenerationNavigationFollowUpRead(item))",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.Conditional = cloneGenerationConditionalState(descriptor.Conditional)",
			"cloned.DispatchPlan = cloneGenerationNavigationDispatchPlan(descriptor.DispatchPlan)",
			"cloned.Invalidates = append([]string(nil), descriptor.Invalidates...)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorResidualCloneShape",
			"cloneGenerationNavigationFollowUpRead",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationConditionalState",
			"cloneGenerationNavigationDispatchPlan",
		})
	})
}
