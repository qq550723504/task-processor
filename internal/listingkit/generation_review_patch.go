package listingkit

import "encoding/json"

func buildGenerationReviewSessionPatch(before, after *GenerationReviewSession) *GenerationReviewSessionPatch {
	if after == nil {
		return nil
	}
	focusChanged := generationReviewSessionFocusChanged(before, after)
	queueSummaryChanged := generationReviewQueueSummaryChanged(before, after)
	reviewSummaryChanged := generationReviewSummaryChanged(before, after)
	overviewChanged := generationReviewOverviewChanged(before, after)
	patch := &GenerationReviewSessionPatch{
		DeltaToken: buildGenerationReviewDeltaToken(after),
	}
	if before == nil {
		patch.SelectedPlatform = after.SelectedPlatform
		patch.SelectedSlot = after.SelectedSlot
		patch.FocusCapability = after.FocusCapability
		patch.FocusedSectionKey = after.FocusedSectionKey
		patch.Overview = after.Overview
		patch.ReviewSummary = cloneGenerationReviewSummary(after.ReviewSummary)
		patch.QueueSummary = cloneGenerationWorkQueueSummary(after.Queue)
		patch.FocusedTarget = after.FocusedTarget
		patch.FocusedRenderPreview = after.FocusedRenderPreview
		patch.FocusedToolbar = after.FocusedToolbar
		patch.Focus = &GenerationReviewFocusPatch{
			SelectedPlatform:     after.SelectedPlatform,
			SelectedSlot:         after.SelectedSlot,
			FocusCapability:      after.FocusCapability,
			FocusedSectionKey:    after.FocusedSectionKey,
			FocusedTarget:        after.FocusedTarget,
			FocusedRenderPreview: after.FocusedRenderPreview,
			FocusedToolbar:       after.FocusedToolbar,
		}
		patch.FocusChanged = true
		patch.Focus.Changed = true
		patch.ChangedSections = append([]GenerationReviewSection(nil), after.Sections...)
		patch.ChangedSlots = append([]GenerationReviewSlot(nil), after.SlotNavigation...)
		patch.ChangedPlatformCards = append([]ListingKitPlatformCard(nil), after.PlatformCards...)
		patch.ChangedPlatformRenderGroups = append([]PlatformAssetRenderPreviews(nil), after.PlatformRenderPreviews...)
		patch.Queue = buildGenerationReviewQueuePatch(patch.QueueSummary, patch.ReviewSummary, patch.ChangedSections, patch.ChangedSlots, true, true)
		patch.PlatformCards = buildGenerationReviewCardsPatch(after.PlatformCards)
		patch.RenderPreviews = buildGenerationReviewPreviewsPatch(after.PlatformRenderPreviews)
		return patch
	}
	patch.FocusChanged = focusChanged
	if patch.FocusChanged {
		patch.SelectedPlatform = after.SelectedPlatform
		patch.SelectedSlot = after.SelectedSlot
		patch.FocusCapability = after.FocusCapability
		patch.FocusedSectionKey = after.FocusedSectionKey
		patch.FocusedTarget = after.FocusedTarget
		patch.FocusedRenderPreview = after.FocusedRenderPreview
		patch.FocusedToolbar = after.FocusedToolbar
		patch.Focus = &GenerationReviewFocusPatch{
			SelectedPlatform:     after.SelectedPlatform,
			SelectedSlot:         after.SelectedSlot,
			FocusCapability:      after.FocusCapability,
			FocusedSectionKey:    after.FocusedSectionKey,
			FocusedTarget:        after.FocusedTarget,
			FocusedRenderPreview: after.FocusedRenderPreview,
			FocusedToolbar:       after.FocusedToolbar,
		}
		patch.Focus.Changed = true
	}
	if overviewChanged {
		patch.Overview = after.Overview
	}
	if reviewSummaryChanged {
		patch.ReviewSummary = cloneGenerationReviewSummary(after.ReviewSummary)
	}
	if queueSummaryChanged {
		patch.QueueSummary = cloneGenerationWorkQueueSummary(after.Queue)
	}
	patch.ChangedSections = diffGenerationReviewSections(before.Sections, after.Sections)
	patch.ChangedSlots = diffGenerationReviewSlots(before.SlotNavigation, after.SlotNavigation)
	patch.ChangedPlatformCards = diffGenerationPlatformCards(before.PlatformCards, after.PlatformCards)
	patch.ChangedPlatformRenderGroups = diffPlatformAssetRenderPreviews(before.PlatformRenderPreviews, after.PlatformRenderPreviews)
	patch.Queue = buildGenerationReviewQueuePatch(
		patch.QueueSummary,
		patch.ReviewSummary,
		patch.ChangedSections,
		patch.ChangedSlots,
		queueSummaryChanged,
		reviewSummaryChanged,
	)
	patch.PlatformCards = buildGenerationReviewCardsPatch(patch.ChangedPlatformCards)
	patch.RenderPreviews = buildGenerationReviewPreviewsPatch(patch.ChangedPlatformRenderGroups)
	return patch
}

