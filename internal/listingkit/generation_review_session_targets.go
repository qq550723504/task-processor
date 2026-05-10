package listingkit

func attachReviewTargetsToSlots(slots []GenerationReviewSlot, focusCapability string) {
	for i := range slots {
		capability := focusCapability
		if capability == "" && len(slots[i].PreviewCapabilities) > 0 {
			capability = slots[i].PreviewCapabilities[0]
		}
		slots[i].ReviewTarget = buildGenerationReviewTarget(slots[i].Platform, slots[i].Slot, capability)
	}
}

func attachReviewTargetsToSections(sections []GenerationReviewSection) {
	for i := range sections {
		sections[i].PrimaryActionTarget = buildGenerationReviewTarget(detectSectionPlatform(sections[i].Platforms), detectSectionSlot(sections[i].Slots), sections[i].Capability)
		sections[i].ReviewTarget = buildGenerationReviewTarget(detectSectionPlatform(sections[i].Platforms), detectSectionSlot(sections[i].Slots), sections[i].Capability)
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
