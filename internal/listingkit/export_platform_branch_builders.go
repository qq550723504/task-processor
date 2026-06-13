package listingkit

func buildAmazonExportPayloadFromResult(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) *AmazonExportPayload {
	if result == nil || result.Amazon == nil {
		return nil
	}
	return buildAmazonExportPayload(amazonExportPayloadInput{
		draft:      result.Amazon.Draft,
		visualBase: buildPlatformVisualExportPayloadInput("amazon", result.Amazon.ImageBundle, result.AssetBundle, platformPreviews),
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
