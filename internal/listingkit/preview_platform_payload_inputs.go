package listingkit

import common "task-processor/internal/publishing/common"

func buildPlatformVisualPreviewPayloadInput(
	imageBundle *common.PublishImageBundle,
) platformVisualPreviewPayloadBase {
	return buildPlatformVisualPresentationBase(imageBundle)
}

func buildReviewablePlatformPreviewPayloadInput(
	headline string,
	reviewNotes []string,
	imageBundle *common.PublishImageBundle,
) reviewablePlatformPreviewPayloadInput {
	return reviewablePlatformPreviewPayloadInput{
		base: buildReviewablePlatformPreviewPayloadBase(
			headline,
			buildReviewablePlatformPreviewBase(reviewNotes, imageBundle),
		),
	}
}
