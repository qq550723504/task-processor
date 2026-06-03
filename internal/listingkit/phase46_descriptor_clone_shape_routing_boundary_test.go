package listingkit

import "testing"

func TestGenerationNavigationDescriptorCloneShapeRoutingBoundary(t *testing.T) {
	t.Parallel()

	t.Run("descriptor_followup_read_routing_home_owns_only_followup_read_slice_routing", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_followup_read_routing.go", "applyGenerationNavigationDescriptorFollowUpReadCloneRouting")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_followup_read_routing.go", "applyGenerationNavigationDescriptorFollowUpReadCloneRouting")

		assertSourceContainsAll(t, source, []string{
			"applyGenerationNavigationDescriptorFollowUpReadRoutingPairing(descriptor, cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"applyGenerationNavigationDescriptorResidualCloneShape(",
			"cloneGenerationConditionalState(",
			"cloneGenerationNavigationDispatchPlan(",
			"if len(descriptor.FollowUpReads) == 0 {",
			"cloned.FollowUpReads = make([]GenerationNavigationFollowUpRead, 0, len(descriptor.FollowUpReads))",
			"cloned.FollowUpReads = append(cloned.FollowUpReads, cloneGenerationNavigationFollowUpRead(item))",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorFollowUpReadRoutingPairing",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationNavigationFollowUpRead",
			"applyGenerationNavigationDescriptorResidualCloneShape",
			"cloneGenerationConditionalState",
			"cloneGenerationNavigationDispatchPlan",
		})
	})

	t.Run("descriptor_clone_shape_home_routes_through_local_routing_homes", func(t *testing.T) {
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
			"cloneGenerationNavigationFollowUpRead",
		})
	})
}
