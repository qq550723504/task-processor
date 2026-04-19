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
