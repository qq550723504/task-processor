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
	buildGenerationReviewNavigationTargetCloneShapePhase().run(target, &cloned)
	return applyIdentityToNavigationTarget(&cloned)
}

type generationReviewNavigationTargetCloneShapePhase struct{}

func buildGenerationReviewNavigationTargetCloneShapePhase() *generationReviewNavigationTargetCloneShapePhase {
	return &generationReviewNavigationTargetCloneShapePhase{}
}

func (p *generationReviewNavigationTargetCloneShapePhase) run(target *GenerationReviewNavigationTarget, cloned *GenerationReviewNavigationTarget) {
	if target == nil || cloned == nil {
		return
	}
	cloned.Conditional = cloneGenerationConditionalState(target.Conditional)
	cloned.Descriptor = cloneGenerationNavigationDescriptor(target.Descriptor)
	cloned.QueueQuery = cloneGenerationQueueQuery(target.QueueQuery)
	cloned.SessionQuery = cloneGenerationQueueQuery(target.SessionQuery)
	cloned.PreviewQuery = cloneGenerationQueueQuery(target.PreviewQuery)
	cloned.ActionTarget = cloneAssetGenerationActionTarget(target.ActionTarget)
}
