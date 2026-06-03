package listingkit

import "testing"

func TestActionTargetPreviewCapabilityFilterMutationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("preview_capability_filter_mutation_home_owns_capability_specialization", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_preview_capability_mutation.go", "applyAssetGenerationPreviewCapabilityFilterMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_preview_capability_mutation.go", "applyAssetGenerationPreviewCapabilityFilterMutation")

		assertSourceContainsAll(t, source, []string{
			"spec := listinggeneration.PreviewCapabilityActionSpecForKey(actionKey)",
			"filters.ExecutionQuality = \"\"",
			"filters.RetryableOnly = false",
			"filters.RenderPreviewAvailable = true",
			"filters.PreviewCapability = spec.Capability",
			"applyAssetGenerationIdealReviewFilters(filters)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"PreviewCapabilityActionSpecForKey",
			"applyAssetGenerationIdealReviewFilters",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneAssetGenerationFilters",
			"cloneGenerationQueueQuery",
			"cloneRetryGenerationTasksRequest",
			"applyAssetGenerationActionFiltersMutation",
		})
	})

	t.Run("broader_filter_mutation_home_routes_preview_specialization_only", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_action_filters_mutation.go", "applyAssetGenerationActionFiltersMutation")
		callNames := readNamedFunctionCallNames(t, "generation_action_filters_mutation.go", "applyAssetGenerationActionFiltersMutation")

		assertSourceContainsAll(t, source, []string{
			"if applyAssetGenerationPreviewCapabilityFilterMutation(actionKey, filters) {",
		})
		assertSourceExcludesAll(t, source, []string{
			"PreviewCapabilityActionSpecForKey(actionKey)",
			"filters.RenderPreviewAvailable = true",
			"filters.PreviewCapability = spec.Capability",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationPreviewCapabilityFilterMutation",
		})
	})
}
