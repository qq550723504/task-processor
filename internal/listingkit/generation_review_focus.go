package listingkit

func buildGenerationReviewSessionFocus(previews []PlatformAssetRenderPreviews, platform, slot, capability string) (*GenerationReviewTarget, *AssetRenderPreviewSlot, string) {
	target := buildGenerationReviewTarget(platform, slot, capability)
	focusedPreview := findGenerationReviewPreview(previews, platform, slot, capability)
	focusedSectionKey := generationReviewSectionKey(capability)
	if focusedSectionKey == "" && focusedPreview != nil {
		focusedSectionKey = generationReviewSectionKey(firstRenderPreviewCapability(*focusedPreview))
	}
	if target != nil && focusedSectionKey == "" {
		focusedSectionKey = target.SectionKey
	}
	if target != nil && target.PanelState != nil && focusedPreview != nil {
		target.PanelState.FocusedPreviewAssetID = focusedPreview.AssetID
	}
	return target, focusedPreview, focusedSectionKey
}

func enrichGenerationReviewSlotsWithFocus(slots []GenerationReviewSlot, previews []PlatformAssetRenderPreviews, selectedCapability string) {
	for i := range slots {
		preview := findGenerationReviewPreview(previews, slots[i].Platform, slots[i].Slot, selectedCapability)
		if preview == nil {
			preview = findGenerationReviewPreview(previews, slots[i].Platform, slots[i].Slot, "")
		}
		if preview == nil {
			continue
		}
		slots[i].FocusCapability = selectedCapability
		if slots[i].FocusCapability == "" {
			slots[i].FocusCapability = firstRenderPreviewCapability(*preview)
		}
		slots[i].FocusRegions = reviewFocusRegions(*preview, slots[i].FocusCapability)
		slots[i].FocusLayerTypes = append([]string(nil), preview.LayerTypes...)
		slots[i].FocusStyleTokens = reviewFocusStyleTokens(*preview, slots[i].FocusCapability)
		if len(slots[i].PreviewCapabilities) == 0 {
			slots[i].PreviewCapabilities = buildRenderPreviewCapabilities(GenerationWorkQueueItem{RenderPreviewLayerTypes: append([]string(nil), preview.LayerTypes...)})
		}
		if slots[i].AssetID == "" {
			slots[i].AssetID = preview.AssetID
		}
		slots[i].AssetRevision = preview.AssetRevision
		slots[i].PreviewRevision = preview.PreviewRevision
		slots[i].TaskRevision = preview.TaskRevision
	}
}

func findGenerationReviewPreview(previews []PlatformAssetRenderPreviews, platform, slot, capability string) *AssetRenderPreviewSlot {
	var platformFallback *AssetRenderPreviewSlot
	for _, group := range previews {
		if platform != "" && group.Platform != platform {
			continue
		}
		for _, candidate := range flattenPlatformRenderPreviewSlots(group) {
			candidate := candidate
			if slot != "" && candidate.Slot != slot {
				if platformFallback == nil && matchesReviewPreviewCapability(candidate, capability) {
					platformFallback = &candidate
				}
				continue
			}
			if matchesReviewPreviewCapability(candidate, capability) {
				return &candidate
			}
			if platformFallback == nil {
				platformFallback = &candidate
			}
		}
	}
	return platformFallback
}

func matchesReviewPreviewCapability(slot AssetRenderPreviewSlot, capability string) bool {
	if capability == "" {
		return true
	}
	for _, item := range buildRenderPreviewCapabilities(GenerationWorkQueueItem{RenderPreviewLayerTypes: append([]string(nil), slot.LayerTypes...)}) {
		if item == capability {
			return true
		}
	}
	return false
}

func firstRenderPreviewCapability(slot AssetRenderPreviewSlot) string {
	capabilities := buildRenderPreviewCapabilities(GenerationWorkQueueItem{RenderPreviewLayerTypes: append([]string(nil), slot.LayerTypes...)})
	if len(capabilities) == 0 {
		return ""
	}
	return capabilities[0]
}

func reviewFocusRegions(slot AssetRenderPreviewSlot, capability string) []string {
	if len(slot.Regions) > 0 {
		return append([]string(nil), slot.Regions...)
	}
	switch capability {
	case "detail_preview":
		return []string{"detail"}
	case "measurement_preview":
		return []string{"measurement"}
	case "badge_preview":
		return []string{"badge"}
	case "copy_preview":
		return []string{"copy"}
	case "subject_preview":
		return []string{"subject"}
	default:
		if len(slot.LayerTypes) > 0 {
			return []string{slot.LayerTypes[0]}
		}
		return nil
	}
}

func reviewFocusStyleTokens(slot AssetRenderPreviewSlot, capability string) []string {
	if len(slot.StyleTokens) > 0 {
		return append([]string(nil), slot.StyleTokens...)
	}
	switch capability {
	case "detail_preview":
		return []string{"detail-focus"}
	case "measurement_preview":
		return []string{"measurement-focus"}
	case "badge_preview":
		return []string{"badge-focus"}
	case "copy_preview":
		return []string{"copy-focus"}
	case "subject_preview":
		return []string{"subject-focus"}
	default:
		return nil
	}
}
