package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

func buildRenderPreviewCapabilitiesForSlot(slot AssetRenderPreviewSlot) []string {
	return listinggeneration.RenderPreviewCapabilitiesForSlot(slot.LayerTypes, slot.PreviewSVG, slot.AssetURL)
}
