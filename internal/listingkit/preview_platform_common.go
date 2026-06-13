package listingkit

import (
	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

type reviewablePlatformPreviewBase struct {
	platformVisualPresentationBase
	needsReview bool
	reviewNotes []string
}

func buildReviewablePlatformPreviewBase(
	reviewNotes []string,
	imageBundle *common.PublishImageBundle,
	assetBundle *asset.Bundle,
	renderPreviews *PlatformAssetRenderPreviews,
) reviewablePlatformPreviewBase {
	return reviewablePlatformPreviewBase{
		platformVisualPresentationBase: buildPlatformVisualPresentationBase(imageBundle, assetBundle, renderPreviews),
		needsReview:                    len(reviewNotes) > 0,
		reviewNotes:                    append([]string(nil), reviewNotes...),
	}
}
