package listingkit

import "task-processor/internal/asset"

func buildAmazonPreviewSection(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "amazon"
	return applyPreviewPlatformSection(selectedPlatform, platform, result != nil && result.Amazon != nil, func() {
		preview.Amazon = buildAmazonPreviewPayloadFromResult(result, preview.PlatformAssetRenderPreviews)
	})
}

func buildAmazonPreviewPayload(pkg *AmazonPackage, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *AmazonPreviewPayload {
	if pkg == nil {
		return nil
	}
	return buildAmazonPreviewPayloadFromInput(amazonPreviewPayloadInput{
		draft:      pkg.Draft,
		visualBase: buildPlatformVisualPreviewPayloadInput(pkg.ImageBundle, assetBundle, renderPreviews),
	})
}
