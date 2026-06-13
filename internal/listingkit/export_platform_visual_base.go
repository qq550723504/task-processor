package listingkit

import (
	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

type platformVisualExportBase struct {
	imageBundle    *common.PublishImageBundle
	renderPreviews *PlatformAssetRenderPreviews
	scenePresets   []PlatformScenePresetSummary
}

func buildPlatformVisualExportBase(
	platform string,
	imageBundle *common.PublishImageBundle,
	assetBundle *asset.Bundle,
	platformPreviews []PlatformAssetRenderPreviews,
) platformVisualExportBase {
	return platformVisualExportBase{
		imageBundle:    imageBundle,
		renderPreviews: platformAssetRenderPreviewsByPlatform(platformPreviews, platform),
		scenePresets:   buildPlatformScenePresetSummaries(imageBundle, assetBundle),
	}
}
