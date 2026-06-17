package listingkit

import "testing"

func TestListingKitReadProjectionBoundary(t *testing.T) {
	t.Parallel()

	t.Run("read_projection_assembly_passes_attachment_extras_as_one_bundle", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "read_projection.go", "buildListingKitReadProjection")
		callNames := readNamedFunctionCallNames(t, "read_projection.go", "buildListingKitReadProjection")

		assertSourceContainsAll(t, source, []string{
			"attachmentExtras := buildListingKitReadProjectionAttachmentExtras(result, selectedPlatform)",
			"attachmentExtras,",
		})
		assertSourceExcludesAll(t, source, []string{
			"assetRenderPreviews, platformRenderPreviews, generationQueue, generationOverview := buildListingKitReadProjectionAttachmentExtras(result, selectedPlatform)",
			"assetRenderPreviews,",
			"platformRenderPreviews,",
			"generationQueue,",
			"generationOverview,",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildListingKitReadProjectionAttachmentExtras",
			"assembleListingKitReadProjection",
		})
	})

	t.Run("attachment_extras_helper_returns_named_bundle", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "read_projection_preview_input.go", "buildListingKitReadProjectionAttachmentExtras")

		assertSourceContainsAll(t, source, []string{
			") listingKitReadProjectionAttachmentExtras {",
			"return listingKitReadProjectionAttachmentExtras{",
			"AssetRenderPreviews:         assetRenderPreviews",
			"PlatformAssetRenderPreviews: platformRenderPreviews",
			"AssetGenerationQueue:        result.AssetGenerationQueue",
			"AssetGenerationOverview:     result.AssetGenerationOverview",
		})
		assertSourceExcludesAll(t, source, []string{
			") ([]AssetRenderPreview, []PlatformAssetRenderPreviews, *GenerationWorkQueue, *AssetGenerationOverview) {",
			"return assetRenderPreviews, platformRenderPreviews, result.AssetGenerationQueue, result.AssetGenerationOverview",
		})
	})
}
