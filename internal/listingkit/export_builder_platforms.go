package listingkit

import listingplatform "task-processor/internal/listing/platform"

func exportPlatformBuilders() []listingplatform.RegisteredSectionBuilder[*ListingKitResult, *ListingKitExport] {
	return listingplatform.SectionBuilders(exportPlatformRegistrations())
}

func buildExportPlatformSections(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {
	return listingplatform.BuildRegisteredSections(exportPlatformBuilders(), result, export, selectedPlatform)
}
