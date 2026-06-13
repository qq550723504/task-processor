package listingkit

import (
	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

func buildPlatformVisualPreviewPayloadInput(
	imageBundle *common.PublishImageBundle,
	assetBundle *asset.Bundle,
	renderPreviews *PlatformAssetRenderPreviews,
) platformVisualPreviewPayloadBase {
	return buildPlatformVisualPresentationBase(imageBundle, assetBundle, renderPreviews)
}

func buildReviewablePlatformPreviewPayloadInput(
	headline string,
	reviewNotes []string,
	imageBundle *common.PublishImageBundle,
	assetBundle *asset.Bundle,
	renderPreviews *PlatformAssetRenderPreviews,
) reviewablePlatformPreviewPayloadInput {
	return reviewablePlatformPreviewPayloadInput{
		base: buildReviewablePlatformPreviewPayloadBase(
			headline,
			buildReviewablePlatformPreviewBase(reviewNotes, imageBundle, assetBundle, renderPreviews),
		),
	}
}
