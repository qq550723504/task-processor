package listingkit

func buildAmazonExportPayloadInputFromResult(
	result *ListingKitResult,
) (amazonExportPayloadInput, bool) {
	if result == nil || result.Amazon == nil {
		return amazonExportPayloadInput{}, false
	}
	return amazonExportPayloadInput{
		draft:      result.Amazon.Draft,
		visualBase: buildPlatformVisualExportPayloadInput(result.Amazon.ImageBundle),
	}, true
}

func buildSheinExportPayloadFromResultInput(
	result *ListingKitResult,
) (*SheinExportPayload, bool) {
	if result == nil || result.Shein == nil {
		return nil, false
	}
	return normalizeSheinExportPayloadSemanticFields(&SheinExportPayload{
		Inspection:     result.Shein.Inspection,
		ImageBundle:    result.Shein.ImageBundle,
		DraftPayload:   result.Shein.DraftPayload,
		PreviewPayload: result.Shein.PreviewPayload,
		ReviewNotes:    append([]string(nil), result.Shein.ReviewNotes...),
	}), true
}

func buildTemuExportPayloadInputFromResult(
	result *ListingKitResult,
) (reviewableExportPayloadInput, *TemuPackage, bool) {
	if result == nil || result.Temu == nil {
		return reviewableExportPayloadInput{}, nil, false
	}
	return buildReviewablePlatformExportPayloadInput(result.Temu.ImageBundle), result.Temu, true
}

func buildWalmartExportPayloadInputFromResult(
	result *ListingKitResult,
) (reviewableExportPayloadInput, *WalmartPackage, bool) {
	if result == nil || result.Walmart == nil {
		return reviewableExportPayloadInput{}, nil, false
	}
	return buildReviewablePlatformExportPayloadInput(result.Walmart.ImageBundle), result.Walmart, true
}
