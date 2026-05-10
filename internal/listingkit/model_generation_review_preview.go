package listingkit

type GenerationReviewToolbarInput struct {
	Platform         string                          `json:"platform,omitempty"`
	Slot             string                          `json:"slot,omitempty"`
	Capability       string                          `json:"capability,omitempty"`
	AssetID          string                          `json:"asset_id,omitempty"`
	VisualMode       string                          `json:"visual_mode,omitempty"`
	PreviewFormat    string                          `json:"preview_format,omitempty"`
	PreviewViewer    *GenerationReviewPreviewViewer  `json:"preview_viewer,omitempty"`
	FocusRegions     []string                        `json:"focus_regions,omitempty"`
	FocusLayerTypes  []string                        `json:"focus_layer_types,omitempty"`
	FocusStyleTokens []string                        `json:"focus_style_tokens,omitempty"`
	SectionActions   []GenerationReviewToolbarAction `json:"section_actions,omitempty"`
	PreviewActions   []GenerationReviewToolbarAction `json:"preview_actions,omitempty"`
}

type GenerationReviewToolbarAction struct {
	Key              string                            `json:"key,omitempty"`
	Label            string                            `json:"label,omitempty"`
	Kind             string                            `json:"kind,omitempty"`
	Selected         bool                              `json:"selected,omitempty"`
	Enabled          bool                              `json:"enabled"`
	Target           *GenerationReviewTarget           `json:"target,omitempty"`
	ActionTarget     *AssetGenerationActionTarget      `json:"action_target,omitempty"`
	ViewerTarget     *GenerationReviewPreviewViewer    `json:"viewer_target,omitempty"`
	NavigationTarget *GenerationReviewNavigationTarget `json:"navigation_target,omitempty"`
	PreviewQuery     *GenerationQueueQuery             `json:"preview_query,omitempty"`
}

type GenerationReviewPreviewViewer struct {
	Platform         string                            `json:"platform,omitempty"`
	Slot             string                            `json:"slot,omitempty"`
	AssetID          string                            `json:"asset_id,omitempty"`
	AssetRevision    string                            `json:"asset_revision,omitempty"`
	PreviewRevision  string                            `json:"preview_revision,omitempty"`
	TaskRevision     string                            `json:"task_revision,omitempty"`
	PreviewFormat    string                            `json:"preview_format,omitempty"`
	VisualMode       string                            `json:"visual_mode,omitempty"`
	FocusKey         string                            `json:"focus_key,omitempty"`
	NavigationTarget *GenerationReviewNavigationTarget `json:"navigation_target,omitempty"`
	PreviewQuery     *GenerationQueueQuery             `json:"preview_query,omitempty"`
}

type GenerationReviewWorkflowResult struct {
	ActionKey  string `json:"action_key,omitempty"`
	Status     string `json:"status,omitempty"`
	Platform   string `json:"platform,omitempty"`
	Slot       string `json:"slot,omitempty"`
	Capability string `json:"capability,omitempty"`
	Message    string `json:"message,omitempty"`
}

type GenerationReviewPreviewResponse struct {
	TaskID                 string                              `json:"task_id,omitempty"`
	DeltaToken             string                              `json:"delta_token,omitempty"`
	NotModified            bool                                `json:"not_modified,omitempty"`
	Conditional            *GenerationConditionalState         `json:"conditional,omitempty"`
	ResourceDescriptors    []GenerationPanelResourceDescriptor `json:"resource_descriptors,omitempty"`
	RecoverySummary        *GenerationRecoverySummary          `json:"recovery_summary,omitempty"`
	ResolvedActionSummary  *GenerationResolvedActionSummary    `json:"resolved_action_summary,omitempty"`
	Viewer                 *GenerationReviewPreviewViewer      `json:"viewer,omitempty"`
	Preview                *AssetRenderPreviewSlot             `json:"preview,omitempty"`
	ScenePreset            *GenerationScenePresetSummary       `json:"scene_preset,omitempty"`
	ReviewTarget           *GenerationReviewTarget             `json:"review_target,omitempty"`
	Toolbar                *GenerationReviewToolbarInput       `json:"toolbar,omitempty"`
	RevisionStatus         string                              `json:"revision_status,omitempty"`
	RevisionMismatchReason string                              `json:"revision_mismatch_reason,omitempty"`
}

type GenerationReviewSessionResponse struct {
	TaskID                string                              `json:"task_id,omitempty"`
	DeltaToken            string                              `json:"delta_token,omitempty"`
	NotModified           bool                                `json:"not_modified,omitempty"`
	ResponseMode          string                              `json:"response_mode,omitempty"`
	Conditional           *GenerationConditionalState         `json:"conditional,omitempty"`
	ResourceDescriptors   []GenerationPanelResourceDescriptor `json:"resource_descriptors,omitempty"`
	RecoverySummary       *GenerationRecoverySummary          `json:"recovery_summary,omitempty"`
	ResolvedActionSummary *GenerationResolvedActionSummary    `json:"resolved_action_summary,omitempty"`
	Patch                 *GenerationReviewSessionPatch       `json:"patch,omitempty"`
	Session               *GenerationReviewSession            `json:"session,omitempty"`
}
