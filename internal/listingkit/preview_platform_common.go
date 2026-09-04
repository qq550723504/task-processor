package listingkit

import common "task-processor/internal/publishing/common"

type reviewablePlatformPreviewBase struct {
	platformVisualPresentationBase
	needsReview bool
	reviewNotes []string
}

func buildReviewablePlatformPreviewBase(
	reviewNotes []string,
	imageBundle *common.PublishImageBundle,
) reviewablePlatformPreviewBase {
	return reviewablePlatformPreviewBase{
		platformVisualPresentationBase: buildPlatformVisualPresentationBase(imageBundle),
		needsReview:                    len(reviewNotes) > 0,
		reviewNotes:                    append([]string(nil), reviewNotes...),
	}
}
