package listingkit

func buildGenerationReviewPreviewViewer(platform, slot, capability string, preview *AssetRenderPreviewSlot) *GenerationReviewPreviewViewer {
	if preview == nil {
		return nil
	}
	return &GenerationReviewPreviewViewer{
		Platform:         platform,
		Slot:             slot,
		AssetID:          preview.AssetID,
		AssetRevision:    preview.AssetRevision,
		PreviewRevision:  preview.PreviewRevision,
		TaskRevision:     preview.TaskRevision,
		PreviewFormat:    preview.PreviewFormat,
		VisualMode:       preview.VisualMode,
		FocusKey:         generationReviewFocusKey(platform, slot, capability),
		NavigationTarget: buildGenerationReviewPreviewNavigationTarget(platform, slot, capability, preview),
		PreviewQuery:     buildGenerationReviewPreviewQuery(platform, slot, capability, preview),
	}
}

func buildGenerationReviewSectionWorkflowActions(queue *GenerationWorkQueue, platform string, slots []GenerationReviewSlot, capability string) []GenerationReviewToolbarAction {
	sectionSlot := detectSectionSlot(slots)
	filters := &AssetGenerationRecommendedFilters{
		QualityGrade:           "provisional",
		QualityGradeLabel:      generationQualityGradeLabel("provisional"),
		Platforms:              []string{platform},
		RetryableOnly:          true,
		RenderPreviewAvailable: true,
		PreviewCapability:      capability,
	}
	retryTarget := buildAssetGenerationActionTarget(queue, "retry_section_generation", filters)
	deferTarget := buildAssetGenerationActionTarget(queue, "defer_section_review", filters)
	approveTarget := buildAssetGenerationActionTarget(queue, "approve_section_review", filters)
	return []GenerationReviewToolbarAction{
		{
			Key:              "retry_section_generation",
			Label:            "Retry Section",
			Kind:             "workflow",
			Enabled:          true,
			ActionTarget:     retryTarget,
			NavigationTarget: buildGenerationReviewActionNavigationTarget(retryTarget),
		},
		{
			Key:              "defer_section_review",
			Label:            "Defer Review",
			Kind:             "workflow",
			Enabled:          true,
			ActionTarget:     deferTarget,
			NavigationTarget: buildGenerationReviewActionNavigationTarget(deferTarget),
		},
		{
			Key:              "approve_section_review",
			Label:            "Approve Review",
			Kind:             "workflow",
			Enabled:          true,
			ActionTarget:     approveTarget,
			NavigationTarget: buildGenerationReviewActionNavigationTarget(approveTarget),
		},
		{
			Key:              "open_preview_svg",
			Label:            "Open SVG",
			Kind:             "viewer",
			Enabled:          true,
			Target:           buildGenerationReviewTarget(platform, sectionSlot, capability),
			NavigationTarget: buildGenerationReviewPreviewNavigationTarget(platform, sectionSlot, capability, nil),
			PreviewQuery:     buildGenerationReviewPreviewQuery(platform, sectionSlot, capability, nil),
		},
	}
}
