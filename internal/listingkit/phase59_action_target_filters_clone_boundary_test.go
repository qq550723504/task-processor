package listingkit

import "testing"

func TestActionTargetFiltersCloneBoundary(t *testing.T) {
	t.Parallel()

	t.Run("action_target_filters_clone_home_owns_top_level_copy_only", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_overview.go", "cloneAssetGenerationFilters")
		callNames := readNamedFunctionCallNames(t, "generation_overview.go", "cloneAssetGenerationFilters")

		assertSourceContainsAll(t, source, []string{
			"cloned := *filters",
			"applyAssetGenerationFiltersPlatformsClone(filters, &cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.Platforms = append([]string(nil), filters.Platforms...)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationFiltersPlatformsClone",
		})
	})

	t.Run("action_target_filters_clone_home_routes_platforms_slice_through_local_home", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_overview.go", "cloneAssetGenerationFilters")
		callNames := readNamedFunctionCallNames(t, "generation_overview.go", "cloneAssetGenerationFilters")

		assertSourceContainsAll(t, source, []string{
			"applyAssetGenerationFiltersPlatformsClone(filters, &cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.Platforms = append([]string(nil), filters.Platforms...)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationFiltersPlatformsClone",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationQueueQuery",
			"cloneRetryGenerationTasksRequest",
		})
	})
}
