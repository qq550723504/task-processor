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
	input, pkg, ok := buildTemuPreviewPayloadInputFromResult(result, platformPreviews)
	if !ok {
		return nil
	}
	return buildTemuPreviewPayloadFromInput(input, pkg)
}

func buildWalmartPreviewPayloadFromResult(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) *WalmartPreviewPayload {
	input, pkg, ok := buildWalmartPreviewPayloadInputFromResult(result, platformPreviews)
	if !ok {
		return nil
	}
	return buildWalmartPreviewPayloadFromInput(input, pkg)
}
