package listingkit

func buildSheinPreviewSection(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "shein"
	return applyReviewablePreviewPlatformSection(selectedPlatform, platform, task.Result != nil && task.Result.Shein != nil, preview, func() bool {
		preview.Shein = buildSheinPreviewPayloadFromResult(task.Result, preview.PlatformAssetRenderPreviews)
		return preview.Shein != nil && preview.Shein.NeedsReview
	})
}
