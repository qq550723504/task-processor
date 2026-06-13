package listingkit

import "task-processor/internal/asset"

func buildTemuPreviewSection(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "temu"
	return applyReviewablePreviewPlatformSection(selectedPlatform, platform, result != nil && result.Temu != nil, preview, func() bool {
		preview.Temu = buildTemuPreviewPayloadFromResult(result, preview.PlatformAssetRenderPreviews)
		return preview.Temu != nil && preview.Temu.NeedsReview
	})
}

func buildTemuPreviewPayload(pkg *TemuPackage, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *TemuPreviewPayload {
	if pkg == nil {
		return nil
	}
	return buildTemuPreviewPayloadBody(
		buildReviewablePlatformPreviewPayloadInput(
			pkg.GoodsName,
			pkg.ReviewNotes,
			pkg.ImageBundle,
			assetBundle,
			renderPreviews,
		),
		pkg,
	)
}
