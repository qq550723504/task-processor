package listingkit

import "testing"

func TestRegularActionKeyFilterMutationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("broader_filter_mutation_home_routes_regular_action_key_rules_only", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_mutation.go", "applyAssetGenerationActionFiltersMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_mutation.go", "applyAssetGenerationActionFiltersMutation")

		assertSourceContainsAll(t, source, []string{
			"if applyAssetGenerationPreviewCapabilityFilterMutation(actionKey, filters) {",
			"applyAssetGenerationRegularActionKeyFilterMutation(actionKey, filters)",
		})
		assertSourceExcludesAll(t, source, []string{
			"case \"generate_missing_assets\", \"review_missing_slots\":",
			"case \"retry_failed_generation\", \"inspect_failed_renderer_tasks\":",
			"case \"defer_section_review\", \"approve_section_review\":",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationPreviewCapabilityFilterMutation",
			"applyAssetGenerationRegularActionKeyFilterMutation",
		})
	})

	t.Run("regular_action_key_mutation_home_routes_retry_and_non_retry_rule_families", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_regular_mutation.go", "applyAssetGenerationRegularActionKeyFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_regular_mutation.go", "applyAssetGenerationRegularActionKeyFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"if applyAssetGenerationRetryOrientedFilterMutation(actionKey, filters) {",
			"case \"generate_missing_assets\", \"review_missing_slots\":",
			"case \"review_ready_assets\", \"continue_publish_review\":",
			"case \"defer_section_review\", \"approve_section_review\":",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationRetryOrientedFilterMutation",
			"applyAssetGenerationIdealReviewFilters",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"applyAssetGenerationPreviewCapabilityFilterMutation",
			"cloneAssetGenerationFilters",
			"cloneGenerationQueueQuery",
			"cloneRetryGenerationTasksRequest",
		})
	})
}
