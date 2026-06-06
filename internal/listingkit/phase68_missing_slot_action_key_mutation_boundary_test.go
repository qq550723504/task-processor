package listingkit

import "testing"

func TestMissingSlotActionKeyMutationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("regular_action_key_home_routes_retry_review_ready_and_missing_slot_rules_only", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_overview.go", "applyAssetGenerationRegularActionKeyFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_overview.go", "applyAssetGenerationRegularActionKeyFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"switch {",
			"case applyAssetGenerationFailedRetryFilterMutation(actionKey, filters):",
			"case applyAssetGenerationProvisionalRetryFilterMutation(actionKey, filters):",
			"case applyAssetGenerationReviewReadyFilterMutation(actionKey, filters):",
			"case \"generate_missing_assets\", \"review_missing_slots\":",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationFailedRetryFilterMutation",
			"applyAssetGenerationProvisionalRetryFilterMutation",
			"applyAssetGenerationReviewReadyFilterMutation",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneAssetGenerationFilters",
			"cloneGenerationQueueQuery",
			"cloneRetryGenerationTasksRequest",
			"applyAssetGenerationMissingSlotFilterMutation",
		})
	})
}
