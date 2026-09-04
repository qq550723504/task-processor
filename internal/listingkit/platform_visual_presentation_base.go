package listingkit

import common "task-processor/internal/publishing/common"

type platformVisualPresentationBase struct {
	imageBundle *common.PublishImageBundle
}

func newPlatformVisualPresentationBase(imageBundle *common.PublishImageBundle) platformVisualPresentationBase {
	return platformVisualPresentationBase{imageBundle: imageBundle}
}

func buildPlatformVisualPresentationBase(imageBundle *common.PublishImageBundle) platformVisualPresentationBase {
	return newPlatformVisualPresentationBase(imageBundle)
}
