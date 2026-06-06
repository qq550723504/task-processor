package listingkit

import "testing"

func TestRetryOrientedActionKeyFilterMutationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("regular_action_key_home_routes_retry_oriented_family", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_regular_mutation.go", "applyAssetGenerationRegularActionKeyFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_regular_mutation.go", "applyAssetGenerationRegularActionKeyFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"if applyAssetGenerationFailedRetryFilterMutation(actionKey, filters) {",
			"if applyAssetGenerationProvisionalRetryFilterMutation(actionKey, filters) {",
			"if applyAssetGenerationReviewReadyFilterMutation(actionKey, filters) {",
			"applyAssetGenerationMissingSlotFilterMutation(actionKey, filters)",
		})
		assertSourceExcludesAll(t, source, []string{
			"case \"retry_failed_generation\", \"inspect_failed_renderer_tasks\":",
			"case \"upgrade_fallback_assets\", \"retry_provisional_slots\":",
			"case \"retry_section_generation\":",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationFailedRetryFilterMutation",
			"applyAssetGenerationProvisionalRetryFilterMutation",
			"applyAssetGenerationReviewReadyFilterMutation",
			"applyAssetGenerationMissingSlotFilterMutation",
		})
	})
}
