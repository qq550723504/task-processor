package listingkit

import listingplatform "task-processor/internal/listing/platform"

func exportPlatformBuilders() []listingplatform.RegisteredSectionBuilder[*ListingKitResult, *ListingKitExport] {
	return listingplatform.SectionBuilders(exportPlatformRegistrations())
}

func buildExportPlatformSections(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {
	return listingplatform.BuildRegisteredSections(exportPlatformBuilders(), result, export, selectedPlatform)
}

func buildAmazonExportSection(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {
	return applyExportPlatformSection(selectedPlatform, "amazon", result != nil && result.Amazon != nil, func() {
		export.Amazon = buildAmazonExportPayloadFromResult(result, export.PlatformAssetRenderPreviews)
	})
}

func buildSheinExportSection(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {
	return applyExportPlatformSection(selectedPlatform, "shein", result != nil && result.Shein != nil, func() {
		export.Shein = buildSheinExportPayloadFromResult(result, export.PlatformAssetRenderPreviews)
	})
}

func buildTemuExportSection(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {
	return applyExportPlatformSection(selectedPlatform, "temu", result != nil && result.Temu != nil, func() {
		export.Temu = buildTemuExportPayloadFromResult(result, export.PlatformAssetRenderPreviews)
	})
}

func buildWalmartExportSection(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {
	return applyExportPlatformSection(selectedPlatform, "walmart", result != nil && result.Walmart != nil, func() {
		export.Walmart = buildWalmartExportPayloadFromResult(result, export.PlatformAssetRenderPreviews)
	})
}
