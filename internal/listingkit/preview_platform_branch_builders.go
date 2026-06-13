package listingkit

func buildAmazonPreviewPayloadFromResult(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) *AmazonPreviewPayload {
	input, ok := buildAmazonPreviewPayloadInputFromResult(result, platformPreviews)
	if !ok {
		return nil
	}
	return buildAmazonPreviewPayloadFromInput(input)
}

func buildSheinPreviewPayloadFromResult(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) *SheinPreviewPayload {
	input, ok := buildSheinPreviewPayloadInputFromResult(result, platformPreviews)
	if !ok {
		return nil
	}
	return buildSheinPreviewPayloadFromInput(input)
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
