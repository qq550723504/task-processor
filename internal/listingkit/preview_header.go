package listingkit

func buildPreviewHeader(result *ListingKitResult, selectedPlatform string) *ListingKitPreviewHeader {
	projection := buildListingKitReadProjection(result, selectedPlatform)
	return buildPreviewHeaderFromReadProjection(projection)
}
