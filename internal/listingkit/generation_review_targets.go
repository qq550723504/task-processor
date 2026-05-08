package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

func buildGenerationReviewTarget(platform, slot, capability string) *GenerationReviewTarget {
	actionKey := reviewActionKeyForCapability(capability)
	sectionKey := generationReviewSectionKey(capability)
	focusKey := generationReviewFocusKey(platform, slot, capability)
	query := buildGenerationReviewQueueQuery(platform, slot, capability)
	return &GenerationReviewTarget{
		Platform:   platform,
		Slot:       slot,
		Capability: capability,
		ActionKey:  actionKey,
		SectionKey: sectionKey,
		FocusKey:   focusKey,
		PanelState: &GenerationReviewPanelState{
			SelectedPlatform:  platform,
			SelectedSlot:      slot,
			FocusCapability:   capability,
			FocusedSectionKey: sectionKey,
		},
		QueueQuery:       query,
		SessionQuery:     buildGenerationReviewSessionQuery(platform, slot, capability),
		NavigationTarget: buildGenerationReviewNavigationTarget(platform, slot, capability, nil),
	}
}

func generationReviewFocusKey(platform, slot, capability string) string {
	out := platform
	if slot != "" {
		if out != "" {
			out += ":"
		}
		out += slot
	}
	if capability != "" {
		if out != "" {
			out += ":"
		}
		out += capability
	}
	return out
}

func reviewActionKeyForCapability(capability string) string {
	return listinggeneration.ReviewActionKeyForCapability(capability)
}

func capabilityActionKey(capability string) string {
	return listinggeneration.CapabilityActionKey(capability)
}

func reviewActionLabelForCapability(capability string) string {
	return listinggeneration.ReviewActionLabelForCapability(capability)
}
