package listingkit

func buildAmazonExportPayloadFromResult(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) *AmazonExportPayload {
	input, ok := buildAmazonExportPayloadInputFromResult(result, platformPreviews)
	if !ok {
		return nil
	}
	return buildAmazonExportPayloadFromInput(input)
}

func buildSheinExportPayloadFromResult(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) *SheinExportPayload {
	payload, ok := buildSheinExportPayloadFromResultInput(result, platformPreviews)
	if !ok {
		return nil
	}
	return payload
}

func buildTemuExportPayloadFromResult(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) *TemuExportPayload {
	input, pkg, ok := buildTemuExportPayloadInputFromResult(result, platformPreviews)
	if !ok {
		return nil
	}
	return buildTemuExportPayloadFromInput(input, pkg)
}

func buildWalmartExportPayloadFromResult(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) *WalmartExportPayload {
	input, pkg, ok := buildWalmartExportPayloadInputFromResult(result, platformPreviews)
	if !ok {
		return nil
	}
	return buildWalmartExportPayloadFromInput(input, pkg)
}
