package listingkit

import "testing"

func TestActionTargetFiltersPlatformSliceBoundary(t *testing.T) {
	t.Parallel()

	t.Run("action_target_filters_platforms_clone_home_owns_only_platforms_slice", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_filters_platforms_clone.go", "applyAssetGenerationFiltersPlatformsClone")
		callNames := readNamedFunctionCallNames(t, "generation_filters_platforms_clone.go", "applyAssetGenerationFiltersPlatformsClone")

		assertSourceContainsAll(t, source, []string{
			"cloned.Platforms = append([]string(nil), filters.Platforms...)",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"applyAssetGenerationFiltersCloneShape",
			"cloneAssetGenerationFilters",
			"cloneGenerationQueueQuery",
		})
	})
}
