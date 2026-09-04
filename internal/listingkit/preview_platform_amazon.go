package listingkit

func buildAmazonPreviewSection(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "amazon"
	return applyPreviewPlatformSection(selectedPlatform, platform, result != nil && result.Amazon != nil, func() {
		preview.Amazon = buildAmazonPreviewPayloadFromResult(result)
	})
}

func buildAmazonPreviewPayload(pkg *AmazonPackage) *AmazonPreviewPayload {
	if pkg == nil {
		return nil
	}
	return buildAmazonPreviewPayloadFromInput(amazonPreviewPayloadInput{
		draft:      pkg.Draft,
		visualBase: buildPlatformVisualPreviewPayloadInput(pkg.ImageBundle),
	})
}
