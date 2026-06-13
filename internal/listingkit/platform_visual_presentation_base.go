package listingkit

import common "task-processor/internal/publishing/common"

type platformVisualPresentationBase struct {
	imageBundle    *common.PublishImageBundle
	renderPreviews *PlatformAssetRenderPreviews
	scenePresets   []PlatformScenePresetSummary
}

func newPlatformVisualPresentationBase(
	imageBundle *common.PublishImageBundle,
	renderPreviews *PlatformAssetRenderPreviews,
	scenePresets []PlatformScenePresetSummary,
) platformVisualPresentationBase {
	return platformVisualPresentationBase{
		imageBundle:    imageBundle,
		renderPreviews: renderPreviews,
		scenePresets:   scenePresets,
	}
}
