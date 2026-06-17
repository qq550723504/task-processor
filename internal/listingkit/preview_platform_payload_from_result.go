package listingkit

func buildAmazonPreviewPayloadInputFromResult(
	result *ListingKitResult,
	platformPreviews []PlatformAssetRenderPreviews,
) (amazonPreviewPayloadInput, bool) {
	if result == nil || result.Amazon == nil {
		return amazonPreviewPayloadInput{}, false
	}
	context := buildPlatformPayloadResultContext(result, platformPreviews)
	return amazonPreviewPayloadInput{
		draft:      result.Amazon.Draft,
		visualBase: context.previewVisualBase("amazon", result.Amazon.ImageBundle),
	}, true
}

func buildSheinPreviewPayloadInputFromResult(
	result *ListingKitResult,
	platformPreviews []PlatformAssetRenderPreviews,
) (sheinPreviewPayloadInput, bool) {
	sheinContext, ok := buildSheinPlatformPayloadContext(result, platformPreviews)
	if !ok {
		return sheinPreviewPayloadInput{}, false
	}
	return buildSheinPreviewPayloadInput(
		sheinContext.pkg,
		result.PodExecution,
		result.CanonicalProduct,
		sheinContext.assetBundle,
		sheinContext.renderPreview,
	), true
}

func buildTemuPreviewPayloadInputFromResult(
	result *ListingKitResult,
	platformPreviews []PlatformAssetRenderPreviews,
) (reviewablePlatformPreviewPayloadInput, *TemuPackage, bool) {
	if result == nil || result.Temu == nil {
		return reviewablePlatformPreviewPayloadInput{}, nil, false
	}
	context := buildPlatformPayloadResultContext(result, platformPreviews)
	return buildReviewablePlatformPreviewPayloadInput(
		result.Temu.GoodsName,
		result.Temu.ReviewNotes,
		result.Temu.ImageBundle,
		context.assetBundle,
		context.previewRenderPreviews("temu"),
	), result.Temu, true
}

func buildWalmartPreviewPayloadInputFromResult(
	result *ListingKitResult,
	platformPreviews []PlatformAssetRenderPreviews,
) (reviewablePlatformPreviewPayloadInput, *WalmartPackage, bool) {
	if result == nil || result.Walmart == nil {
		return reviewablePlatformPreviewPayloadInput{}, nil, false
	}
	context := buildPlatformPayloadResultContext(result, platformPreviews)
	return buildReviewablePlatformPreviewPayloadInput(
		result.Walmart.ProductName,
		result.Walmart.ReviewNotes,
		result.Walmart.ImageBundle,
		context.assetBundle,
		context.previewRenderPreviews("walmart"),
	), result.Walmart, true
}
