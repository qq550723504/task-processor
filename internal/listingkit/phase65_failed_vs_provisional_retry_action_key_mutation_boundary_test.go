package listingkit

import "testing"

func TestFailedVsProvisionalRetryActionKeyMutationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("regular_action_key_home_routes_failed_and_provisional_retry_homes", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_overview.go", "applyAssetGenerationRegularActionKeyFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_overview.go", "applyAssetGenerationRegularActionKeyFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"switch {",
			"case applyAssetGenerationFailedRetryFilterMutation(actionKey, filters):",
			"case applyAssetGenerationProvisionalRetryFilterMutation(actionKey, filters):",
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

	t.Run("regular_action_key_home_owns_failed_retry_rule_family", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_overview_filter_support.go", "applyAssetGenerationFailedRetryFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_overview_filter_support.go", "applyAssetGenerationFailedRetryFilterMutation")

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

	t.Run("regular_action_key_home_owns_provisional_retry_rule_family", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_overview_filter_support.go", "applyAssetGenerationProvisionalRetryFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_overview_filter_support.go", "applyAssetGenerationProvisionalRetryFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"case \"retry_section_generation\":",
			"case \"upgrade_fallback_assets\", \"retry_provisional_slots\":",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"applyAssetGenerationSectionRetryFilterMutation",
		})
	})
}
