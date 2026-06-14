package generation

import previewdomain "task-processor/internal/listing/preview"

func RenderPreviewCapabilities(layerTypes []string) []string {
	return previewdomain.RenderPreviewCapabilities(layerTypes)
}

func RenderPreviewCapabilitiesForSlot(layerTypes []string, previewSVG, assetURL string) []string {
	return previewdomain.RenderPreviewCapabilitiesForSlot(layerTypes, previewSVG, assetURL)
}
