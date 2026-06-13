package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func buildAmazonExportPayloadFromResult(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) *AmazonExportPayload {
	if result == nil || result.Amazon == nil {
		return nil
	}
	return buildAmazonExportPayload(amazonExportPayloadInput{
		draft:      result.Amazon.Draft,
		visualBase: buildPlatformVisualExportPayloadInput("amazon", result.Amazon.ImageBundle, result.AssetBundle, platformPreviews),
	})
}

func buildSheinExportPayloadFromResult(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) *SheinExportPayload {
	if result == nil || result.Shein == nil {
		return nil
	}
	sheinpub.NormalizePackageSemanticFields(result.Shein)
	visualBase := buildPlatformVisualExportBase("shein", result.Shein.ImageBundle, result.AssetBundle, platformPreviews)
	return normalizeSheinExportPayloadSemanticFields(&SheinExportPayload{
		Inspection:     result.Shein.Inspection,
		ImageBundle:    visualBase.imageBundle,
		RenderPreviews: visualBase.renderPreviews,
		ScenePresets:   visualBase.scenePresets,
		DraftPayload:   result.Shein.DraftPayload,
		PreviewPayload: result.Shein.PreviewPayload,
		ReviewNotes:    append([]string(nil), result.Shein.ReviewNotes...),
	})
}

func buildTemuExportPayloadFromResult(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) *TemuExportPayload {
	if result == nil || result.Temu == nil {
		return nil
	}
	return buildTemuExportPayload(
		buildReviewablePlatformExportPayloadInput("temu", result.Temu.ImageBundle, result.AssetBundle, platformPreviews),
		result.Temu,
	)
}

func buildWalmartExportPayloadFromResult(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) *WalmartExportPayload {
	if result == nil || result.Walmart == nil {
		return nil
	}
	return buildWalmartExportPayload(
		buildReviewablePlatformExportPayloadInput("walmart", result.Walmart.ImageBundle, result.AssetBundle, platformPreviews),
		result.Walmart,
	)
}
