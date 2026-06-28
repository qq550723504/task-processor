package listingkit

func previewPlatformBuilders() []platformSectionBuilder[*ListingKitPreview] {
	return platformSectionBuilders(previewPlatformRegistrations())
}

func buildPreviewPlatformSections(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error {
	return buildPlatformSections(previewPlatformBuilders(), result, preview, selectedPlatform)
}
