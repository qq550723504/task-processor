package listingkit

import "strings"

func detectReviewSessionPlatform(queue *GenerationWorkQueue, previews []PlatformAssetRenderPreviews) string {
	if queue != nil {
		for _, item := range queue.Items {
			if item.Platform != "" {
				return item.Platform
			}
		}
	}
	for _, group := range previews {
		if group.Platform != "" {
			return group.Platform
		}
	}
	return ""
}

func buildGenerationReviewSlots(queue *GenerationWorkQueue, selectedPlatform string, previews []PlatformAssetRenderPreviews) []GenerationReviewSlot {
	out := make([]GenerationReviewSlot, 0, 8)
	slotIndex := map[string]int{}
	if queue != nil {
		for _, item := range queue.Items {
			if selectedPlatform != "" && item.Platform != selectedPlatform {
				continue
			}
			slot := GenerationReviewSlot{
				Platform:               item.Platform,
				Slot:                   item.Slot,
				Purpose:                item.Purpose,
				State:                  item.State,
				QualityGrade:           item.QualityGrade,
				QualityGradeLabel:      item.QualityGradeLabel,
				AssetID:                firstNonEmpty(item.SelectedAssetID, item.AssetID),
				TemplateLabel:          item.TemplateLabel,
				RenderPreviewAvailable: item.RenderPreviewAvailable,
				PreviewCapabilities:    append([]string(nil), item.PreviewCapabilities...),
			}
			key := item.Platform + ":" + item.Slot
			slotIndex[key] = len(out)
			out = append(out, slot)
		}
	}
	for _, group := range previews {
		if selectedPlatform != "" && group.Platform != selectedPlatform {
			continue
		}
		for _, slot := range flattenPlatformRenderPreviewSlots(group) {
			key := group.Platform + ":" + slot.Slot
			capabilities := buildRenderPreviewCapabilitiesForSlot(slot)
			if idx, ok := slotIndex[key]; ok {
				out[idx].RenderPreviewAvailable = true
				if out[idx].AssetID == "" {
					out[idx].AssetID = slot.AssetID
				}
				if out[idx].TemplateLabel == "" {
					out[idx].TemplateLabel = slot.TemplateLabel
				}
				if len(out[idx].PreviewCapabilities) == 0 {
					out[idx].PreviewCapabilities = capabilities
				}
				continue
			}
			out = append(out, GenerationReviewSlot{
				Platform:               group.Platform,
				Slot:                   slot.Slot,
				Purpose:                slot.Purpose,
				AssetID:                slot.AssetID,
				TemplateLabel:          slot.TemplateLabel,
				RenderPreviewAvailable: true,
				PreviewCapabilities:    capabilities,
			})
			slotIndex[key] = len(out) - 1
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func flattenPlatformRenderPreviewSlots(group PlatformAssetRenderPreviews) []AssetRenderPreviewSlot {
	out := make([]AssetRenderPreviewSlot, 0, 1+len(group.Gallery)+len(group.Auxiliary))
	if group.Main != nil {
		out = append(out, *group.Main)
	}
	out = append(out, group.Gallery...)
	out = append(out, group.Auxiliary...)
	return out
}

func detectReviewSessionSlot(slots []GenerationReviewSlot, query *GenerationQueueQuery) string {
	if query != nil {
		targetSlot := strings.TrimSpace(query.Slot)
		if targetSlot != "" {
			for _, slot := range slots {
				if slot.Slot == targetSlot {
					return slot.Slot
				}
			}
		}
	}
	for _, slot := range slots {
		if strings.EqualFold(slot.ReviewStatus, "pending") && slot.RenderPreviewAvailable {
			return slot.Slot
		}
	}
	for _, slot := range slots {
		if slot.RenderPreviewAvailable {
			return slot.Slot
		}
	}
	if len(slots) == 0 {
		return ""
	}
	return slots[0].Slot
}

func detectReviewSessionCapability(query *GenerationQueueQuery, slots []GenerationReviewSlot, previews []PlatformAssetRenderPreviews, state *generationReviewState) string {
	if query != nil && query.PreviewCapability != "" {
		return query.PreviewCapability
	}
	for _, slot := range slots {
		if strings.EqualFold(slot.ReviewStatus, "pending") && len(slot.PreviewCapabilities) > 0 {
			return slot.PreviewCapabilities[0]
		}
	}
	if state != nil {
		for _, item := range state.ByKey {
			if item.Pending {
				return item.Key.Capability
			}
		}
	}
	for _, slot := range slots {
		if len(slot.PreviewCapabilities) > 0 {
			return slot.PreviewCapabilities[0]
		}
	}
	for _, group := range previews {
		if group.Summary == nil {
			continue
		}
		for capability, count := range group.Summary.CapabilityCounts {
			if count > 0 {
				return capability
			}
		}
	}
	return ""
}

func markSelectedReviewSlots(slots []GenerationReviewSlot, selectedSlot string) {
	for i := range slots {
		slots[i].Selected = slots[i].Slot == selectedSlot
	}
}
