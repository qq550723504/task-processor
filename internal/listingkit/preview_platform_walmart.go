package listingkit

import "task-processor/internal/asset"

func buildWalmartPreviewSection(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "walmart"
	return applyReviewablePreviewPlatformSection(selectedPlatform, platform, task.Result.Walmart != nil, preview, func() bool {
		preview.Walmart = buildWalmartPreviewPayload(
			task.Result.Walmart,
			task.Result.AssetBundle,
			platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, platform),
		)
		return preview.Walmart != nil && preview.Walmart.NeedsReview
	})
}

func buildWalmartPreviewPayload(pkg *WalmartPackage, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *WalmartPreviewPayload {
	if pkg == nil {
		return nil
	}
	return buildWalmartPreviewPayloadBody(
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
