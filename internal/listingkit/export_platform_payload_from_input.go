package listingkit

func buildAmazonExportPayloadFromInput(input amazonExportPayloadInput) *AmazonExportPayload {
	if input.draft == nil {
		return nil
	}
	return &AmazonExportPayload{
		Draft:          input.draft,
		ImageBundle:    input.visualBase.imageBundle,
		RenderPreviews: input.visualBase.renderPreviews,
		ScenePresets:   input.visualBase.scenePresets,
	}
}

func buildTemuExportPayloadFromInput(input reviewableExportPayloadInput, pkg *TemuPackage) *TemuExportPayload {
	if pkg == nil {
		return nil
	}
	return &TemuExportPayload{
		ImageBundle:    input.visualBase.imageBundle,
		RenderPreviews: input.visualBase.renderPreviews,
		ScenePresets:   input.visualBase.scenePresets,
		Package:        pkg,
	}
}

func buildWalmartExportPayloadFromInput(input reviewableExportPayloadInput, pkg *WalmartPackage) *WalmartExportPayload {
	if pkg == nil {
		return nil
	}
	return &WalmartExportPayload{
		ImageBundle:    input.visualBase.imageBundle,
		RenderPreviews: input.visualBase.renderPreviews,
		ScenePresets:   input.visualBase.scenePresets,
		Package:        pkg,
	}
}
