package listingkit

import "testing"

func TestProvisionalVsSectionRetryActionKeyMutationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("provisional_retry_home_routes_section_retry_and_provisional_pair", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_provisional_retry_mutation.go", "applyAssetGenerationProvisionalRetryFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_provisional_retry_mutation.go", "applyAssetGenerationProvisionalRetryFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"if applyAssetGenerationSectionRetryFilterMutation(actionKey, filters) {",
			"case \"upgrade_fallback_assets\", \"retry_provisional_slots\":",
		})
		assertSourceExcludesAll(t, source, []string{
			"case \"retry_section_generation\":",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationSectionRetryFilterMutation",
		})
	})

	t.Run("section_retry_home_owns_section_retry_rule_family", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_section_retry_mutation.go", "applyAssetGenerationSectionRetryFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_section_retry_mutation.go", "applyAssetGenerationSectionRetryFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"case \"retry_section_generation\":",
			"filters.QualityGrade = \"provisional\"",
			"filters.RetryableOnly = true",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"applyAssetGenerationProvisionalRetryFilterMutation",
			"applyAssetGenerationFailedRetryFilterMutation",
			"cloneAssetGenerationFilters",
		})
	})
}
