package listingkit

import "time"

type GenerationReviewSession struct {
	SelectedPlatform       string                          `json:"selected_platform,omitempty"`
	SelectedSlot           string                          `json:"selected_slot,omitempty"`
	FocusCapability        string                          `json:"focus_capability,omitempty"`
	FocusedSectionKey      string                          `json:"focused_section_key,omitempty"`
	DefaultTarget          *GenerationReviewTarget         `json:"default_target,omitempty"`
	FocusedTarget          *GenerationReviewTarget         `json:"focused_target,omitempty"`
	FocusedRenderPreview   *AssetRenderPreviewSlot         `json:"focused_render_preview,omitempty"`
	FocusedScenePreset     *GenerationScenePresetSummary   `json:"focused_scene_preset,omitempty"`
	FocusedToolbar         *GenerationReviewToolbarInput   `json:"focused_toolbar,omitempty"`
	LastWorkflowResult     *GenerationReviewWorkflowResult `json:"last_workflow_result,omitempty"`
	ReviewSummary          *GenerationReviewSummary        `json:"review_summary,omitempty"`
	Queue                  *GenerationWorkQueue            `json:"queue,omitempty"`
	Overview               *AssetGenerationOverview        `json:"overview,omitempty"`
	PlatformCards          []ListingKitPlatformCard        `json:"platform_cards,omitempty"`
	PlatformRenderPreviews []PlatformAssetRenderPreviews   `json:"platform_render_previews,omitempty"`
	SlotNavigation         []GenerationReviewSlot          `json:"slot_navigation,omitempty"`
	Sections               []GenerationReviewSection       `json:"sections,omitempty"`
}

type GenerationReviewSection struct {
	Capability          string                          `json:"capability,omitempty"`
	CapabilityLabel     string                          `json:"capability_label,omitempty"`
	SectionKey          string                          `json:"section_key,omitempty"`
	Title               string                          `json:"title,omitempty"`
	Description         string                          `json:"description,omitempty"`
	EmptyState          string                          `json:"empty_state,omitempty"`
	Selected            bool                            `json:"selected,omitempty"`
	ItemCount           int                             `json:"item_count"`
	Platforms           []string                        `json:"platforms,omitempty"`
	PrimaryAction       string                          `json:"primary_action,omitempty"`
	PrimaryActionKey    string                          `json:"primary_action_key,omitempty"`
	PrimaryActionTarget *GenerationReviewTarget         `json:"primary_action_target,omitempty"`
	ReviewTarget        *GenerationReviewTarget         `json:"review_target,omitempty"`
	ToolbarActions      []GenerationReviewToolbarAction `json:"toolbar_actions,omitempty"`
	WorkflowActions     []GenerationReviewToolbarAction `json:"workflow_actions,omitempty"`
	WorkflowState       string                          `json:"workflow_state,omitempty"`
	WorkflowMessage     string                          `json:"workflow_message,omitempty"`
	ReviewDecision      string                          `json:"review_decision,omitempty"`
	ReviewStatus        string                          `json:"review_status,omitempty"`
	ReviewedAt          *time.Time                      `json:"reviewed_at,omitempty"`
	Slots               []GenerationReviewSlot          `json:"slots,omitempty"`
}

type GenerationReviewSlot struct {
	Platform               string                  `json:"platform,omitempty"`
	Slot                   string                  `json:"slot,omitempty"`
	Purpose                string                  `json:"purpose,omitempty"`
	State                  string                  `json:"state,omitempty"`
	QualityGrade           string                  `json:"quality_grade,omitempty"`
	QualityGradeLabel      string                  `json:"quality_grade_label,omitempty"`
	AssetID                string                  `json:"asset_id,omitempty"`
	TemplateLabel          string                  `json:"template_label,omitempty"`
	RenderPreviewAvailable bool                    `json:"render_preview_available,omitempty"`
	PreviewCapabilities    []string                `json:"preview_capabilities,omitempty"`
	FocusCapability        string                  `json:"focus_capability,omitempty"`
	FocusRegions           []string                `json:"focus_regions,omitempty"`
	FocusLayerTypes        []string                `json:"focus_layer_types,omitempty"`
	FocusStyleTokens       []string                `json:"focus_style_tokens,omitempty"`
	ReviewDecision         string                  `json:"review_decision,omitempty"`
	ReviewStatus           string                  `json:"review_status,omitempty"`
	Selected               bool                    `json:"selected,omitempty"`
	ReviewTarget           *GenerationReviewTarget `json:"review_target,omitempty"`
	AssetRevision          string                  `json:"asset_revision,omitempty"`
	PreviewRevision        string                  `json:"preview_revision,omitempty"`
	TaskRevision           string                  `json:"task_revision,omitempty"`
}

type GenerationReviewTarget struct {
	Platform         string                            `json:"platform,omitempty"`
	Slot             string                            `json:"slot,omitempty"`
	Capability       string                            `json:"capability,omitempty"`
	ActionKey        string                            `json:"action_key,omitempty"`
	SectionKey       string                            `json:"section_key,omitempty"`
	FocusKey         string                            `json:"focus_key,omitempty"`
	PanelState       *GenerationReviewPanelState       `json:"panel_state,omitempty"`
	NavigationDelta  *GenerationReviewNavigationDelta  `json:"navigation_delta,omitempty"`
	NavigationTarget *GenerationReviewNavigationTarget `json:"navigation_target,omitempty"`
	QueueQuery       *GenerationQueueQuery             `json:"queue_query,omitempty"`
	SessionQuery     *GenerationQueueQuery             `json:"session_query,omitempty"`
	AssetID          string                            `json:"asset_id,omitempty"`
	AssetRevision    string                            `json:"asset_revision,omitempty"`
	PreviewRevision  string                            `json:"preview_revision,omitempty"`
	TaskRevision     string                            `json:"task_revision,omitempty"`
}

type GenerationReviewPanelState struct {
	SelectedPlatform      string `json:"selected_platform,omitempty"`
	SelectedSlot          string `json:"selected_slot,omitempty"`
	FocusCapability       string `json:"focus_capability,omitempty"`
	FocusedSectionKey     string `json:"focused_section_key,omitempty"`
	FocusedPreviewAssetID string `json:"focused_preview_asset_id,omitempty"`
}
