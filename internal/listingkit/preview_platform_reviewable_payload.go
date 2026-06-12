package listingkit

import common "task-processor/internal/publishing/common"

type reviewablePlatformPreviewPayloadBase struct {
	headline       string
	needsReview    bool
	reviewNotes    []string
	imageBundle    *common.PublishImageBundle
	renderPreviews *PlatformAssetRenderPreviews
	scenePresets   []PlatformScenePresetSummary
}

func buildReviewablePlatformPreviewPayloadBase(headline string, base reviewablePlatformPreviewBase) reviewablePlatformPreviewPayloadBase {
	return reviewablePlatformPreviewPayloadBase{
		headline:       headline,
		needsReview:    base.needsReview,
		reviewNotes:    base.reviewNotes,
		imageBundle:    base.imageBundle,
		renderPreviews: base.renderPreviews,
		scenePresets:   base.scenePresets,
	}
}
