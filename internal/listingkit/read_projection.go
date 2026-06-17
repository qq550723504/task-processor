package listingkit

func buildListingKitReadProjection(result *ListingKitResult, selectedPlatform string) *listingKitReadProjection {
	if result == nil {
		return nil
	}

	previewInput := buildListingKitPreviewReadModelInput(result, selectedPlatform)
	platformCards := buildPlatformPreviewCards(result, selectedPlatform)
	attachmentExtras := buildListingKitReadProjectionAttachmentExtras(result, selectedPlatform)
	return assembleListingKitReadProjection(
		previewInput,
		platformCards,
		attachmentExtras,
		buildPreviewDomainRevisionHistoryMetaInput(result),
	)
}
