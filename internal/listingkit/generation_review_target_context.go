package listingkit

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
