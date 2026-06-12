package listingkit

import common "task-processor/internal/publishing/common"

type platformVisualPreviewPayloadBase struct {
	imageBundle    *common.PublishImageBundle
	renderPreviews *PlatformAssetRenderPreviews
	scenePresets   []PlatformScenePresetSummary
}

func buildPlatformVisualPreviewPayloadBase(base platformVisualPreviewBase) platformVisualPreviewPayloadBase {
	return platformVisualPreviewPayloadBase{
		imageBundle:    base.imageBundle,
		renderPreviews: base.renderPreviews,
		scenePresets:   base.scenePresets,
	}
}
