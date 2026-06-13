package listingkit

import "task-processor/internal/asset"

func buildWalmartPreviewSection(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "walmart"
	return applyReviewablePreviewPlatformSection(selectedPlatform, platform, result != nil && result.Walmart != nil, preview, func() bool {
		preview.Walmart = buildWalmartPreviewPayloadFromResult(result, preview.PlatformAssetRenderPreviews)
		return preview.Walmart != nil && preview.Walmart.NeedsReview
	})
}

func buildWalmartPreviewPayload(pkg *WalmartPackage, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *WalmartPreviewPayload {
	if pkg == nil {
		return nil
	}
	return buildWalmartPreviewPayloadFromInput(
		buildReviewablePlatformPreviewPayloadInput(
			pkg.ProductName,
			pkg.ReviewNotes,
			pkg.ImageBundle,
			assetBundle,
			renderPreviews,
		),
		pkg,
	)
}
