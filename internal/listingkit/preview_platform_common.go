package listingkit

import (
	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

type platformVisualPreviewBase struct {
	imageBundle    *common.PublishImageBundle
	renderPreviews *PlatformAssetRenderPreviews
	scenePresets   []PlatformScenePresetSummary
}

type reviewablePlatformPreviewBase struct {
	platformVisualPreviewBase
	needsReview bool
	reviewNotes []string
}

func buildPlatformVisualPreviewBase(
	imageBundle *common.PublishImageBundle,
	assetBundle *asset.Bundle,
	renderPreviews *PlatformAssetRenderPreviews,
) platformVisualPreviewBase {
	return platformVisualPreviewBase{
		imageBundle:    imageBundle,
		renderPreviews: renderPreviews,
		scenePresets:   buildPlatformScenePresetSummaries(imageBundle, assetBundle),
	}
}

func buildReviewablePlatformPreviewBase(
	reviewNotes []string,
	imageBundle *common.PublishImageBundle,
	assetBundle *asset.Bundle,
	renderPreviews *PlatformAssetRenderPreviews,
) reviewablePlatformPreviewBase {
	base := buildPlatformVisualPreviewBase(imageBundle, assetBundle, renderPreviews)
	return reviewablePlatformPreviewBase{
		platformVisualPreviewBase: base,
		needsReview:               len(reviewNotes) > 0,
		reviewNotes:               append([]string(nil), reviewNotes...),
	}
}
