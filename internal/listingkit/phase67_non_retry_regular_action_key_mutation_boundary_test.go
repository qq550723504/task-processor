package listingkit

import "testing"

func TestNonRetryRegularActionKeyMutationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("regular_action_key_home_routes_retry_review_ready_and_missing_slot_homes_only", func(t *testing.T) {
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
			"case \"generate_missing_assets\", \"review_missing_slots\":",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationFailedRetryFilterMutation",
			"applyAssetGenerationProvisionalRetryFilterMutation",
			"applyAssetGenerationReviewReadyFilterMutation",
			"applyAssetGenerationMissingSlotFilterMutation",
		})
	})

	t.Run("preview_capability_home_owns_review_ready_and_section_review_rule_family", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_preview_capability_mutation.go", "applyAssetGenerationReviewReadyFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_preview_capability_mutation.go", "applyAssetGenerationReviewReadyFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"case \"review_ready_assets\", \"continue_publish_review\":",
			"case \"defer_section_review\", \"approve_section_review\":",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationIdealReviewFilters",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"applyAssetGenerationRetryOrientedFilterMutation",
			"cloneAssetGenerationFilters",
			"cloneGenerationQueueQuery",
			"cloneRetryGenerationTasksRequest",
		})
	})
}
