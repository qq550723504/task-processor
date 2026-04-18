package listingkit

import "strings"

func resolveGenerationReviewPreviewResponse(session *GenerationReviewSession, query *GenerationQueueQuery) (*GenerationReviewPreviewViewer, *AssetRenderPreviewSlot, *GenerationReviewTarget, *GenerationReviewToolbarInput) {
	if session == nil {
		return nil, nil, nil, nil
	}
	if query == nil || strings.TrimSpace(query.AssetID) == "" {
		if session.FocusedToolbar == nil {
			return nil, session.FocusedRenderPreview, session.FocusedTarget, nil
		}
		return session.FocusedToolbar.GetViewer(), session.FocusedRenderPreview, session.FocusedTarget, session.FocusedToolbar
	}
	for _, group := range session.PlatformRenderPreviews {
		for _, slot := range flattenPlatformRenderPreviewSlots(group) {
			if slot.AssetID != strings.TrimSpace(query.AssetID) {
				continue
			}
			viewer := buildGenerationReviewPreviewViewer(group.Platform, slot.Slot, firstNonEmpty(query.PreviewCapability, session.FocusCapability), &slot)
			target := buildGenerationReviewTarget(group.Platform, slot.Slot, firstNonEmpty(query.PreviewCapability, session.FocusCapability))
			target = enrichGenerationReviewTargetWithContext(target, group.Platform, slot.Slot, firstNonEmpty(query.PreviewCapability, session.FocusCapability), generationReviewSectionKey(firstNonEmpty(query.PreviewCapability, session.FocusCapability)), &slot)
			toolbar := buildGenerationReviewToolbarInput(session.Queue, session.PlatformRenderPreviews, session.SlotNavigation, group.Platform, slot.Slot, firstNonEmpty(query.PreviewCapability, session.FocusCapability))
			return viewer, &slot, target, toolbar
		}
	}
	return &GenerationReviewPreviewViewer{
		Platform: strings.TrimSpace(query.Platform),
		Slot:     strings.TrimSpace(query.Slot),
		FocusKey: generationReviewFocusKey(strings.TrimSpace(query.Platform), strings.TrimSpace(query.Slot), strings.TrimSpace(query.PreviewCapability)),
	}, nil, nil, nil
}

func resolveGenerationReviewPreviewRevisionStatus(viewer *GenerationReviewPreviewViewer, query *GenerationQueueQuery) (string, string) {
	if viewer == nil || query == nil {
		return "", ""
	}
	if query.AssetID != "" && viewer.AssetID != query.AssetID {
		return "mismatch", "requested asset preview is not available in the current task revision"
	}
	if query.AssetRevision != "" && viewer.AssetRevision != "" && query.AssetRevision != viewer.AssetRevision {
		return "mismatch", "asset revision does not match current preview"
	}
	if query.PreviewRevision != "" && viewer.PreviewRevision != "" && query.PreviewRevision != viewer.PreviewRevision {
		return "mismatch", "preview revision does not match current preview"
	}
	if query.TaskRevision != "" && viewer.TaskRevision != "" && query.TaskRevision != viewer.TaskRevision {
		return "mismatch", "task revision does not match current preview"
	}
	return "matched", ""
}