func generationReviewSessionFocusChanged(before, after *GenerationReviewSession) bool {
	if before == nil || after == nil {
		return before != after
	}
	if before.SelectedPlatform != after.SelectedPlatform ||
		before.SelectedSlot != after.SelectedSlot ||
		before.FocusCapability != after.FocusCapability ||
		before.FocusedSectionKey != after.FocusedSectionKey {
		return true
	}
	if !equalReviewPatchValue(before.FocusedTarget, after.FocusedTarget) {
		return true
	}
	if !equalReviewPatchValue(before.FocusedRenderPreview, after.FocusedRenderPreview) {
		return true
	}
	return !equalReviewPatchValue(before.FocusedToolbar, after.FocusedToolbar)
}

func diffGenerationReviewSections(before, after []GenerationReviewSection) []GenerationReviewSection {
	beforeIndex := map[string]GenerationReviewSection{}
	for _, item := range before {
		beforeIndex[item.SectionKey] = item
	}
	changed := make([]GenerationReviewSection, 0, len(after))
	for _, item := range after {
		if !equalReviewPatchValue(beforeIndex[item.SectionKey], item) {
			changed = append(changed, item)
		}
	}
	return changed
}

func diffGenerationReviewSlots(before, after []GenerationReviewSlot) []GenerationReviewSlot {
	beforeIndex := map[string]GenerationReviewSlot{}
	for _, item := range before {
		beforeIndex[item.Platform+":"+item.Slot] = item
	}
	changed := make([]GenerationReviewSlot, 0, len(after))
	for _, item := range after {
		if !equalReviewPatchValue(beforeIndex[item.Platform+":"+item.Slot], item) {
			changed = append(changed, item)
		}
	}
	return changed
}

func diffGenerationPlatformCards(before, after []ListingKitPlatformCard) []ListingKitPlatformCard {
	beforeIndex := map[string]ListingKitPlatformCard{}
	for _, item := range before {
		beforeIndex[item.Platform] = item
	}
	changed := make([]ListingKitPlatformCard, 0, len(after))
	for _, item := range after {
		if !equalReviewPatchValue(beforeIndex[item.Platform], item) {
			changed = append(changed, item)
		}
	}
	return changed
}

func diffPlatformAssetRenderPreviews(before, after []PlatformAssetRenderPreviews) []PlatformAssetRenderPreviews {
	beforeIndex := map[string]PlatformAssetRenderPreviews{}
	for _, item := range before {
		beforeIndex[item.Platform] = item
	}
	changed := make([]PlatformAssetRenderPreviews, 0, len(after))
	for _, item := range after {
		if !equalReviewPatchValue(beforeIndex[item.Platform], item) {
			changed = append(changed, item)
		}
	}
	return changed
}

func cloneGenerationWorkQueueSummary(queue *GenerationWorkQueue) *GenerationWorkQueueSummary {
	if queue == nil || queue.Summary == nil {
		return nil
	}
	summary := *queue.Summary
	return &summary
}

