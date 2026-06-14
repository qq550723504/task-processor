package listingkit

func buildListingKitReadProjection(result *ListingKitResult, selectedPlatform string) *listingKitReadProjection {
	if result == nil {
		return nil
	}

	previewInput := buildListingKitPreviewReadModelInput(result, selectedPlatform)
	platformCards := buildPlatformPreviewCards(result, selectedPlatform)
	assetRenderPreviews, platformRenderPreviews, generationQueue, generationOverview := buildListingKitReadProjectionAttachmentExtras(result, selectedPlatform)
	return assembleListingKitReadProjection(
		previewInput,
		platformCards,
		assetRenderPreviews,
		platformRenderPreviews,
		generationQueue,
		generationOverview,
		buildPreviewDomainRevisionHistoryMetaInput(result),
	)
}
