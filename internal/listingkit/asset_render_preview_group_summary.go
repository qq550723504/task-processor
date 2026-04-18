package listingkit

func buildPlatformAssetRenderPreviewSummary(group PlatformAssetRenderPreviews) *PlatformAssetRenderPreviewSummary {
	slots := make([]AssetRenderPreviewSlot, 0, 1+len(group.Gallery)+len(group.Auxiliary))
	if group.Main != nil {
		slots = append(slots, *group.Main)
	}
	slots = append(slots, group.Gallery...)
	slots = append(slots, group.Auxiliary...)
	if len(slots) == 0 {
		return nil
	}
	summary := &PlatformAssetRenderPreviewSummary{
		TotalPreviews:  len(slots),
		MainAvailable:  group.Main != nil,
		GalleryCount:   len(group.Gallery),
		AuxiliaryCount: len(group.Auxiliary),
	}
	capabilityCounts := map[string]int{}
	visualModes := make([]string, 0, len(slots))
	for _, slot := range slots {
		item := GenerationWorkQueueItem{RenderPreviewLayerTypes: append([]string(nil), slot.LayerTypes...)}
		for _, capability := range buildRenderPreviewCapabilities(item) {
			capabilityCounts[capability]++
		}
		if slot.VisualMode != "" {
			visualModes = append(visualModes, slot.VisualMode)
		}
	}
	summary.CapabilityCounts = cloneStringIntMap(capabilityCounts)
	summary.VisualModes = uniqueStrings(visualModes)
	return summary
}
