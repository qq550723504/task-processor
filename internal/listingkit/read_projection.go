package listingkit

func buildListingKitReadProjection(result *ListingKitResult, selectedPlatform string) *listingKitReadProjection {
	if result == nil {
		return nil
	}

	platformCards := buildPlatformPreviewCards(result, selectedPlatform)
	previewInput := buildListingKitPreviewReadModelInput(result, platformCards)
	attachmentExtras := buildListingKitReadProjectionAttachmentExtras(result, selectedPlatform)
	return assembleListingKitReadProjection(
		previewInput,
		platformCards,
		attachmentExtras,
		buildPreviewDomainRevisionHistoryMetaInput(result),
	)
}
