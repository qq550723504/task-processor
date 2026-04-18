package listingkit

func buildGenerationPanelFocusedDescriptors(update *GenerationReviewPanelUpdate) []GenerationPanelResourceDescriptor {
	if update == nil {
		return nil
	}
	var out []GenerationPanelResourceDescriptor
	if item := buildGenerationPanelTargetDescriptor("focused_session", update.FocusedTarget); item != nil {
		applyGenerationPanelFocusedSourceMetadata(item, update)
		out = append(out, *item)
	}
	if update.FocusedToolbar != nil && update.FocusedToolbar.PreviewViewer != nil {
		if item := buildGenerationPanelViewerDescriptor("focused_preview", update.FocusedToolbar.PreviewViewer); item != nil {
			applyGenerationPanelFocusedSourceMetadata(item, update)
			out = append(out, *item)
		}
	}
	return uniqueGenerationPanelResourceDescriptors(out)
}

func buildGenerationPanelChangedDescriptors(update *GenerationReviewPanelUpdate) []GenerationPanelResourceDescriptor {
	if update == nil || update.ReviewPatch == nil {
		return nil
	}
	var out []GenerationPanelResourceDescriptor
	for _, section := range update.ReviewPatch.ChangedSections {
		if item := buildGenerationPanelTargetDescriptor("changed_section", section.ReviewTarget); item != nil {
			item.SectionKey = section.SectionKey
			out = append(out, *item)
		}
	}
	for _, slot := range update.ReviewPatch.ChangedSlots {
		if item := buildGenerationPanelTargetDescriptor("changed_slot", slot.ReviewTarget); item != nil {
			out = append(out, *item)
		}
	}
	for _, card := range update.ReviewPatch.ChangedPlatformCards {
		if item := buildGenerationPanelTargetDescriptor("changed_platform_card", card.ReviewTarget); item != nil {
			out = append(out, *item)
		}
	}
	return uniqueGenerationPanelResourceDescriptors(out)
}

func buildGenerationPanelTargetDescriptor(role string, target *GenerationReviewTarget) *GenerationPanelResourceDescriptor {
	if target == nil || target.NavigationTarget == nil || target.NavigationTarget.Descriptor == nil {
		return nil
	}
	return &GenerationPanelResourceDescriptor{
		Role:       role,
		Platform:   target.Platform,
		Slot:       target.Slot,
		Capability: target.Capability,
		SectionKey: target.SectionKey,
		Descriptor: cloneGenerationNavigationDescriptor(target.NavigationTarget.Descriptor),
	}
}

func buildGenerationPanelViewerDescriptor(role string, viewer *GenerationReviewPreviewViewer) *GenerationPanelResourceDescriptor {
	if viewer == nil || viewer.NavigationTarget == nil || viewer.NavigationTarget.Descriptor == nil {
		return nil
	}
	return &GenerationPanelResourceDescriptor{
		Role:       role,
		Platform:   viewer.Platform,
		Slot:       viewer.Slot,
		Descriptor: cloneGenerationNavigationDescriptor(viewer.NavigationTarget.Descriptor),
	}
}

func applyGenerationPanelFocusedSourceMetadata(item *GenerationPanelResourceDescriptor, update *GenerationReviewPanelUpdate) {
	if item == nil || update == nil {
		return
	}
	item.SourceKind = update.FocusedSourceKind
	item.SourceStep = update.FocusedSourceStep
	item.ViaFallback = update.FocusedViaFallback
	item.FallbackReason = update.FocusedFallbackReason
	if update.FocusedViaFallback {
		item.RecoveryScope = "focused_resource"
		item.RecoveryHint = "review_fallback"
		item.Retryable = false
	}
	applyGenerationPanelResourceRecovery(item)
}

func uniqueGenerationPanelResourceDescriptors(items []GenerationPanelResourceDescriptor) []GenerationPanelResourceDescriptor {
	if len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]GenerationPanelResourceDescriptor, 0, len(items))
	for _, item := range items {
		if item.Descriptor == nil || item.Descriptor.CacheKey == "" {
			continue
		}
		key := item.Role + "|" + item.Descriptor.CacheKey
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}
