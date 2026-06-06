package listingkit

import "testing"

func TestActionTargetFilterMutationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("action_target_filter_mutation_home_owns_clone_init_and_dispatch", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_overview.go", "actionFiltersForKey")
		callNames := readNamedFunctionCallNames(t, "generation_overview.go", "actionFiltersForKey")

		assertSourceContainsAll(t, source, []string{
			"filters := cloneAssetGenerationFilters(base)",
			"filters = &AssetGenerationRecommendedFilters{}",
			"applyAssetGenerationActionFiltersMutation(actionKey, filters)",
		})
		assertSourceExcludesAll(t, source, []string{
			"PreviewCapabilityActionSpecForKey(actionKey)",
			"filters.QualityGrade = \"missing\"",
			"filters.QualityGrade = \"provisional\"",
			"filters.RenderPreviewAvailable = true",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneAssetGenerationFilters",
			"applyAssetGenerationActionFiltersMutation",
		})
	})

	t.Run("action_target_filter_mutation_shape_home_routes_preview_and_regular_action_key_homes", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_overview.go", "applyAssetGenerationActionFiltersMutation")
		callNames := readNamedFunctionCallNames(t, "generation_overview.go", "applyAssetGenerationActionFiltersMutation")

		assertSourceContainsAll(t, source, []string{
			"if applyAssetGenerationPreviewCapabilityFilterMutation(actionKey, filters) {",
			"applyAssetGenerationRegularActionKeyFilterMutation(actionKey, filters)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationPreviewCapabilityFilterMutation",
			"applyAssetGenerationRegularActionKeyFilterMutation",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneAssetGenerationFilters",
			"cloneGenerationQueueQuery",
			"cloneRetryGenerationTasksRequest",
		})
	})
}
