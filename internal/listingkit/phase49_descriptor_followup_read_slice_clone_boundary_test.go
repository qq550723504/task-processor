package listingkit

import "testing"

func TestGenerationNavigationDescriptorFollowUpReadSliceCloneBoundary(t *testing.T) {
	t.Parallel()

	t.Run("followup_read_slice_clone_home_owns_slice_orchestration", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_followup_read_slice_clone.go", "cloneGenerationNavigationFollowUpReadSlice")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_followup_read_slice_clone.go", "cloneGenerationNavigationFollowUpReadSlice")

		assertSourceContainsAll(t, source, []string{
			"cloned := make([]GenerationNavigationFollowUpRead, 0, len(items))",
			"cloned = append(cloned, cloneGenerationNavigationFollowUpRead(item))",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloneGenerationQueueQuery(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneGenerationNavigationFollowUpRead",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationQueueQuery",
		})
	})

	t.Run("followup_read_routing_pairing_home_routes_through_slice_clone_home", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_followup_read_routing_pairing.go", "applyGenerationNavigationDescriptorFollowUpReadRoutingPairing")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_followup_read_routing_pairing.go", "applyGenerationNavigationDescriptorFollowUpReadRoutingPairing")

		assertSourceContainsAll(t, source, []string{
			"cloned.FollowUpReads = cloneGenerationNavigationFollowUpReadSlice(descriptor.FollowUpReads)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.FollowUpReads = make([]GenerationNavigationFollowUpRead, 0, len(descriptor.FollowUpReads))",
			"cloneGenerationNavigationFollowUpRead(item)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneGenerationNavigationFollowUpReadSlice",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationNavigationFollowUpRead",
		})
	})
}
