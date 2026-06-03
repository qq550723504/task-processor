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

	t.Run("action_target_filter_mutation_shape_home_owns_preview_and_action_key_rules", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_mutation.go", "applyAssetGenerationActionFiltersMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_mutation.go", "applyAssetGenerationActionFiltersMutation")

		assertSourceContainsAll(t, source, []string{
			"if applyAssetGenerationPreviewCapabilityFilterMutation(actionKey, filters) {",
			"case \"generate_missing_assets\", \"review_missing_slots\":",
			"case \"retry_failed_generation\", \"inspect_failed_renderer_tasks\":",
			"case \"defer_section_review\", \"approve_section_review\":",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationPreviewCapabilityFilterMutation",
			"applyAssetGenerationIdealReviewFilters",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneAssetGenerationFilters",
			"cloneGenerationQueueQuery",
			"cloneRetryGenerationTasksRequest",
		})
	})
}
