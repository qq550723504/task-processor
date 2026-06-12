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
	return listinggeneration.ReviewFocusKey(platform, slot, capability)
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

func enrichGenerationReviewTargetWithContext(target *GenerationReviewTarget, selectedPlatform, selectedSlot, selectedCapability, selectedSectionKey string, focusedPreview *AssetRenderPreviewSlot) *GenerationReviewTarget {
	if target == nil {
		return nil
	}
	cloned := *target
	if cloned.PanelState == nil {
		cloned.PanelState = &GenerationReviewPanelState{}
	}
	cloned.PanelState.SelectedPlatform = target.Platform
	cloned.PanelState.SelectedSlot = target.Slot
	cloned.PanelState.FocusCapability = target.Capability
	cloned.PanelState.FocusedSectionKey = target.SectionKey
	if focusedPreview != nil && target.Platform == selectedPlatform && target.Slot == selectedSlot {
		cloned.PanelState.FocusedPreviewAssetID = focusedPreview.AssetID
		cloned.AssetID = focusedPreview.AssetID
		cloned.AssetRevision = focusedPreview.AssetRevision
		cloned.PreviewRevision = focusedPreview.PreviewRevision
		cloned.TaskRevision = focusedPreview.TaskRevision
		if cloned.QueueQuery != nil {
			cloned.QueueQuery.AssetID = focusedPreview.AssetID
			cloned.QueueQuery.AssetRevision = focusedPreview.AssetRevision
			cloned.QueueQuery.PreviewRevision = focusedPreview.PreviewRevision
			cloned.QueueQuery.TaskRevision = focusedPreview.TaskRevision
		}
	}
	if cloned.NavigationTarget == nil {
		cloned.NavigationTarget = buildGenerationReviewNavigationTarget(target.Platform, target.Slot, target.Capability, nil)
	}
	if cloned.NavigationTarget.QueueQuery == nil {
		cloned.NavigationTarget.QueueQuery = buildGenerationReviewQueueQuery(target.Platform, target.Slot, target.Capability)
	}
	if cloned.SessionQuery == nil {
		cloned.SessionQuery = buildGenerationReviewSessionQuery(target.Platform, target.Slot, target.Capability)
	}
	cloned.SessionQuery.Platform = target.Platform
	cloned.SessionQuery.Slot = target.Slot
	cloned.SessionQuery.PreviewCapability = target.Capability
	cloned.SessionQuery.ResponseMode = "patch_only"
	cloned.SessionQuery.FromPlatform = selectedPlatform
	cloned.SessionQuery.FromSlot = selectedSlot
	cloned.SessionQuery.FromCapability = selectedCapability
	cloned.SessionQuery.FromSectionKey = selectedSectionKey
	cloned.NavigationTarget.SessionQuery = cloned.SessionQuery
	if cloned.NavigationTarget.PreviewQuery == nil {
		cloned.NavigationTarget.PreviewQuery = buildGenerationReviewPreviewQuery(target.Platform, target.Slot, target.Capability, nil)
	}
	if focusedPreview != nil && target.Platform == selectedPlatform && target.Slot == selectedSlot {
		cloned.NavigationTarget.PreviewQuery = buildGenerationReviewPreviewQuery(target.Platform, target.Slot, target.Capability, focusedPreview)
	}
	cloned.NavigationDelta = &GenerationReviewNavigationDelta{
		PlatformChanged:   selectedPlatform != "" && target.Platform != selectedPlatform,
		SlotChanged:       selectedSlot != "" && target.Slot != selectedSlot,
		CapabilityChanged: selectedCapability != "" && target.Capability != selectedCapability,
		SectionChanged:    selectedSectionKey != "" && target.SectionKey != selectedSectionKey,
	}
	return &cloned
}

func buildGenerationReviewSessionQuery(platform, slot, capability string) *GenerationQueueQuery {
	return &GenerationQueueQuery{
		Platform:          platform,
		Slot:              slot,
		PreviewCapability: capability,
		ResponseMode:      "patch_only",
	}
}

func buildGenerationReviewPreviewQuery(platform, slot, capability string, preview *AssetRenderPreviewSlot) *GenerationQueueQuery {
	query := &GenerationQueueQuery{
		Platform:          platform,
		Slot:              slot,
		PreviewCapability: capability,
	}
	if preview == nil {
		return query
	}
	query.AssetID = preview.AssetID
	query.AssetRevision = preview.AssetRevision
	query.PreviewRevision = preview.PreviewRevision
	query.TaskRevision = preview.TaskRevision
	return query
}

func buildGenerationReviewNavigationTarget(platform, slot, capability string, preview *AssetRenderPreviewSlot) *GenerationReviewNavigationTarget {
	return applyIdentityToNavigationTarget(&GenerationReviewNavigationTarget{
		DispatchKind: "session",
		QueueQuery:   buildGenerationReviewQueueQuery(platform, slot, capability),
		SessionQuery: buildGenerationReviewSessionQuery(platform, slot, capability),
		PreviewQuery: buildGenerationReviewPreviewQuery(platform, slot, capability, preview),
	})
}

func buildGenerationReviewQueueQuery(platform, slot, capability string) *GenerationQueueQuery {
	query := &GenerationQueueQuery{
		Platform:                      platform,
		Slot:                          slot,
		PreviewCapability:             capability,
		RenderPreviewAvailable:        true,
		RenderPreviewAvailablePresent: true,
		SortBy:                        "quality_grade",
		SortOrder:                     "asc",
	}
	if capability == "" {
		query.PreviewCapability = ""
	}
	if slot == "" {
		query.Slot = ""
	}
	return query
}

func buildGenerationReviewActionNavigationTarget(target *AssetGenerationActionTarget) *GenerationReviewNavigationTarget {
	if target == nil {
		return nil
	}
	navigation := &GenerationReviewNavigationTarget{
		DispatchKind: "action",
		ActionTarget: cloneAssetGenerationActionTargetForNavigation(target),
	}
	if target.QueueQuery != nil {
		navigation.QueueQuery = cloneGenerationQueueQuery(target.QueueQuery)
	}
	return applyIdentityToNavigationTarget(navigation)
}

func cloneAssetGenerationActionTargetForNavigation(target *AssetGenerationActionTarget) *AssetGenerationActionTarget {
	cloned := cloneAssetGenerationActionTarget(target)
	if cloned == nil {
		return nil
	}
	cloned.NavigationTarget = nil
	return cloned
}

func buildGenerationReviewPreviewNavigationTarget(platform, slot, capability string, preview *AssetRenderPreviewSlot) *GenerationReviewNavigationTarget {
	target := buildGenerationReviewNavigationTarget(platform, slot, capability, preview)
	target.DispatchKind = "preview"
	return applyIdentityToNavigationTarget(target)
}

func cloneGenerationReviewNavigationTarget(target *GenerationReviewNavigationTarget) *GenerationReviewNavigationTarget {
	if target == nil {
		return nil
	}
	cloned := *target
	applyGenerationReviewNavigationTargetCloneShape(target, &cloned)
	return applyIdentityToNavigationTarget(&cloned)
}
