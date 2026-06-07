package listingkit

import "testing"

func TestGenerationNavigationDescriptorFollowUpReadCloneBoundary(t *testing.T) {
	t.Parallel()

	t.Run("followup_read_clone_home_owns_only_read_specific_shaping", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_target_identity.go", "cloneGenerationNavigationFollowUpRead")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_target_identity.go", "cloneGenerationNavigationFollowUpRead")

		assertSourceContainsAll(t, source, []string{
			"cloned := item",
			"applyGenerationNavigationFollowUpReadCloneShape(item, &cloned)",
			"return cloned",
		})
		assertSourceExcludesAll(t, source, []string{
			"func cloneGenerationNavigationDescriptor(",
			"func cloneGenerationNavigationDispatchPlan(",
			"cloneGenerationConditionalState(",
			"Kind:         item.Kind,",
			"ResponseMode: item.ResponseMode,",
			"Query:        cloneGenerationQueueQuery(item.Query),",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationFollowUpReadCloneShape",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationQueueQuery",
			"cloneGenerationConditionalState",
			"cloneGenerationNavigationDispatchPlan",
		})
	})

	t.Run("descriptor_clone_shape_home_routes_followup_reads_through_read_clone_home", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_target_identity.go", "cloneGenerationNavigationDescriptor")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_target_identity.go", "cloneGenerationNavigationDescriptor")

		assertSourceContainsAll(t, source, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing(descriptor, &cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.FollowUpReads = make([]GenerationNavigationFollowUpRead, 0, len(descriptor.FollowUpReads))",
			"cloned.FollowUpReads = append(cloned.FollowUpReads, cloneGenerationNavigationFollowUpRead(item))",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationNavigationFollowUpRead",
			"cloneGenerationQueueQuery",
		})
	})
}
