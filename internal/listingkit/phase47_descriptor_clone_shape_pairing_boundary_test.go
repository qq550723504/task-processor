package listingkit

import "testing"

func TestGenerationNavigationDescriptorCloneShapePairingBoundary(t *testing.T) {
	t.Parallel()

	t.Run("descriptor_clone_shape_pairing_home_owns_only_local_home_pairing", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_target_identity.go", "applyGenerationNavigationDescriptorCloneShapePairing")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_target_identity.go", "applyGenerationNavigationDescriptorCloneShapePairing")

		assertSourceContainsAll(t, source, []string{
			"cloned.Conditional = cloneGenerationConditionalState(descriptor.Conditional)",
			"cloned.Invalidates = append([]string(nil), descriptor.Invalidates...)",
			"cloned.DispatchPlan = cloneGenerationNavigationDispatchPlan(descriptor.DispatchPlan)",
			"cloned.FollowUpReads = cloneGenerationNavigationFollowUpReadSlice(descriptor.FollowUpReads)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloneGenerationNavigationFollowUpRead(",
			"cloned.FollowUpReads = make([]GenerationNavigationFollowUpRead, 0, len(descriptor.FollowUpReads))",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneGenerationConditionalState",
			"cloneGenerationNavigationDispatchPlan",
			"cloneGenerationNavigationFollowUpReadSlice",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationNavigationFollowUpRead",
		})
	})

	t.Run("descriptor_clone_shape_pairing_home_keeps_slice_clone_local", func(t *testing.T) {
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

	t.Run("descriptor_clone_shape_home_routes_through_pairing_home", func(t *testing.T) {
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
			"cloned.FollowUpReads = cloneGenerationNavigationFollowUpReadSlice(descriptor.FollowUpReads)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationConditionalState",
			"cloneGenerationNavigationDispatchPlan",
			"cloneGenerationNavigationFollowUpReadSlice",
		})
	})
}
