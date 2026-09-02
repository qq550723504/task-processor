package listingkit

func buildAmazonPreviewPayloadFromResult(result *ListingKitResult) *AmazonPreviewPayload {
	input, ok := buildAmazonPreviewPayloadInputFromResult(result)
	if !ok {
		return nil
	}
	return buildAmazonPreviewPayloadFromInput(input)
}

func buildSheinPreviewPayloadFromResult(result *ListingKitResult) *SheinPreviewPayload {
	input, ok := buildSheinPreviewPayloadInputFromResult(result)
	if !ok {
		return nil
	}
	return buildSheinPreviewPayloadFromInput(input)
}

func buildTemuPreviewPayloadFromResult(result *ListingKitResult) *TemuPreviewPayload {
	input, pkg, ok := buildTemuPreviewPayloadInputFromResult(result)
	if !ok {
		return nil
	}
	return buildTemuPreviewPayloadFromInput(input, pkg)
}

func buildWalmartPreviewPayloadFromResult(result *ListingKitResult) *WalmartPreviewPayload {
	input, pkg, ok := buildWalmartPreviewPayloadInputFromResult(result)
	if !ok {
		return nil
	}
	return buildWalmartPreviewPayloadFromInput(input, pkg)
}
