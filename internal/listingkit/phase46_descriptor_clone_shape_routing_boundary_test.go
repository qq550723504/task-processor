package listingkit

import "testing"

func TestGenerationNavigationDescriptorCloneShapeRoutingBoundary(t *testing.T) {
	t.Parallel()

	t.Run("descriptor_followup_read_routing_pairing_home_owns_only_followup_read_slice_routing", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_target_identity.go", "applyGenerationNavigationDescriptorCloneShapePairing")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_target_identity.go", "applyGenerationNavigationDescriptorCloneShapePairing")

		assertSourceContainsAll(t, source, []string{
			"cloned.FollowUpReads = cloneGenerationNavigationFollowUpReadSlice(descriptor.FollowUpReads)",
		})
		assertSourceExcludesAll(t, source, []string{
			"applyGenerationNavigationDescriptorResidualClonePairing(",
			"if len(descriptor.FollowUpReads) == 0 {",
			"cloned.FollowUpReads = make([]GenerationNavigationFollowUpRead, 0, len(descriptor.FollowUpReads))",
			"cloned.FollowUpReads = append(cloned.FollowUpReads, cloneGenerationNavigationFollowUpRead(item))",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneGenerationNavigationFollowUpReadSlice",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationNavigationFollowUpRead",
			"applyGenerationNavigationDescriptorResidualClonePairing",
		})
	})

	t.Run("descriptor_clone_shape_home_routes_through_local_routing_homes", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_target_identity.go", "cloneGenerationNavigationDescriptor")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_target_identity.go", "cloneGenerationNavigationDescriptor")

		assertSourceContainsAll(t, source, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing(descriptor, &cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.FollowUpReads = cloneGenerationNavigationFollowUpReadSlice(descriptor.FollowUpReads)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationNavigationFollowUpRead",
		})
	})
}
