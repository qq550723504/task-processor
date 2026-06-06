package listingkit

import "testing"

func TestGenerationNavigationFollowUpReadItemCloneBoundary(t *testing.T) {
	t.Parallel()

	t.Run("followup_read_item_clone_home_owns_top_level_copy_only", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_target_identity.go", "cloneGenerationNavigationFollowUpRead")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_target_identity.go", "cloneGenerationNavigationFollowUpRead")

		assertSourceContainsAll(t, source, []string{
			"cloned := item",
			"applyGenerationNavigationFollowUpReadCloneShape(item, &cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"Query:        cloneGenerationQueueQuery(item.Query),",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationFollowUpReadCloneShape",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationQueueQuery",
		})
	})

	t.Run("followup_read_clone_shape_home_owns_query_clone_delegation", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_target_identity.go", "applyGenerationNavigationFollowUpReadCloneShape")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_target_identity.go", "applyGenerationNavigationFollowUpReadCloneShape")

		assertSourceContainsAll(t, source, []string{
			"cloned.Query = cloneGenerationQueueQuery(item.Query)",
		})
		assertSourceExcludesAll(t, source, []string{
			"Kind:         item.Kind,",
			"ResponseMode: item.ResponseMode,",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneGenerationQueueQuery",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{})
	})
}
