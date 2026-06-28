package listingkit

func applyPreviewPlatformSection(selectedPlatform, platform string, available bool, build func()) error {
	return applyPlatformSection(selectedPlatform, platform, available, build)
}

func applyReviewablePreviewPlatformSection(selectedPlatform, platform string, available bool, preview *ListingKitPreview, build func() bool) error {
	return applyPlatformSection(selectedPlatform, platform, available, func() {
		needsReview := build()
		if preview != nil {
			preview.NeedsReview = preview.NeedsReview || needsReview
		}
	})
}
