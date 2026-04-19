package listingkit

func buildActionPlatformRenderPreviews(result *ListingKitResult, query *GenerationQueueQuery) []PlatformAssetRenderPreviews {
	if result == nil {
		return nil
	}
	groups := result.PlatformAssetRenderPreviews
	if len(groups) == 0 {
		groups = buildPlatformAssetRenderPreviews(result)
	}
	if len(groups) == 0 {
		groups = buildPlatformAssetRenderPreviewsFromQueue(result)
	}
	if len(groups) == 0 {
		return nil
	}
	if query == nil {
		return groups
	}
	groups = filterPlatformAssetRenderPreviews(groups, query.Platform)
	if len(groups) == 0 {
		return buildFallbackActionPlatformRenderPreviews(result, query)
	}
	if !query.RenderPreviewAvailable && !query.RenderPreviewAvailablePresent && query.PreviewCapability == "" {
		return groups
	}
	out := make([]PlatformAssetRenderPreviews, 0, len(groups))
	for _, group := range groups {
		filtered := filterPlatformAssetRenderPreviewGroup(group, query)
		if filtered.Summary == nil && filtered.Main == nil && len(filtered.Gallery) == 0 && len(filtered.Auxiliary) == 0 {
			continue
		}
		out = append(out, filtered)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildFallbackActionPlatformRenderPreviews(result *ListingKitResult, query *GenerationQueueQuery) []PlatformAssetRenderPreviews {
	if result == nil {
		return nil
	}
	previews := result.AssetRenderPreviews
	if len(previews) == 0 {
		previews = buildAssetRenderPreviews(result.AssetBundle)
	}
	if len(previews) != 1 || previews[0].PreviewSVG == "" {
		return nil
	}
	platform := ""
	if query != nil {
		platform = query.Platform
	}
	if platform == "" && result.AssetGenerationQueue != nil && len(result.AssetGenerationQueue.Items) > 0 {
		platform = result.AssetGenerationQueue.Items[0].Platform
	}
	if platform == "" {
		return nil
	}
	preview := previews[0]
	group := PlatformAssetRenderPreviews{
		Platform: platform,
		Main: &AssetRenderPreviewSlot{
			Slot:                "main",
			AssetID:             preview.AssetID,
			Role:                preview.Role,
			TemplateLabel:       preview.TemplateLabel,
			RenderProfile:       preview.RenderProfile,
			PreviewFormat:       preview.PreviewFormat,
			PreviewSVG:          preview.PreviewSVG,
			SourceKind:          preview.SourceKind,
			GenerationMode:      preview.GenerationMode,
			VisualMode:          preview.VisualMode,
			LayoutEngine:        preview.LayoutEngine,
			RenderOutputVersion: preview.RenderOutputVersion,
			DrawOutputVersion:   preview.DrawOutputVersion,
			DrawPreviewVersion:  preview.DrawPreviewVersion,
			LayerTypes:          append([]string(nil), preview.LayerTypes...),
			Regions:             append([]string(nil), preview.Regions...),
			StyleTokens:         append([]string(nil), preview.StyleTokens...),
		},
	}
	group.Summary = buildPlatformAssetRenderPreviewSummary(group)
	return []PlatformAssetRenderPreviews{group}
}

func buildPlatformAssetRenderPreviewsFromQueue(result *ListingKitResult) []PlatformAssetRenderPreviews {
	if result == nil || result.AssetGenerationQueue == nil || len(result.AssetGenerationQueue.Items) == 0 {
		return nil
	}
	previewByAssetID := make(map[string]AssetRenderPreview, len(result.AssetRenderPreviews))
	previews := result.AssetRenderPreviews
	if len(previews) == 0 {
		previews = buildAssetRenderPreviews(result.AssetBundle)
	}
	for _, preview := range previews {
		if preview.AssetID != "" {
			previewByAssetID[preview.AssetID] = preview
		}
	}
	if len(previewByAssetID) == 0 {
		return nil
	}
	var singlePreview *AssetRenderPreview
	if len(previewByAssetID) == 1 {
		for _, preview := range previewByAssetID {
			copyPreview := preview
			singlePreview = &copyPreview
		}
	}
	groupMap := map[string]*PlatformAssetRenderPreviews{}
	for _, item := range result.AssetGenerationQueue.Items {
		assetID := firstNonEmpty(item.SelectedAssetID, item.AssetID)
		if assetID == "" || item.Platform == "" {
			if singlePreview == nil || item.Platform == "" {
				continue
			}
		}
		preview, ok := previewByAssetID[assetID]
		if !ok || preview.PreviewSVG == "" {
			if singlePreview == nil || singlePreview.PreviewSVG == "" {
				continue
			}
			preview = *singlePreview
			if assetID == "" {
				assetID = preview.AssetID
			}
		}
		group := groupMap[item.Platform]
		if group == nil {
			group = &PlatformAssetRenderPreviews{Platform: item.Platform}
			groupMap[item.Platform] = group
		}
		slot := AssetRenderPreviewSlot{
			Slot:                item.Slot,
			Purpose:             item.Purpose,
			AssetID:             assetID,
			Kind:                item.IdealKind,
			Role:                preview.Role,
			StateLabel:          item.State,
			RetryHint:           item.RetryHint,
			TemplateLabel:       firstNonEmpty(item.TemplateLabel, preview.TemplateLabel),
			RenderProfile:       firstNonEmpty(item.RenderProfile, preview.RenderProfile),
			PreviewFormat:       preview.PreviewFormat,
			PreviewSVG:          preview.PreviewSVG,
			SourceKind:          preview.SourceKind,
			GenerationMode:      preview.GenerationMode,
			VisualMode:          preview.VisualMode,
			LayoutEngine:        preview.LayoutEngine,
			RenderOutputVersion: preview.RenderOutputVersion,
			DrawOutputVersion:   preview.DrawOutputVersion,
			DrawPreviewVersion:  preview.DrawPreviewVersion,
			LayerTypes:          append([]string(nil), preview.LayerTypes...),
			Regions:             append([]string(nil), preview.Regions...),
			StyleTokens:         append([]string(nil), preview.StyleTokens...),
		}
		switch item.Slot {
		case "main":
			if group.Main == nil {
				group.Main = &slot
			}
		case "gallery":
			group.Gallery = append(group.Gallery, slot)
		default:
			group.Auxiliary = append(group.Auxiliary, slot)
		}
	}
	out := make([]PlatformAssetRenderPreviews, 0, len(groupMap))
	for _, group := range groupMap {
		group.Summary = buildPlatformAssetRenderPreviewSummary(*group)
		out = append(out, *group)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func filterPlatformAssetRenderPreviewGroup(group PlatformAssetRenderPreviews, query *GenerationQueueQuery) PlatformAssetRenderPreviews {
	filtered := PlatformAssetRenderPreviews{Platform: group.Platform}
	if group.Main != nil && matchesRenderPreviewSlot(*group.Main, query) {
		slot := *group.Main
		filtered.Main = &slot
	}
	filtered.Gallery = filterAssetRenderPreviewSlots(group.Gallery, query)
	filtered.Auxiliary = filterAssetRenderPreviewSlots(group.Auxiliary, query)
	if filtered.Main == nil && len(filtered.Gallery) == 0 && len(filtered.Auxiliary) == 0 {
		return PlatformAssetRenderPreviews{}
	}
	filtered.Summary = buildPlatformAssetRenderPreviewSummary(filtered)
	return filtered
}

func filterAssetRenderPreviewSlots(slots []AssetRenderPreviewSlot, query *GenerationQueueQuery) []AssetRenderPreviewSlot {
	if len(slots) == 0 {
		return nil
	}
	out := make([]AssetRenderPreviewSlot, 0, len(slots))
	for _, slot := range slots {
		if matchesRenderPreviewSlot(slot, query) {
			out = append(out, slot)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func matchesRenderPreviewSlot(slot AssetRenderPreviewSlot, query *GenerationQueueQuery) bool {
	if query == nil {
		return true
	}
	if query.RenderPreviewAvailablePresent && !query.RenderPreviewAvailable {
		return false
	}
	if query.PreviewCapability == "" {
		return true
	}
	item := GenerationWorkQueueItem{RenderPreviewLayerTypes: append([]string(nil), slot.LayerTypes...)}
	for _, capability := range buildRenderPreviewCapabilities(item) {
		if capability == query.PreviewCapability {
			return true
		}
	}
	return false
}
