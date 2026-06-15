package listingkit

type GenerationReviewSessionPatch struct {
	DeltaToken                  string                          `json:"delta_token,omitempty"`
	SelectedPlatform            string                          `json:"selected_platform,omitempty"`
	SelectedSlot                string                          `json:"selected_slot,omitempty"`
	FocusCapability             string                          `json:"focus_capability,omitempty"`
	FocusedSectionKey           string                          `json:"focused_section_key,omitempty"`
	FocusChanged                bool                            `json:"focus_changed,omitempty"`
	Overview                    *AssetGenerationOverview        `json:"overview,omitempty"`
	ReviewSummary               *GenerationReviewSummary        `json:"review_summary,omitempty"`
	QueueSummary                *GenerationWorkQueueSummary     `json:"queue_summary,omitempty"`
	LastWorkflowResult          *GenerationReviewWorkflowResult `json:"last_workflow_result,omitempty"`
	FocusedTarget               *GenerationReviewTarget         `json:"focused_target,omitempty"`
	FocusedRenderPreview        *AssetRenderPreviewSlot         `json:"focused_render_preview,omitempty"`
	FocusedToolbar              *GenerationReviewToolbarInput   `json:"focused_toolbar,omitempty"`
	Focus                       *GenerationReviewFocusPatch     `json:"focus,omitempty"`
	Queue                       *GenerationReviewQueuePatch     `json:"queue,omitempty"`
	PlatformCards               *GenerationReviewCardsPatch     `json:"platform_cards,omitempty"`
	RenderPreviews              *GenerationReviewPreviewsPatch  `json:"render_previews,omitempty"`
	ChangedSections             []GenerationReviewSection       `json:"changed_sections,omitempty"`
	ChangedSlots                []GenerationReviewSlot          `json:"changed_slots,omitempty"`
	ChangedPlatformCards        []ListingKitPlatformCard        `json:"changed_platform_cards,omitempty"`
	ChangedPlatformRenderGroups []PlatformAssetRenderPreviews   `json:"changed_platform_render_previews,omitempty"`
}

type GenerationReviewFocusPatch struct {
	SelectedPlatform     string                        `json:"selected_platform,omitempty"`
	SelectedSlot         string                        `json:"selected_slot,omitempty"`
	FocusCapability      string                        `json:"focus_capability,omitempty"`
	FocusedSectionKey    string                        `json:"focused_section_key,omitempty"`
	Changed              bool                          `json:"changed,omitempty"`
	FocusedTarget        *GenerationReviewTarget       `json:"focused_target,omitempty"`
	FocusedRenderPreview *AssetRenderPreviewSlot       `json:"focused_render_preview,omitempty"`
	FocusedToolbar       *GenerationReviewToolbarInput `json:"focused_toolbar,omitempty"`
}

type GenerationReviewQueuePatch struct {
	Summary         *GenerationWorkQueueSummary `json:"summary,omitempty"`
	ReviewSummary   *GenerationReviewSummary    `json:"review_summary,omitempty"`
	ChangedSections []GenerationReviewSection   `json:"changed_sections,omitempty"`
	ChangedSlots    []GenerationReviewSlot      `json:"changed_slots,omitempty"`
}

type GenerationReviewCardsPatch struct {
	ChangedPlatforms []string                 `json:"changed_platforms,omitempty"`
	Items            []ListingKitPlatformCard `json:"items,omitempty"`
}

type GenerationReviewPreviewsPatch struct {
	ChangedPlatforms []string                      `json:"changed_platforms,omitempty"`
	Items            []PlatformAssetRenderPreviews `json:"items,omitempty"`
}

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
