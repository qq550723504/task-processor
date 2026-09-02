package listingkit

func buildAmazonExportPayloadFromResult(result *ListingKitResult) *AmazonExportPayload {
	input, ok := buildAmazonExportPayloadInputFromResult(result)
	if !ok {
		return nil
	}
	return buildAmazonExportPayloadFromInput(input)
}

func buildSheinExportPayloadFromResult(result *ListingKitResult) *SheinExportPayload {
	payload, ok := buildSheinExportPayloadFromResultInput(result)
	if !ok {
		return nil
	}
	return payload
}

func buildTemuExportPayloadFromResult(result *ListingKitResult) *TemuExportPayload {
	input, pkg, ok := buildTemuExportPayloadInputFromResult(result)
	if !ok {
		return nil
	}
	return buildTemuExportPayloadFromInput(input, pkg)
}

func buildWalmartExportPayloadFromResult(result *ListingKitResult) *WalmartExportPayload {
	input, pkg, ok := buildWalmartExportPayloadInputFromResult(result)
	if !ok {
		return nil
	}
	return buildWalmartExportPayloadFromInput(input, pkg)
}
