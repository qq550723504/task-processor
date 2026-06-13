package listingkit

func buildAmazonPreviewPayloadBody(input amazonPreviewPayloadInput) *AmazonPreviewPayload {
	if input.draft == nil {
		return nil
	}
	return &AmazonPreviewPayload{
		Title:          input.draft.Title,
		Brand:          input.draft.Brand,
		ProductType:    input.draft.ProductType,
		ImageBundle:    input.visualBase.imageBundle,
		RenderPreviews: input.visualBase.renderPreviews,
		ScenePresets:   input.visualBase.scenePresets,
		Draft:          input.draft,
	}
}
