package listingkit

func buildSheinPreviewSection(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "shein"
	return applyReviewablePreviewPlatformSection(selectedPlatform, platform, task.Result.Shein != nil, preview, func() bool {
		preview.Shein = buildSheinPreviewPayload(
			task.Result.Shein,
			task.Result.PodExecution,
			task.Result.CanonicalProduct,
			task.Result.AssetBundle,
			platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, platform),
		)
		return preview.Shein != nil && preview.Shein.NeedsReview
	})
}
