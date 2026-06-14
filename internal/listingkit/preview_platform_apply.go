package listingkit

import previewdomain "task-processor/internal/listing/preview"

func applyPreviewPlatformSection(selectedPlatform, platform string, available bool, build func()) error {
	return adaptPreviewPlatformSectionError(previewdomain.BuildPlatformSection(selectedPlatform, platform, available, build))
}

func applyReviewablePreviewPlatformSection(selectedPlatform, platform string, available bool, preview *ListingKitPreview, build func() bool) error {
	return adaptPreviewPlatformSectionError(previewdomain.BuildPlatformSection(selectedPlatform, platform, available, func() {
		needsReview := build()
		if preview != nil {
			preview.NeedsReview = preview.NeedsReview || needsReview
		}
	}))
}

func adaptPreviewPlatformSectionError(err error) error {
	if err == previewdomain.ErrPlatformUnavailable {
		return ErrPreviewPlatformUnavailable
	}
	return err
}
