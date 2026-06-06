package listingkit

import "testing"

func TestGenerationNavigationDescriptorFollowUpReadSliceCloneBoundary(t *testing.T) {
	t.Parallel()

	t.Run("descriptor_clone_shape_pairing_home_owns_slice_orchestration", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_target_identity.go", "cloneGenerationNavigationFollowUpReadSlice")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_target_identity.go", "cloneGenerationNavigationFollowUpReadSlice")

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

	t.Run("descriptor_clone_shape_pairing_home_routes_through_local_slice_clone", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_target_identity.go", "applyGenerationNavigationDescriptorCloneShapePairing")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_target_identity.go", "applyGenerationNavigationDescriptorCloneShapePairing")

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
