package listingkit

func buildSheinPreviewSection(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "shein"
	return buildPreviewPlatformSection(selectedPlatform, platform, task.Result.Shein != nil, func() {
		preview.Shein = buildSheinPreviewPayload(
			task.Result.Shein,
			task.Result.PodExecution,
			task.Result.CanonicalProduct,
			task.Result.AssetBundle,
			platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, platform),
		)
		preview.NeedsReview = preview.NeedsReview || preview.Shein.NeedsReview
	})
}
