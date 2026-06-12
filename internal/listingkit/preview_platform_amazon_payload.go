package listingkit

import amazonlisting "task-processor/internal/amazonlisting"

type amazonPreviewPayloadInput struct {
	draft      *amazonlisting.AmazonListingDraft
	visualBase platformVisualPreviewPayloadBase
}

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
