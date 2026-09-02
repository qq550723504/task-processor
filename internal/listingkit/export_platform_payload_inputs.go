package listingkit

import common "task-processor/internal/publishing/common"

func buildPlatformVisualExportPayloadInput(imageBundle *common.PublishImageBundle) platformVisualExportBase {
	return buildPlatformVisualPresentationBase(imageBundle)
}

func buildReviewablePlatformExportPayloadInput(
	imageBundle *common.PublishImageBundle,
) reviewableExportPayloadInput {
	return reviewableExportPayloadInput{
		visualBase: buildPlatformVisualExportPayloadInput(imageBundle),
	}
}
