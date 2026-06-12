package listingkit

import "task-processor/internal/asset"

func buildWalmartPreviewSection(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "walmart"
	return buildPreviewPlatformSection(selectedPlatform, platform, task.Result.Walmart != nil, func() {
		preview.Walmart = buildWalmartPreviewPayload(
			task.Result.Walmart,
			task.Result.AssetBundle,
			platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, platform),
		)
		preview.NeedsReview = preview.NeedsReview || preview.Walmart.NeedsReview
	})
}

func buildWalmartPreviewPayload(pkg *WalmartPackage, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *WalmartPreviewPayload {
	if pkg == nil {
		return nil
	}
	base := buildReviewablePlatformPreviewBase(pkg.ReviewNotes, pkg.ImageBundle, assetBundle, renderPreviews)
	return &WalmartPreviewPayload{
		Headline:       pkg.ProductName,
		NeedsReview:    base.needsReview,
		ReviewNotes:    base.reviewNotes,
		ImageBundle:    base.imageBundle,
		RenderPreviews: base.renderPreviews,
		ScenePresets:   base.scenePresets,
		Package:        pkg,
	}
}
