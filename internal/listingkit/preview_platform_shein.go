package listingkit

func buildSheinPreviewSection(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "shein"
	return applyReviewablePreviewPlatformSection(selectedPlatform, platform, result != nil && result.Shein != nil, preview, func() bool {
		preview.Shein = buildSheinPreviewPayloadFromResult(result, preview.PlatformAssetRenderPreviews)
		return preview.Shein != nil && preview.Shein.NeedsReview
	})
}
