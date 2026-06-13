package listingkit

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
	sheinContext, ok := buildSheinPlatformPayloadContext(result, platformPreviews)
	if !ok {
		return nil, false
	}
	return normalizeSheinExportPayloadSemanticFields(&SheinExportPayload{
		Inspection:     sheinContext.pkg.Inspection,
		ImageBundle:    sheinContext.exportBase.imageBundle,
		RenderPreviews: sheinContext.exportBase.renderPreviews,
		ScenePresets:   sheinContext.exportBase.scenePresets,
		DraftPayload:   sheinContext.pkg.DraftPayload,
		PreviewPayload: sheinContext.pkg.PreviewPayload,
		ReviewNotes:    append([]string(nil), sheinContext.pkg.ReviewNotes...),
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
