package listingkit

import "task-processor/internal/asset"

func buildAmazonPreviewPayload(pkg *AmazonPackage, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *AmazonPreviewPayload {
	if pkg == nil || pkg.Draft == nil {
		return nil
	}
	return &AmazonPreviewPayload{
		Title:          pkg.Draft.Title,
		Brand:          pkg.Draft.Brand,
		ProductType:    pkg.Draft.ProductType,
		ImageBundle:    pkg.ImageBundle,
		RenderPreviews: renderPreviews,
		ScenePresets:   buildPlatformScenePresetSummaries(pkg.ImageBundle, assetBundle),
		Draft:          pkg.Draft,
	}
}

func buildTemuPreviewPayload(pkg *TemuPackage, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *TemuPreviewPayload {
	if pkg == nil {
		return nil
	}
	return &TemuPreviewPayload{
		Headline:       pkg.GoodsName,
		NeedsReview:    len(pkg.ReviewNotes) > 0,
		ReviewNotes:    append([]string(nil), pkg.ReviewNotes...),
		ImageBundle:    pkg.ImageBundle,
		RenderPreviews: renderPreviews,
		ScenePresets:   buildPlatformScenePresetSummaries(pkg.ImageBundle, assetBundle),
		Package:        pkg,
	}
}

func buildWalmartPreviewPayload(pkg *WalmartPackage, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *WalmartPreviewPayload {
	if pkg == nil {
		return nil
	}
	return &WalmartPreviewPayload{
		Headline:       pkg.ProductName,
		NeedsReview:    len(pkg.ReviewNotes) > 0,
		ReviewNotes:    append([]string(nil), pkg.ReviewNotes...),
		ImageBundle:    pkg.ImageBundle,
		RenderPreviews: renderPreviews,
		ScenePresets:   buildPlatformScenePresetSummaries(pkg.ImageBundle, assetBundle),
		Package:        pkg,
	}
}
