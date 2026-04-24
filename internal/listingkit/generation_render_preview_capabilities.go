package listingkit

func buildRenderPreviewCapabilitiesForSlot(slot AssetRenderPreviewSlot) []string {
	item := GenerationWorkQueueItem{
		RenderPreviewLayerTypes: append([]string(nil), slot.LayerTypes...),
	}
	capabilities := buildRenderPreviewCapabilities(item)
	if len(capabilities) > 0 {
		return capabilities
	}
	if slot.PreviewSVG == "" && slot.AssetURL != "" {
		return []string{"subject_preview"}
	}
	return nil
}
