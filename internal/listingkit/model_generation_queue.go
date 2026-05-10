package listingkit

import "time"

type GenerationWorkQueueSummary struct {
	TotalItems                      int                                  `json:"total_items"`
	ReadyItems                      int                                  `json:"ready_items"`
	FallbackItems                   int                                  `json:"fallback_items"`
	MissingItems                    int                                  `json:"missing_items"`
	QueuedItems                     int                                  `json:"queued_items"`
	RunningItems                    int                                  `json:"running_items"`
	CompletedItems                  int                                  `json:"completed_items"`
	FailedItems                     int                                  `json:"failed_items"`
	StubbedItems                    int                                  `json:"stubbed_items"`
	RetryableItems                  int                                  `json:"retryable_items"`
	PreviewableItems                int                                  `json:"previewable_items"`
	Platforms                       []string                             `json:"platforms,omitempty"`
	PlatformCounts                  map[string]int                       `json:"platform_counts,omitempty"`
	PlatformPreviewableCounts       map[string]int                       `json:"platform_previewable_counts,omitempty"`
	PreviewCapabilityCounts         map[string]int                       `json:"preview_capability_counts,omitempty"`
	PlatformPreviewCapabilityCounts map[string]map[string]int            `json:"platform_preview_capability_counts,omitempty"`
	StateCounts                     map[string]int                       `json:"state_counts,omitempty"`
	PlatformStateCounts             map[string]map[string]int            `json:"platform_state_counts,omitempty"`
	ExecutionQualityCounts          map[string]int                       `json:"execution_quality_counts,omitempty"`
	ExecutionQualityLabels          map[string]string                    `json:"execution_quality_labels,omitempty"`
	PlatformExecutionQualityCounts  map[string]map[string]int            `json:"platform_execution_quality_counts,omitempty"`
	QualityGradeCounts              map[string]int                       `json:"quality_grade_counts,omitempty"`
	QualityGradeLabels              map[string]string                    `json:"quality_grade_labels,omitempty"`
	PlatformQualityGradeCounts      map[string]map[string]int            `json:"platform_quality_grade_counts,omitempty"`
	GradeStateCounts                map[string]map[string]int            `json:"grade_state_counts,omitempty"`
	PlatformGradeStateCounts        map[string]map[string]map[string]int `json:"platform_grade_state_counts,omitempty"`
	DominantQualityGrade            string                               `json:"dominant_quality_grade,omitempty"`
	DominantQualityGradeLabel       string                               `json:"dominant_quality_grade_label,omitempty"`
	ApprovedSections                int                                  `json:"approved_sections"`
	DeferredSections                int                                  `json:"deferred_sections"`
	ReviewPendingSections           int                                  `json:"review_pending_sections"`
}

type GenerationWorkQueueItem struct {
	TaskID                   string                        `json:"task_id,omitempty"`
	GenerationTask           string                        `json:"generation_task,omitempty"`
	Platform                 string                        `json:"platform,omitempty"`
	Slot                     string                        `json:"slot,omitempty"`
	Purpose                  string                        `json:"purpose,omitempty"`
	IdealKind                string                        `json:"ideal_kind,omitempty"`
	State                    string                        `json:"state,omitempty"`
	SatisfiedBy              string                        `json:"satisfied_by,omitempty"`
	IsFallback               bool                          `json:"is_fallback,omitempty"`
	Retryable                bool                          `json:"retryable,omitempty"`
	RecipeID                 string                        `json:"recipe_id,omitempty"`
	TemplateLabel            string                        `json:"template_label,omitempty"`
	RenderProfile            string                        `json:"render_profile,omitempty"`
	AssetID                  string                        `json:"asset_id,omitempty"`
	ExecutionMode            string                        `json:"execution_mode,omitempty"`
	ExecutionState           string                        `json:"execution_status,omitempty"`
	RetryHint                string                        `json:"retry_hint,omitempty"`
	StateReason              string                        `json:"state_reason,omitempty"`
	SelectedAssetID          string                        `json:"selected_asset_id,omitempty"`
	TargetAssetKind          string                        `json:"target_asset_kind,omitempty"`
	ExecutionQuality         string                        `json:"execution_quality,omitempty"`
	ExecutionQualityLabel    string                        `json:"execution_quality_label,omitempty"`
	QualityGrade             string                        `json:"quality_grade,omitempty"`
	QualityGradeLabel        string                        `json:"quality_grade_label,omitempty"`
	RenderPreviewAvailable   bool                          `json:"render_preview_available,omitempty"`
	RenderPreviewFormat      string                        `json:"render_preview_format,omitempty"`
	RenderPreviewVisualMode  string                        `json:"render_preview_visual_mode,omitempty"`
	RenderPreviewLayerTypes  []string                      `json:"render_preview_layer_types,omitempty"`
	RenderPreviewRegions     []string                      `json:"render_preview_regions,omitempty"`
	RenderPreviewStyleTokens []string                      `json:"render_preview_style_tokens,omitempty"`
	PreviewCapabilities      []string                      `json:"preview_capabilities,omitempty"`
	ReviewDecision           string                        `json:"review_decision,omitempty"`
	ReviewStatus             string                        `json:"review_status,omitempty"`
	ReviewBlocked            bool                          `json:"review_blocked,omitempty"`
	ReviewedAt               *time.Time                    `json:"reviewed_at,omitempty"`
	ScenePreset              *GenerationScenePresetSummary `json:"scene_preset,omitempty"`
}

type GenerationWorkQueue struct {
	Summary *GenerationWorkQueueSummary `json:"summary,omitempty"`
	Items   []GenerationWorkQueueItem   `json:"items,omitempty"`
}
