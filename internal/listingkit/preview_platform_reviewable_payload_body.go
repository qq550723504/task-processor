package listingkit

func buildTemuPreviewPayloadBody(input reviewablePlatformPreviewPayloadInput, pkg *TemuPackage) *TemuPreviewPayload {
	if pkg == nil {
		return nil
	}
	return &TemuPreviewPayload{
		Headline:       input.base.headline,
		NeedsReview:    input.base.needsReview,
		ReviewNotes:    input.base.reviewNotes,
		ImageBundle:    input.base.imageBundle,
		RenderPreviews: input.base.renderPreviews,
		ScenePresets:   input.base.scenePresets,
		Package:        pkg,
	}
}

func buildWalmartPreviewPayloadBody(input reviewablePlatformPreviewPayloadInput, pkg *WalmartPackage) *WalmartPreviewPayload {
	if pkg == nil {
		return nil
	}
	return &WalmartPreviewPayload{
		Headline:       input.base.headline,
		NeedsReview:    input.base.needsReview,
		ReviewNotes:    input.base.reviewNotes,
		ImageBundle:    input.base.imageBundle,
		RenderPreviews: input.base.renderPreviews,
		ScenePresets:   input.base.scenePresets,
		Package:        pkg,
	}
}
