package listingkit

import (
	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

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

func buildPlatformVisualPresentationBase(
	imageBundle *common.PublishImageBundle,
	assetBundle *asset.Bundle,
	renderPreviews *PlatformAssetRenderPreviews,
) platformVisualPresentationBase {
	return newPlatformVisualPresentationBase(
		imageBundle,
		renderPreviews,
		buildPlatformScenePresetSummaries(imageBundle, assetBundle),
	)
}

func buildPlatformVisualPresentationBaseForPlatform(
	platform string,
	imageBundle *common.PublishImageBundle,
	assetBundle *asset.Bundle,
	platformPreviews []PlatformAssetRenderPreviews,
) platformVisualPresentationBase {
	return buildPlatformVisualPresentationBase(
		imageBundle,
		assetBundle,
		platformAssetRenderPreviewsByPlatform(platformPreviews, platform),
	)
}
