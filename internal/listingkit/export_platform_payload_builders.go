package listingkit

import amazonlisting "task-processor/internal/amazonlisting"

type amazonExportPayloadInput struct {
	draft      *amazonlisting.AmazonListingDraft
	visualBase platformVisualExportBase
}

func buildAmazonExportPayload(input amazonExportPayloadInput) *AmazonExportPayload {
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

type reviewableExportPayloadInput struct {
	visualBase platformVisualExportBase
}

func buildTemuExportPayload(input reviewableExportPayloadInput, pkg *TemuPackage) *TemuExportPayload {
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

func buildWalmartExportPayload(input reviewableExportPayloadInput, pkg *WalmartPackage) *WalmartExportPayload {
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
