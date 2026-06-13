package listingkit

type platformVisualPreviewPayloadBase = platformVisualPresentationBase

func buildPlatformVisualPreviewPayloadBase(base platformVisualPreviewBase) platformVisualPreviewPayloadBase {
	return newPlatformVisualPresentationBase(base.imageBundle, base.renderPreviews, base.scenePresets)
}
