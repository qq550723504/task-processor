package listingkit

func buildAmazonExportSection(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {
	return applyExportPlatformSection(selectedPlatform, "amazon", result != nil && result.Amazon != nil, func() {
		export.Amazon = buildAmazonExportPayloadFromResult(result)
	})
}

func buildSheinExportSection(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {
	return applyExportPlatformSection(selectedPlatform, "shein", result != nil && result.Shein != nil, func() {
		export.Shein = buildSheinExportPayloadFromResult(result)
	})
}

func buildTemuExportSection(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {
	return applyExportPlatformSection(selectedPlatform, "temu", result != nil && result.Temu != nil, func() {
		export.Temu = buildTemuExportPayloadFromResult(result)
	})
}

func buildWalmartExportSection(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {
	return applyExportPlatformSection(selectedPlatform, "walmart", result != nil && result.Walmart != nil, func() {
		export.Walmart = buildWalmartExportPayloadFromResult(result)
	})
}
