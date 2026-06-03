package listingkit

import "testing"

func TestNonRetryRegularActionKeyMutationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("regular_action_key_home_routes_retry_and_review_ready_homes_then_keeps_missing_slot_rules", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_regular_mutation.go", "applyAssetGenerationRegularActionKeyFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_regular_mutation.go", "applyAssetGenerationRegularActionKeyFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"if applyAssetGenerationRetryOrientedFilterMutation(actionKey, filters) {",
			"if applyAssetGenerationReviewReadyFilterMutation(actionKey, filters) {",
			"case \"generate_missing_assets\", \"review_missing_slots\":",
		})
		assertSourceExcludesAll(t, source, []string{
			"case \"review_ready_assets\", \"continue_publish_review\":",
			"case \"defer_section_review\", \"approve_section_review\":",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationRetryOrientedFilterMutation",
			"applyAssetGenerationReviewReadyFilterMutation",
		})
	})

	t.Run("review_ready_home_owns_review_ready_and_section_review_rule_family", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_review_ready_mutation.go", "applyAssetGenerationReviewReadyFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_review_ready_mutation.go", "applyAssetGenerationReviewReadyFilterMutation")

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
