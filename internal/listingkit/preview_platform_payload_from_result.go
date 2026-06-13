package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func buildAmazonPreviewPayloadInputFromResult(
	result *ListingKitResult,
	platformPreviews []PlatformAssetRenderPreviews,
) (amazonPreviewPayloadInput, bool) {
	if result == nil || result.Amazon == nil {
		return amazonPreviewPayloadInput{}, false
	}
	return amazonPreviewPayloadInput{
		draft: result.Amazon.Draft,
		visualBase: buildPlatformVisualPreviewPayloadInput(
			result.Amazon.ImageBundle,
			result.AssetBundle,
			platformAssetRenderPreviewsByPlatform(platformPreviews, "amazon"),
		),
	}, true
}

func buildSheinPreviewPayloadInputFromResult(
	result *ListingKitResult,
	platformPreviews []PlatformAssetRenderPreviews,
) (sheinPreviewPayloadInput, bool) {
	if result == nil || result.Shein == nil {
		return sheinPreviewPayloadInput{}, false
	}
	sheinpub.NormalizePackageSemanticFields(result.Shein)
	needsReview, summary := buildSheinPreviewReviewSummary(result.Shein)
	projection := buildSheinSubmitReadinessProjectionWithPod(result.Shein, result.PodExecution)
	readiness := projection.Readiness
	checklist := projection.Checklist
	repairCenter := buildSheinRepairCenter(readiness, checklist)
	submitState := projection.SubmitState
	statusOverview := projection.StatusOverview
	return sheinPreviewPayloadInput{
		pkg:               result.Shein,
		canonical:         result.CanonicalProduct,
		visualAssetBundle: result.AssetBundle,
		renderPreviews:    platformAssetRenderPreviewsByPlatform(platformPreviews, "shein"),
		needsReview:       needsReview,
		summary:           summary,
		readiness:         readiness,
		checklist:         checklist,
		repairCenter:      repairCenter,
		statusOverview:    statusOverview,
		workspaceOverview: buildSheinPreviewWorkspaceOverview(statusOverview, submitState, repairCenter),
	}, true
}
