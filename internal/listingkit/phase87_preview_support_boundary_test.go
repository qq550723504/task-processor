package listingkit

import "testing"

func TestSheinPreviewSupportBoundary(t *testing.T) {
	t.Parallel()

	t.Run("resolution cache summary delegates to workspace", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "preview_builder_shein_resolution_cache.go", "buildSheinResolutionCacheSummary")
		callNames := readNamedFunctionCallNames(t, "preview_builder_shein_resolution_cache.go", "buildSheinResolutionCacheSummary")

		assertSourceContainsAll(t, source, []string{
			"return sheinworkspace.BuildResolutionCacheSummary(pkg)",
		})
		assertSourceExcludesAll(t, source, []string{
			"enrichCategoryResolutionCacheInfo(",
			"enrichPricingResolutionCacheInfo(",
			"CloneResolutionCacheInfo(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"BuildResolutionCacheSummary",
		})
	})

	t.Run("image upload preflight delegates aggregation to workspace", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "preview_builder_shein_image_upload.go", "buildSheinImageUploadPreflight")
		callNames := readNamedFunctionCallNames(t, "preview_builder_shein_image_upload.go", "buildSheinImageUploadPreflight")

		assertSourceContainsAll(t, source, []string{
			"return sheinworkspace.BuildImageUploadPreflight(",
			"isSheinUploadedImageURL,",
			"sheinImageUploadCacheHit,",
			"isSDSImageURL,",
		})
		assertSourceExcludesAll(t, source, []string{
			"collectSheinProductImageURLs(pkg.PreviewPayload)",
			"buildSheinImageUploadPreflightSummary(report)",
			"report.PendingUploadURLs++",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"BuildImageUploadPreflight",
		})
	})
}
