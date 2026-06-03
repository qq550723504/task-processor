package listingkit

import "testing"

func TestGenerationNavigationDescriptorFollowUpReadRoutingPairingBoundary(t *testing.T) {
	t.Parallel()

	t.Run("followup_read_routing_pairing_home_owns_slice_and_item_dispatch", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_followup_read_routing_pairing.go", "applyGenerationNavigationDescriptorFollowUpReadRoutingPairing")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_followup_read_routing_pairing.go", "applyGenerationNavigationDescriptorFollowUpReadRoutingPairing")

		assertSourceContainsAll(t, source, []string{
			"cloned.FollowUpReads = cloneGenerationNavigationFollowUpReadSlice(descriptor.FollowUpReads)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloneGenerationQueueQuery(",
			"cloned.FollowUpReads = make([]GenerationNavigationFollowUpRead, 0, len(descriptor.FollowUpReads))",
			"cloned.FollowUpReads = append(cloned.FollowUpReads, cloneGenerationNavigationFollowUpRead(item))",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneGenerationNavigationFollowUpReadSlice",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationNavigationFollowUpRead",
			"cloneGenerationQueueQuery",
		})
	})

	t.Run("followup_read_routing_home_routes_through_pairing_home", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_followup_read_routing.go", "applyGenerationNavigationDescriptorFollowUpReadCloneRouting")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_followup_read_routing.go", "applyGenerationNavigationDescriptorFollowUpReadCloneRouting")

		assertSourceContainsAll(t, source, []string{
			"applyGenerationNavigationDescriptorFollowUpReadRoutingPairing(descriptor, cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.FollowUpReads = make([]GenerationNavigationFollowUpRead, 0, len(descriptor.FollowUpReads))",
			"cloneGenerationNavigationFollowUpRead(item)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorFollowUpReadRoutingPairing",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationNavigationFollowUpRead",
		})
	})
}
