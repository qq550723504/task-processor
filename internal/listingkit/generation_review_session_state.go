package listingkit

func applyReviewStateToReviewSlots(slots []GenerationReviewSlot, state *generationReviewState) {
	if state == nil {
		return
	}
	for i := range slots {
		key := normalizeReviewKey(slots[i].Platform) + ":" + normalizeReviewKey(slots[i].Slot)
		if slotState, ok := state.SlotSummary[key]; ok {
			slots[i].ReviewDecision = slotState.Decision
			slots[i].ReviewStatus = slotState.Status
			slots[i].AssetRevision = resolveSlotReviewAssetRevision(state, slots[i].Platform, slots[i].Slot, slots[i].PreviewCapabilities)
			slots[i].PreviewRevision = resolveSlotReviewPreviewRevision(state, slots[i].Platform, slots[i].Slot, slots[i].PreviewCapabilities)
			slots[i].TaskRevision = resolveSlotReviewTaskRevision(state, slots[i].Platform, slots[i].Slot, slots[i].PreviewCapabilities)
		}
	}
}

func applyReviewStateToSection(section *GenerationReviewSection, state *generationReviewState) {
	if section == nil || state == nil {
		return
	}
	key := generationReviewStateKey{
		Platform:   normalizeReviewKey(detectSectionPlatform(section.Platforms)),
		Slot:       normalizeReviewKey(detectSectionSlot(section.Slots)),
		Capability: normalizeReviewKey(section.Capability),
	}
	item, ok := state.ByKey[key]
	if !ok {
		section.ReviewStatus = "pending"
		return
	}
	if item.Pending || item.Record == nil {
		section.ReviewStatus = "pending"
		return
	}
	section.ReviewDecision = string(item.Record.Decision)
	section.ReviewStatus = item.Record.Status
	if !item.Record.ReviewedAt.IsZero() {
		reviewedAt := item.Record.ReviewedAt
		section.ReviewedAt = &reviewedAt
	}
}

func resolveSlotReviewAssetRevision(state *generationReviewState, platform, slot string, capabilities []string) string {
	for _, capability := range capabilities {
		if item, ok := state.ByKey[generationReviewStateKey{
			Platform:   normalizeReviewKey(platform),
			Slot:       normalizeReviewKey(slot),
			Capability: normalizeReviewKey(capability),
		}]; ok {
			return item.AssetRev
		}
	}
	return ""
}

func resolveSlotReviewPreviewRevision(state *generationReviewState, platform, slot string, capabilities []string) string {
	for _, capability := range capabilities {
		if item, ok := state.ByKey[generationReviewStateKey{
			Platform:   normalizeReviewKey(platform),
			Slot:       normalizeReviewKey(slot),
			Capability: normalizeReviewKey(capability),
		}]; ok {
			return item.PreviewRev
		}
	}
	return ""
}

func resolveSlotReviewTaskRevision(state *generationReviewState, platform, slot string, capabilities []string) string {
	for _, capability := range capabilities {
		if item, ok := state.ByKey[generationReviewStateKey{
			Platform:   normalizeReviewKey(platform),
			Slot:       normalizeReviewKey(slot),
			Capability: normalizeReviewKey(capability),
		}]; ok {
			return item.TaskRev
		}
	}
	return ""
}
