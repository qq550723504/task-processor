package listingkit

func buildGenerationReviewSessionQuery(platform, slot, capability string) *GenerationQueueQuery {
	return &GenerationQueueQuery{
		Platform:          platform,
		Slot:              slot,
		PreviewCapability: capability,
		ResponseMode:      "patch_only",
	}
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
		cloned := *target.QueueQuery
		navigation.QueueQuery = &cloned
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
