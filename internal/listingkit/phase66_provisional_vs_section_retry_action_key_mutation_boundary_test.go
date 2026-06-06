package listingkit

import "testing"

func TestProvisionalVsSectionRetryActionKeyMutationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("regular_action_key_home_owns_section_retry_and_provisional_pair", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_overview.go", "applyAssetGenerationProvisionalRetryFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_overview.go", "applyAssetGenerationProvisionalRetryFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"case \"retry_section_generation\":",
			"case \"upgrade_fallback_assets\", \"retry_provisional_slots\":",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"applyAssetGenerationFailedRetryFilterMutation",
			"cloneAssetGenerationFilters",
			"applyAssetGenerationSectionRetryFilterMutation",
		})
	})
}
