package listingkit

type AssetGenerationRecommendedFilters struct {
	QualityGrade           string   `json:"quality_grade,omitempty"`
	QualityGradeLabel      string   `json:"quality_grade_label,omitempty"`
	Platforms              []string `json:"platforms,omitempty"`
	RetryableOnly          bool     `json:"retryable_only,omitempty"`
	ExecutionQuality       string   `json:"execution_quality,omitempty"`
	RenderPreviewAvailable bool     `json:"render_preview_available,omitempty"`
	PreviewCapability      string   `json:"preview_capability,omitempty"`
}

type AssetGenerationActionTarget struct {
	ActionKey        string                             `json:"action_key,omitempty"`
	InteractionMode  string                             `json:"interaction_mode,omitempty"`
	Filters          *AssetGenerationRecommendedFilters `json:"filters,omitempty"`
	NavigationTarget *GenerationReviewNavigationTarget  `json:"navigation_target,omitempty"`
	QueueQuery       *GenerationQueueQuery              `json:"queue_query,omitempty"`
	RetryRequest     *RetryGenerationTasksRequest       `json:"retry_request,omitempty"`
	ExpectedImpact   *AssetGenerationActionImpact       `json:"expected_impact,omitempty"`
}

type ExecuteGenerationActionRequest struct {
	ActionKey    string                       `json:"action_key,omitempty"`
	ResponseMode string                       `json:"response_mode,omitempty"`
	Target       *AssetGenerationActionTarget `json:"target,omitempty"`
}

type GenerationActionExecutionResult struct {
	ActionKey              string                              `json:"action_key,omitempty"`
	InteractionMode        string                              `json:"interaction_mode,omitempty"`
	ResponseMode           string                              `json:"response_mode,omitempty"`
	DeltaToken             string                              `json:"delta_token,omitempty"`
	Conditional            *GenerationConditionalState         `json:"conditional,omitempty"`
	ResourceDescriptors    []GenerationPanelResourceDescriptor `json:"resource_descriptors,omitempty"`
	RecoverySummary        *GenerationRecoverySummary          `json:"recovery_summary,omitempty"`
	ResolvedActionSummary  *GenerationResolvedActionSummary    `json:"resolved_action_summary,omitempty"`
	Overview               *AssetGenerationOverview            `json:"overview,omitempty"`
	ResolvedTarget         *AssetGenerationActionTarget        `json:"resolved_target,omitempty"`
	Queue                  *GenerationQueuePage                `json:"queue,omitempty"`
	Retry                  *GenerationTaskPage                 `json:"retry,omitempty"`
	ReviewWorkflow         *GenerationReviewWorkflowResult     `json:"review_workflow,omitempty"`
	ReviewSession          *GenerationReviewSession            `json:"review_session,omitempty"`
	ReviewPatch            *GenerationReviewSessionPatch       `json:"review_patch,omitempty"`
	PlatformRenderPreviews []PlatformAssetRenderPreviews       `json:"platform_render_previews,omitempty"`
	Audit                  *GenerationActionAudit              `json:"audit,omitempty"`
}
