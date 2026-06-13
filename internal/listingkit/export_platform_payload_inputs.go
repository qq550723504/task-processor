package listingkit

import (
	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

func buildPlatformVisualExportPayloadInput(
	platform string,
	imageBundle *common.PublishImageBundle,
	assetBundle *asset.Bundle,
	platformPreviews []PlatformAssetRenderPreviews,
) platformVisualExportBase {
	return buildPlatformVisualExportBase(platform, imageBundle, assetBundle, platformPreviews)
}

func buildReviewablePlatformExportPayloadInput(
	platform string,
	imageBundle *common.PublishImageBundle,
	assetBundle *asset.Bundle,
	platformPreviews []PlatformAssetRenderPreviews,
) reviewableExportPayloadInput {
	return reviewableExportPayloadInput{
		visualBase: buildPlatformVisualExportPayloadInput(platform, imageBundle, assetBundle, platformPreviews),
	}
}