func buildGenerationReviewQueuePatch(summary *GenerationWorkQueueSummary, reviewSummary *GenerationReviewSummary, sections []GenerationReviewSection, slots []GenerationReviewSlot, summaryChanged, reviewSummaryChanged bool) *GenerationReviewQueuePatch {
	if !summaryChanged {
		summary = nil
	}
	if !reviewSummaryChanged {
		reviewSummary = nil
	}
	if summary == nil && reviewSummary == nil && len(sections) == 0 && len(slots) == 0 {
		return nil
	}
	return &GenerationReviewQueuePatch{
		Summary:         summary,
		ReviewSummary:   reviewSummary,
		ChangedSections: append([]GenerationReviewSection(nil), sections...),
		ChangedSlots:    append([]GenerationReviewSlot(nil), slots...),
	}
}

func generationReviewQueueSummaryChanged(before, after *GenerationReviewSession) bool {
	if before == nil || after == nil {
		return before != after
	}
	var beforeSummary any
	if before.Queue != nil {
		beforeSummary = before.Queue.Summary
	}
	var afterSummary any
	if after.Queue != nil {
		afterSummary = after.Queue.Summary
	}
	return !equalReviewPatchValue(beforeSummary, afterSummary)
}

func generationReviewSummaryChanged(before, after *GenerationReviewSession) bool {
	if before == nil || after == nil {
		return before != after
	}
	return !equalReviewPatchValue(before.ReviewSummary, after.ReviewSummary)
}

func generationReviewOverviewChanged(before, after *GenerationReviewSession) bool {
	if before == nil || after == nil {
		return before != after
	}
	return !equalReviewPatchValue(before.Overview, after.Overview)
}

func buildGenerationReviewCardsPatch(items []ListingKitPlatformCard) *GenerationReviewCardsPatch {
	if len(items) == 0 {
		return nil
	}
	platforms := make([]string, 0, len(items))
	for _, item := range items {
		if item.Platform == "" {
			continue
		}
		platforms = append(platforms, item.Platform)
	}
	return &GenerationReviewCardsPatch{
		ChangedPlatforms: uniqueStrings(platforms),
		Items:            append([]ListingKitPlatformCard(nil), items...),
	}
}

func buildGenerationReviewPreviewsPatch(items []PlatformAssetRenderPreviews) *GenerationReviewPreviewsPatch {
	if len(items) == 0 {
		return nil
	}
	platforms := make([]string, 0, len(items))
	for _, item := range items {
		if item.Platform == "" {
			continue
		}
		platforms = append(platforms, item.Platform)
	}
	return &GenerationReviewPreviewsPatch{
		ChangedPlatforms: uniqueStrings(platforms),
		Items:            append([]PlatformAssetRenderPreviews(nil), items...),
	}
}

func equalReviewPatchValue(left, right any) bool {
	leftBytes, leftErr := json.Marshal(left)
	rightBytes, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return string(leftBytes) == string(rightBytes)
}

func isGenerationReviewSessionPatchEmpty(patch *GenerationReviewSessionPatch) bool {
	if patch == nil {
		return true
	}
	if patch.SelectedPlatform != "" ||
		patch.SelectedSlot != "" ||
		patch.FocusCapability != "" ||
		patch.FocusedSectionKey != "" ||
		patch.FocusChanged ||
		patch.Overview != nil ||
		patch.ReviewSummary != nil ||
		patch.QueueSummary != nil ||
		patch.LastWorkflowResult != nil ||
		patch.FocusedTarget != nil ||
		patch.FocusedRenderPreview != nil ||
		patch.FocusedToolbar != nil ||
		patch.Focus != nil ||
		patch.Queue != nil ||
		patch.PlatformCards != nil ||
		patch.RenderPreviews != nil {
		return false
	}
	return len(patch.ChangedSections) == 0 &&
		len(patch.ChangedSlots) == 0 &&
		len(patch.ChangedPlatformCards) == 0 &&
		len(patch.ChangedPlatformRenderGroups) == 0
}
