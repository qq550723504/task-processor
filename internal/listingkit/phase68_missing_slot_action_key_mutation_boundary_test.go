package listingkit

import "testing"

func TestMissingSlotActionKeyMutationBoundary(t *testing.T) {
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

	t.Run("missing_slot_home_owns_missing_slot_rule_family", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_missing_slot_mutation.go", "applyAssetGenerationMissingSlotFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_missing_slot_mutation.go", "applyAssetGenerationMissingSlotFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"case \"generate_missing_assets\", \"review_missing_slots\":",
			"filters.QualityGrade = \"missing\"",
			"filters.ExecutionQuality = \"\"",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"applyAssetGenerationRetryOrientedFilterMutation",
			"applyAssetGenerationReviewReadyFilterMutation",
			"cloneAssetGenerationFilters",
			"cloneGenerationQueueQuery",
			"cloneRetryGenerationTasksRequest",
		})
	})
}
