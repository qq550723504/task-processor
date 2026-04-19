package listingkit

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
	if spec := previewCapabilityActionSpecForKey(capabilityActionKey(capability)); spec != nil {
		return spec.ActionKey
	}
	switch capability {
	case "detail_preview":
		return assetGenerationActionReviewDetailPreviews
	case "measurement_preview":
		return assetGenerationActionReviewMeasurementPreviews
	case "badge_preview":
		return assetGenerationActionReviewBadgePreviews
	case "copy_preview":
		return assetGenerationActionReviewCopyPreviews
	case "subject_preview":
		return assetGenerationActionReviewSubjectPreviews
	default:
		return assetGenerationActionReviewReadyAssets
	}
}

func capabilityActionKey(capability string) string {
	switch capability {
	case "detail_preview":
		return assetGenerationActionReviewDetailPreviews
	case "measurement_preview":
		return assetGenerationActionReviewMeasurementPreviews
	case "badge_preview":
		return assetGenerationActionReviewBadgePreviews
	case "copy_preview":
		return assetGenerationActionReviewCopyPreviews
	case "subject_preview":
		return assetGenerationActionReviewSubjectPreviews
	default:
		return ""
	}
}

func reviewActionLabelForCapability(capability string) string {
	if spec := previewCapabilityActionSpecForKey(capabilityActionKey(capability)); spec != nil {
		return spec.Label
	}
	return "Review Previews"
}
