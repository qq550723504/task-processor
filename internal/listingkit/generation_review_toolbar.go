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

func buildGenerationReviewToolbarInput(queue *GenerationWorkQueue, previews []PlatformAssetRenderPreviews, slots []GenerationReviewSlot, platform, slot, capability string) *GenerationReviewToolbarInput {
	preview := findGenerationReviewPreview(previews, platform, slot, capability)
	if preview == nil {
		return nil
	}
	focusCapability := capability
	if focusCapability == "" {
		focusCapability = firstRenderPreviewCapability(*preview)
	}
	return &GenerationReviewToolbarInput{
		Platform:         platform,
		Slot:             slot,
		Capability:       focusCapability,
		AssetID:          preview.AssetID,
		VisualMode:       preview.VisualMode,
		PreviewFormat:    preview.PreviewFormat,
		PreviewViewer:    buildGenerationReviewPreviewViewer(platform, slot, focusCapability, preview),
		FocusRegions:     reviewFocusRegions(*preview, focusCapability),
		FocusLayerTypes:  append([]string(nil), preview.LayerTypes...),
		FocusStyleTokens: reviewFocusStyleTokens(*preview, focusCapability),
		SectionActions:   buildGenerationReviewToolbarSectionActions(slots, platform, slot, focusCapability),
		PreviewActions:   buildGenerationReviewToolbarPreviewActions(queue, platform, slot, focusCapability, preview),
	}
}

func buildGenerationReviewToolbarSectionActions(slots []GenerationReviewSlot, platform, slot, selectedCapability string) []GenerationReviewToolbarAction {
	seen := map[string]struct{}{}
	out := make([]GenerationReviewToolbarAction, 0, 4)
	for _, item := range slots {
		if platform != "" && item.Platform != platform {
			continue
		}
		if slot != "" && item.Slot != slot {
			continue
		}
		for _, capability := range item.PreviewCapabilities {
			if _, ok := seen[capability]; ok {
				continue
			}
			seen[capability] = struct{}{}
			cfg := generationReviewSectionConfigForCapability(capability)
			out = append(out, GenerationReviewToolbarAction{
				Key:              cfg.ActionKey,
				Label:            cfg.Title,
				Kind:             "navigation",
				Selected:         capability == selectedCapability,
				Enabled:          true,
				Target:           buildGenerationReviewTarget(platform, slot, capability),
				NavigationTarget: buildGenerationReviewNavigationTarget(platform, slot, capability, nil),
				PreviewQuery:     buildGenerationReviewPreviewQuery(platform, slot, capability, nil),
			})
		}
	}
	return out
}

func buildGenerationReviewToolbarPreviewActions(queue *GenerationWorkQueue, platform, slot, capability string, preview *AssetRenderPreviewSlot) []GenerationReviewToolbarAction {
	target := buildGenerationReviewTarget(platform, slot, capability)
	if target == nil {
		return nil
	}
	viewer := buildGenerationReviewPreviewViewer(platform, slot, capability, preview)
	retryTarget := buildAssetGenerationActionTarget(queue, "retry_section_generation", &AssetGenerationRecommendedFilters{
		QualityGrade:           "provisional",
		QualityGradeLabel:      generationQualityGradeLabel("provisional"),
		Platforms:              []string{platform},
		RetryableOnly:          true,
		RenderPreviewAvailable: true,
		PreviewCapability:      capability,
	})
	approveTarget := buildAssetGenerationActionTarget(queue, "approve_section_review", &AssetGenerationRecommendedFilters{
		QualityGrade:           "ideal",
		QualityGradeLabel:      generationQualityGradeLabel("ideal"),
		Platforms:              []string{platform},
		RenderPreviewAvailable: true,
		PreviewCapability:      capability,
	})
	return []GenerationReviewToolbarAction{
		{
			Key:              "open_preview_svg",
			Label:            "Open SVG",
			Kind:             "viewer",
			Selected:         false,
			Enabled:          true,
			Target:           target,
			ViewerTarget:     viewer,
			NavigationTarget: buildGenerationReviewPreviewNavigationTarget(platform, slot, capability, preview),
			PreviewQuery:     buildGenerationReviewPreviewQuery(platform, slot, capability, preview),
		},
		{
			Key:              "retry_section_generation",
			Label:            "Retry Section",
			Kind:             "workflow",
			Selected:         false,
			Enabled:          true,
			ActionTarget:     retryTarget,
			NavigationTarget: buildGenerationReviewActionNavigationTarget(retryTarget),
		},
		{
			Key:              "approve_section_review",
			Label:            "Approve Review",
			Kind:             "workflow",
			Selected:         false,
			Enabled:          true,
			ActionTarget:     approveTarget,
			NavigationTarget: buildGenerationReviewActionNavigationTarget(approveTarget),
		},
	}
}

func buildGenerationReviewSectionToolbarActions(queue *GenerationWorkQueue, platform string, slots []GenerationReviewSlot, capability string) []GenerationReviewToolbarAction {
	cfg := generationReviewSectionConfigForCapability(capability)
	out := make([]GenerationReviewToolbarAction, 0, len(cfg.DefaultToolbarAction))
	sectionSlot := detectSectionSlot(slots)
	var preview *AssetRenderPreviewSlot
	for _, action := range cfg.DefaultToolbarAction {
		item := GenerationReviewToolbarAction{
			Key:     action.Key,
			Label:   action.Label,
			Kind:    "navigation",
			Enabled: true,
		}
		switch action.Key {
		case "open_preview_svg":
			item.Kind = "viewer"
			item.Target = buildGenerationReviewTarget(platform, sectionSlot, capability)
			item.ViewerTarget = buildGenerationReviewPreviewViewer(platform, sectionSlot, capability, preview)
			item.NavigationTarget = buildGenerationReviewNavigationTarget(platform, sectionSlot, capability, preview)
			item.PreviewQuery = buildGenerationReviewPreviewQuery(platform, sectionSlot, capability, preview)
		default:
			item.Target = buildGenerationReviewTarget(platform, sectionSlot, capability)
			item.Selected = action.Key == cfg.ActionKey
			item.NavigationTarget = buildGenerationReviewNavigationTarget(platform, sectionSlot, capability, preview)
			item.PreviewQuery = buildGenerationReviewPreviewQuery(platform, sectionSlot, capability, preview)
		}
		out = append(out, item)
	}
	return out
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
