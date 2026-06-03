package listingkit

import "testing"

func TestRetryOrientedActionKeyFilterMutationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("regular_action_key_home_routes_retry_oriented_family", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_regular_mutation.go", "applyAssetGenerationRegularActionKeyFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_regular_mutation.go", "applyAssetGenerationRegularActionKeyFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"if applyAssetGenerationRetryOrientedFilterMutation(actionKey, filters) {",
			"case \"generate_missing_assets\", \"review_missing_slots\":",
			"if applyAssetGenerationReviewReadyFilterMutation(actionKey, filters) {",
		})
		assertSourceExcludesAll(t, source, []string{
			"case \"retry_failed_generation\", \"inspect_failed_renderer_tasks\":",
			"case \"upgrade_fallback_assets\", \"retry_provisional_slots\":",
			"case \"retry_section_generation\":",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationRetryOrientedFilterMutation",
			"applyAssetGenerationReviewReadyFilterMutation",
		})
	})

	t.Run("retry_oriented_mutation_home_routes_failed_and_provisional_retry_families", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_retry_oriented_mutation.go", "applyAssetGenerationRetryOrientedFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_retry_oriented_mutation.go", "applyAssetGenerationRetryOrientedFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"if applyAssetGenerationFailedRetryFilterMutation(actionKey, filters) {",
			"return applyAssetGenerationProvisionalRetryFilterMutation(actionKey, filters)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationFailedRetryFilterMutation",
			"applyAssetGenerationProvisionalRetryFilterMutation",
		})
	})
}
