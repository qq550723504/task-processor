package listingkit

import "strings"

func attachReviewTargetsToSlots(slots []GenerationReviewSlot, focusCapability string) {
	for i := range slots {
		capability := focusCapability
		if capability == "" && len(slots[i].PreviewCapabilities) > 0 {
			capability = slots[i].PreviewCapabilities[0]
		}
		slots[i].ReviewTarget = buildGenerationReviewTarget(slots[i].Platform, slots[i].Slot, capability)
	}
}

func attachReviewTargetsToPlatformCards(cards []ListingKitPlatformCard, selectedSlot, focusCapability string) {
	for i := range cards {
		cards[i].ReviewTarget = buildGenerationReviewTarget(cards[i].Platform, selectedSlot, focusCapability)
	}
}

func enrichReviewTargetsWithContext(slots []GenerationReviewSlot, sections []GenerationReviewSection, cards []ListingKitPlatformCard, selectedPlatform, selectedSlot, selectedCapability, selectedSectionKey string, focusedPreview *AssetRenderPreviewSlot) {
	for i := range slots {
		slots[i].ReviewTarget = enrichGenerationReviewTargetWithContext(slots[i].ReviewTarget, selectedPlatform, selectedSlot, selectedCapability, selectedSectionKey, focusedPreview)
	}
	for i := range sections {
		sections[i].PrimaryActionTarget = enrichGenerationReviewTargetWithContext(sections[i].PrimaryActionTarget, selectedPlatform, selectedSlot, selectedCapability, selectedSectionKey, focusedPreview)
		sections[i].ReviewTarget = enrichGenerationReviewTargetWithContext(sections[i].ReviewTarget, selectedPlatform, selectedSlot, selectedCapability, selectedSectionKey, focusedPreview)
		for j := range sections[i].ToolbarActions {
			sections[i].ToolbarActions[j].Target = enrichGenerationReviewTargetWithContext(sections[i].ToolbarActions[j].Target, selectedPlatform, selectedSlot, selectedCapability, selectedSectionKey, focusedPreview)
		}
		for j := range sections[i].WorkflowActions {
			sections[i].WorkflowActions[j].Target = enrichGenerationReviewTargetWithContext(sections[i].WorkflowActions[j].Target, selectedPlatform, selectedSlot, selectedCapability, selectedSectionKey, focusedPreview)
		}
	}
	for i := range cards {
		cards[i].ReviewTarget = enrichGenerationReviewTargetWithContext(cards[i].ReviewTarget, selectedPlatform, selectedSlot, selectedCapability, selectedSectionKey, focusedPreview)
	}
}

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

func cloneGenerationWorkQueue(queue *GenerationWorkQueue) *GenerationWorkQueue {
	if queue == nil {
		return nil
	}
	cloned := &GenerationWorkQueue{
		Items: append([]GenerationWorkQueueItem(nil), queue.Items...),
	}
	if queue.Summary != nil {
		summary := *queue.Summary
		cloned.Summary = &summary
	}
	return cloned
}
