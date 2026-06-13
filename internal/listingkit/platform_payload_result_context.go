package listingkit

import (
	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

type platformPayloadResultContext struct {
	assetBundle      *asset.Bundle
	platformPreviews []PlatformAssetRenderPreviews
}

func buildPlatformPayloadResultContext(result *ListingKitResult, platformPreviews []PlatformAssetRenderPreviews) platformPayloadResultContext {
	if result == nil {
		return platformPayloadResultContext{
			platformPreviews: append([]PlatformAssetRenderPreviews(nil), platformPreviews...),
		}
	}
	return platformPayloadResultContext{
		assetBundle:      result.AssetBundle,
		platformPreviews: append([]PlatformAssetRenderPreviews(nil), platformPreviews...),
	}
}

func (c platformPayloadResultContext) previewVisualBase(
	platform string,
	imageBundle *common.PublishImageBundle,
) platformVisualPreviewPayloadBase {
	return buildPlatformVisualPreviewPayloadInput(
		imageBundle,
		c.assetBundle,
		platformAssetRenderPreviewsByPlatform(c.platformPreviews, platform),
	)
}

func (c platformPayloadResultContext) exportVisualBase(
	platform string,
	imageBundle *common.PublishImageBundle,
) platformVisualExportBase {
	return buildPlatformVisualExportPayloadInput(
		platform,
		imageBundle,
		c.assetBundle,
		c.platformPreviews,
	)
}

func (c platformPayloadResultContext) previewRenderPreviews(platform string) *PlatformAssetRenderPreviews {
	return platformAssetRenderPreviewsByPlatform(c.platformPreviews, platform)
}
