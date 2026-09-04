package listingkit

import "task-processor/internal/product/catalog/canonical"

func buildAmazonPreviewPayloadInputFromResult(
	result *ListingKitResult,
) (amazonPreviewPayloadInput, bool) {
	if result == nil || result.Amazon == nil {
		return amazonPreviewPayloadInput{}, false
	}
	return amazonPreviewPayloadInput{
		draft:      result.Amazon.Draft,
		visualBase: buildPlatformVisualPreviewPayloadInput(result.Amazon.ImageBundle),
	}, true
}

func buildSheinPreviewPayloadInputFromResult(
	result *ListingKitResult,
) (sheinPreviewPayloadInput, bool) {
	if result == nil || result.Shein == nil {
		return sheinPreviewPayloadInput{}, false
	}
	return buildSheinPreviewPayloadInput(
		result.Shein,
		result.PodExecution,
		canonicalProductForResultTarget(result, "shein"),
	), true
}

func canonicalProductForResultTarget(result *ListingKitResult, target string) *canonical.Product {
	if result == nil {
		return nil
	}
	if result.StandardProductSnapshot != nil {
		if targetProduct := canonicalProductFromStandardSnapshotForTarget(result.StandardProductSnapshot, target); targetProduct != nil {
			return targetProduct
		}
	}
	return result.CanonicalProduct
}

func buildTemuPreviewPayloadInputFromResult(
	result *ListingKitResult,
) (reviewablePlatformPreviewPayloadInput, *TemuPackage, bool) {
	if result == nil || result.Temu == nil {
		return reviewablePlatformPreviewPayloadInput{}, nil, false
	}
	return buildReviewablePlatformPreviewPayloadInput(
		result.Temu.GoodsName,
		result.Temu.ReviewNotes,
		result.Temu.ImageBundle,
	), result.Temu, true
}

func buildWalmartPreviewPayloadInputFromResult(
	result *ListingKitResult,
) (reviewablePlatformPreviewPayloadInput, *WalmartPackage, bool) {
	if result == nil || result.Walmart == nil {
		return reviewablePlatformPreviewPayloadInput{}, nil, false
	}
	return buildReviewablePlatformPreviewPayloadInput(
		result.Walmart.ProductName,
		result.Walmart.ReviewNotes,
		result.Walmart.ImageBundle,
	), result.Walmart, true
}
