package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

func buildRenderPreviewCapabilities(item GenerationWorkQueueItem) []string {
	return listinggeneration.RenderPreviewCapabilities(item.RenderPreviewLayerTypes)
}

func buildRenderPreviewCapabilitiesForSlot(slot AssetRenderPreviewSlot) []string {
	return listinggeneration.RenderPreviewCapabilitiesForSlot(slot.LayerTypes, slot.PreviewSVG, slot.AssetURL)
}
