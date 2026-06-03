package listingkit

import "testing"

func TestGenerationNavigationDescriptorFollowUpReadCloneBoundary(t *testing.T) {
	t.Parallel()

	t.Run("followup_read_clone_home_owns_only_read_specific_shaping", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_followup_read_clone.go", "cloneGenerationNavigationFollowUpRead")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_followup_read_clone.go", "cloneGenerationNavigationFollowUpRead")

		assertSourceContainsAll(t, source, []string{
			"Kind:         item.Kind,",
			"ResponseMode: item.ResponseMode,",
			"Query:        cloneGenerationQueueQuery(item.Query),",
		})
		assertSourceExcludesAll(t, source, []string{
			"func cloneGenerationNavigationDescriptor(",
			"func cloneGenerationNavigationDispatchPlan(",
			"cloneGenerationConditionalState(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneGenerationQueueQuery",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationConditionalState",
			"cloneGenerationNavigationDispatchPlan",
		})
	})

	t.Run("descriptor_clone_shape_home_routes_followup_reads_through_read_clone_home", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_clone_shape.go", "run")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_clone_shape.go", "run")

		assertSourceContainsAll(t, source, []string{
			"cloned.FollowUpReads = make([]GenerationNavigationFollowUpRead, 0, len(descriptor.FollowUpReads))",
			"cloned.FollowUpReads = append(cloned.FollowUpReads, cloneGenerationNavigationFollowUpRead(item))",
		})
		assertSourceExcludesAll(t, source, []string{
			"Kind:         item.Kind,",
			"ResponseMode: item.ResponseMode,",
			"Query:        cloneGenerationQueueQuery(item.Query),",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneGenerationNavigationFollowUpRead",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationQueueQuery",
		})
	})
}
