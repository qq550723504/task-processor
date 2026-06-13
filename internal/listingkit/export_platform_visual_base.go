package listingkit

import (
	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

type platformVisualExportBase = platformVisualPresentationBase

func buildPlatformVisualExportBase(
	platform string,
	imageBundle *common.PublishImageBundle,
	assetBundle *asset.Bundle,
	platformPreviews []PlatformAssetRenderPreviews,
) platformVisualExportBase {
	return newPlatformVisualPresentationBase(
		imageBundle,
		platformAssetRenderPreviewsByPlatform(platformPreviews, platform),
		buildPlatformScenePresetSummaries(imageBundle, assetBundle),
	)
}
