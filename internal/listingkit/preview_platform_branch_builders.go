package listingkit

func buildAmazonPreviewPayloadFromResult(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) *AmazonPreviewPayload {
	if result == nil || result.Amazon == nil {
		return nil
	}
	return buildAmazonPreviewPayload(
		result.Amazon,
		result.AssetBundle,
		platformAssetRenderPreviewsByPlatform(platformPreviews, "amazon"),
	)
}

func buildSheinPreviewPayloadFromResult(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) *SheinPreviewPayload {
	if result == nil || result.Shein == nil {
		return nil
	}
	return buildSheinPreviewPayload(
		result.Shein,
		result.PodExecution,
		result.CanonicalProduct,
		result.AssetBundle,
		platformAssetRenderPreviewsByPlatform(platformPreviews, "shein"),
	)
}

func buildTemuPreviewPayloadFromResult(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) *TemuPreviewPayload {
	if result == nil || result.Temu == nil {
		return nil
	}
	return buildTemuPreviewPayload(
		result.Temu,
		result.AssetBundle,
		platformAssetRenderPreviewsByPlatform(platformPreviews, "temu"),
	)
}

func buildWalmartPreviewPayloadFromResult(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) *WalmartPreviewPayload {
	if result == nil || result.Walmart == nil {
		return nil
	}
	return buildWalmartPreviewPayload(
		result.Walmart,
		result.AssetBundle,
		platformAssetRenderPreviewsByPlatform(platformPreviews, "walmart"),
	)
}
