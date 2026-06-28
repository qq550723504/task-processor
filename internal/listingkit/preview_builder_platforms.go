package listingkit

import listingplatform "task-processor/internal/listing/platform"

func previewPlatformBuilders() []listingplatform.RegisteredSectionBuilder[*ListingKitResult, *ListingKitPreview] {
	return listingplatform.SectionBuilders(previewPlatformRegistrations())
}

func buildPreviewPlatformSections(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error {
	return listingplatform.BuildRegisteredSections(previewPlatformBuilders(), result, preview, selectedPlatform)
}
