package listingkit

func buildWalmartPreviewSection(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "walmart"
	return applyReviewablePreviewPlatformSection(selectedPlatform, platform, result != nil && result.Walmart != nil, preview, func() bool {
		preview.Walmart = buildWalmartPreviewPayloadFromResult(result)
		return preview.Walmart != nil && preview.Walmart.NeedsReview
	})
}

func buildWalmartPreviewPayload(pkg *WalmartPackage) *WalmartPreviewPayload {
	if pkg == nil {
		return nil
	}
	return buildWalmartPreviewPayloadFromInput(
		buildReviewablePlatformPreviewPayloadInput(
			pkg.ProductName,
			pkg.ReviewNotes,
			pkg.ImageBundle,
		),
		pkg,
	)
}
