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

	t.Run("preview_input_reuses_read_projection_platform_cards", func(t *testing.T) {
		t.Parallel()

		readSource := readNamedFunctionSource(t, "read_projection.go", "buildListingKitReadProjection")
		inputSource := readNamedFunctionSource(t, "read_projection_preview_input.go", "buildListingKitPreviewReadModelInput")
		headerSource := readNamedFunctionSource(t, "read_projection_preview_input.go", "buildListingKitPreviewHeaderInput")

		assertSourceContainsAll(t, readSource, []string{
			"platformCards := buildPlatformPreviewCards(result, selectedPlatform)",
			"previewInput := buildListingKitPreviewReadModelInput(result, platformCards)",
		})
		assertSourceExcludesAll(t, readSource, []string{
			"previewInput := buildListingKitPreviewReadModelInput(result, selectedPlatform)",
		})
		assertSourceContainsAll(t, inputSource, []string{
			"func buildListingKitPreviewReadModelInput(result *ListingKitResult, platformCards []ListingKitPlatformCard) previewdomain.ReadModelInput",
			"Overview:    buildListingKitPreviewHeaderInput(result, platformCards)",
		})
		assertSourceContainsAll(t, headerSource, []string{
			"func buildListingKitPreviewHeaderInput(result *ListingKitResult, platformCards []ListingKitPlatformCard) *previewdomain.HeaderInput",
		})
		assertSourceExcludesAll(t, headerSource, []string{
			"platformCards := buildPlatformPreviewCards(result, selectedPlatform)",
		})
	})

	t.Run("preview_header_uses_platform_card_adapter", func(t *testing.T) {
		t.Parallel()

		headerSource := readNamedFunctionSource(t, "read_projection_preview_input.go", "buildListingKitPreviewHeaderInput")
		adapterSource := readNamedFunctionSource(t, "read_projection_preview_input.go", "buildPreviewDomainPlatformCards")

		assertSourceContainsAll(t, headerSource, []string{
			"input.PlatformCards = buildPreviewDomainPlatformCards(platformCards)",
		})
		assertSourceExcludesAll(t, headerSource, []string{
			"make([]previewdomain.PlatformCard, 0, len(platformCards))",
			"input.PlatformCards = append(input.PlatformCards, previewdomain.PlatformCard{",
		})
		assertSourceContainsAll(t, adapterSource, []string{
			"func buildPreviewDomainPlatformCards(platformCards []ListingKitPlatformCard) []previewdomain.PlatformCard",
			"cards := make([]previewdomain.PlatformCard, 0, len(platformCards))",
			"cards = append(cards, previewdomain.PlatformCard{",
			"PrimaryActionKey:      card.PrimaryActionKey",
			"PrimaryCTAKind:        card.PrimaryCTAKind",
		})
	})
}
