package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func buildAmazonExportPayloadInputFromResult(
	result *ListingKitResult,
	platformPreviews []PlatformAssetRenderPreviews,
) (amazonExportPayloadInput, bool) {
	if result == nil || result.Amazon == nil {
		return amazonExportPayloadInput{}, false
	}
	context := buildPlatformPayloadResultContext(result, platformPreviews)
	return amazonExportPayloadInput{
		draft:      result.Amazon.Draft,
		visualBase: context.exportVisualBase("amazon", result.Amazon.ImageBundle),
	}, true
}

func buildSheinExportPayloadFromResultInput(
	result *ListingKitResult,
	platformPreviews []PlatformAssetRenderPreviews,
) (*SheinExportPayload, bool) {
	if result == nil || result.Shein == nil {
		return nil, false
	}
	sheinpub.NormalizePackageSemanticFields(result.Shein)
	context := buildPlatformPayloadResultContext(result, platformPreviews)
	visualBase := context.exportVisualBase("shein", result.Shein.ImageBundle)
	return normalizeSheinExportPayloadSemanticFields(&SheinExportPayload{
		Inspection:     result.Shein.Inspection,
		ImageBundle:    visualBase.imageBundle,
		RenderPreviews: visualBase.renderPreviews,
		ScenePresets:   visualBase.scenePresets,
		DraftPayload:   result.Shein.DraftPayload,
		PreviewPayload: result.Shein.PreviewPayload,
		ReviewNotes:    append([]string(nil), result.Shein.ReviewNotes...),
	}), true
}

func buildTemuExportPayloadInputFromResult(
	result *ListingKitResult,
	platformPreviews []PlatformAssetRenderPreviews,
) (reviewableExportPayloadInput, *TemuPackage, bool) {
	if result == nil || result.Temu == nil {
		return reviewableExportPayloadInput{}, nil, false
	}
	context := buildPlatformPayloadResultContext(result, platformPreviews)
	return buildReviewablePlatformExportPayloadInput("temu", result.Temu.ImageBundle, context.assetBundle, context.platformPreviews), result.Temu, true
}

func buildWalmartExportPayloadInputFromResult(
	result *ListingKitResult,
	platformPreviews []PlatformAssetRenderPreviews,
) (reviewableExportPayloadInput, *WalmartPackage, bool) {
	if result == nil || result.Walmart == nil {
		return reviewableExportPayloadInput{}, nil, false
	}
	context := buildPlatformPayloadResultContext(result, platformPreviews)
	return buildReviewablePlatformExportPayloadInput("walmart", result.Walmart.ImageBundle, context.assetBundle, context.platformPreviews), result.Walmart, true
}
