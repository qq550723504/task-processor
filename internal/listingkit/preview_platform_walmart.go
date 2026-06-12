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
	base := buildReviewablePlatformPreviewBase(pkg.ReviewNotes, pkg.ImageBundle, assetBundle, renderPreviews)
	payloadBase := buildReviewablePlatformPreviewPayloadBase(pkg.ProductName, base)
	return &WalmartPreviewPayload{
		Headline:       payloadBase.headline,
		NeedsReview:    payloadBase.needsReview,
		ReviewNotes:    payloadBase.reviewNotes,
		ImageBundle:    payloadBase.imageBundle,
		RenderPreviews: payloadBase.renderPreviews,
		ScenePresets:   payloadBase.scenePresets,
		Package:        pkg,
	}
}
