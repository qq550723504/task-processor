package listingkit

import listingplatform "task-processor/internal/listing/platform"

func applyPreviewPlatformSection(selectedPlatform, platform string, available bool, build func()) error {
	return listingplatform.BuildOne(listingplatform.Section{
		SelectedPlatform: selectedPlatform,
		Platform:         platform,
		Available:        available,
		Build:            build,
		UnavailableError: ErrPreviewPlatformUnavailable,
	})
}

func applyReviewablePreviewPlatformSection(selectedPlatform, platform string, available bool, preview *ListingKitPreview, build func() bool) error {
	return listingplatform.BuildOne(listingplatform.Section{
		SelectedPlatform: selectedPlatform,
		Platform:         platform,
		Available:        available,
		Build: func() {
			needsReview := build()
			if preview != nil {
				preview.NeedsReview = preview.NeedsReview || needsReview
			}
		},
		UnavailableError: ErrPreviewPlatformUnavailable,
	})
}
