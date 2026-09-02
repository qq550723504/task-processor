package listingkit

func buildTemuPreviewSection(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "temu"
	return applyReviewablePreviewPlatformSection(selectedPlatform, platform, result != nil && result.Temu != nil, preview, func() bool {
		preview.Temu = buildTemuPreviewPayloadFromResult(result)
		return preview.Temu != nil && preview.Temu.NeedsReview
	})
}

func buildTemuPreviewPayload(pkg *TemuPackage) *TemuPreviewPayload {
	if pkg == nil {
		return nil
	}
	return buildTemuPreviewPayloadFromInput(
		buildReviewablePlatformPreviewPayloadInput(
			pkg.GoodsName,
			pkg.ReviewNotes,
			pkg.ImageBundle,
		),
		pkg,
	)
}
