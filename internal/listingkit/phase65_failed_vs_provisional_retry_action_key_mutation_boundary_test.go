package listingkit

import "testing"

func TestFailedVsProvisionalRetryActionKeyMutationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("retry_oriented_home_routes_failed_and_provisional_retry_homes", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_retry_oriented_mutation.go", "applyAssetGenerationRetryOrientedFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_retry_oriented_mutation.go", "applyAssetGenerationRetryOrientedFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"if applyAssetGenerationFailedRetryFilterMutation(actionKey, filters) {",
			"return applyAssetGenerationProvisionalRetryFilterMutation(actionKey, filters)",
		})
		assertSourceExcludesAll(t, source, []string{
			"case \"retry_failed_generation\", \"inspect_failed_renderer_tasks\":",
			"case \"upgrade_fallback_assets\", \"retry_provisional_slots\":",
			"case \"retry_section_generation\":",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationFailedRetryFilterMutation",
			"applyAssetGenerationProvisionalRetryFilterMutation",
		})
	})

	t.Run("failed_retry_home_owns_failed_retry_rule_family", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_failed_retry_mutation.go", "applyAssetGenerationFailedRetryFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_failed_retry_mutation.go", "applyAssetGenerationFailedRetryFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"case \"retry_failed_generation\", \"inspect_failed_renderer_tasks\":",
			"filters.ExecutionQuality = \"failed\"",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"applyAssetGenerationProvisionalRetryFilterMutation",
			"applyAssetGenerationIdealReviewFilters",
			"cloneAssetGenerationFilters",
		})
	})

	t.Run("provisional_retry_home_routes_section_retry_and_provisional_pair", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_provisional_retry_mutation.go", "applyAssetGenerationProvisionalRetryFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_provisional_retry_mutation.go", "applyAssetGenerationProvisionalRetryFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"if applyAssetGenerationSectionRetryFilterMutation(actionKey, filters) {",
			"case \"upgrade_fallback_assets\", \"retry_provisional_slots\":",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationSectionRetryFilterMutation",
		})
	})
}
