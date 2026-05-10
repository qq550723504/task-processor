package listingkit

import (
	"time"

	assetgeneration "task-processor/internal/asset/generation"
)

type AssetGenerationSummary struct {
	TotalTasks          int      `json:"total_tasks"`
	PlannedTasks        int      `json:"planned_tasks"`
	CompletedTasks      int      `json:"completed_tasks"`
	FailedTasks         int      `json:"failed_tasks"`
	RendererBackedTasks int      `json:"renderer_backed_tasks"`
	FallbackTasks       int      `json:"fallback_tasks"`
	RetryableTasks      int      `json:"retryable_tasks"`
	Platforms           []string `json:"platforms,omitempty"`
}

type GenerationTaskPage struct {
	TaskID        string                  `json:"task_id"`
	Summary       *AssetGenerationSummary `json:"summary,omitempty"`
	Page          int                     `json:"page"`
	PageSize      int                     `json:"page_size"`
	Total         int                     `json:"total"`
	Tasks         []assetgeneration.Task  `json:"tasks,omitempty"`
	MatchedQueue  *GenerationWorkQueue    `json:"matched_queue,omitempty"`
	ExecutedQueue *GenerationWorkQueue    `json:"executed_queue,omitempty"`
	UpdatedAt     time.Time               `json:"updated_at,omitempty"`
}

type GenerationTaskQuery struct {
	Platform        string `form:"platform" json:"platform,omitempty"`
	Slot            string `form:"slot" json:"slot,omitempty"`
	ExecutionMode   string `form:"execution_mode" json:"execution_mode,omitempty"`
	ExecutionStatus string `form:"execution_status" json:"execution_status,omitempty"`
	SatisfiedBy     string `form:"satisfied_by" json:"satisfied_by,omitempty"`
	Page            int    `form:"page" json:"page,omitempty"`
	PageSize        int    `form:"page_size" json:"page_size,omitempty"`
	SortBy          string `form:"sort_by" json:"sort_by,omitempty"`
	SortOrder       string `form:"sort_order" json:"sort_order,omitempty"`
}

type GenerationQueueQuery struct {
	Platform                      string `form:"platform" json:"platform,omitempty"`
	Slot                          string `form:"slot" json:"slot,omitempty"`
	FromPlatform                  string `form:"from_platform" json:"from_platform,omitempty"`
	FromSlot                      string `form:"from_slot" json:"from_slot,omitempty"`
	FromCapability                string `form:"from_capability" json:"from_capability,omitempty"`
	FromSectionKey                string `form:"from_section_key" json:"from_section_key,omitempty"`
	AssetID                       string `form:"asset_id" json:"asset_id,omitempty"`
	AssetRevision                 string `form:"asset_revision" json:"asset_revision,omitempty"`
	PreviewRevision               string `form:"preview_revision" json:"preview_revision,omitempty"`
	TaskRevision                  string `form:"task_revision" json:"task_revision,omitempty"`
	DeltaToken                    string `form:"delta_token" json:"delta_token,omitempty"`
	IfMatch                       string `form:"if_match" json:"if_match,omitempty"`
	ResponseMode                  string `form:"response_mode" json:"response_mode,omitempty"`
	State                         string `form:"state" json:"state,omitempty"`
	ExecutionMode                 string `form:"execution_mode" json:"execution_mode,omitempty"`
	ExecutionQuality              string `form:"execution_quality" json:"execution_quality,omitempty"`
	QualityGrade                  string `form:"quality_grade" json:"quality_grade,omitempty"`
	QualityGradeLabel             string `form:"quality_grade_label" json:"quality_grade_label,omitempty"`
	PreviewCapability             string `form:"preview_capability" json:"preview_capability,omitempty"`
	RenderPreviewAvailable        bool   `form:"render_preview_available" json:"render_preview_available,omitempty"`
	RenderPreviewAvailablePresent bool   `json:"-"`
	Retryable                     bool   `form:"retryable" json:"retryable,omitempty"`
	RetryablePresent              bool   `json:"-"`
	Page                          int    `form:"page" json:"page,omitempty"`
	PageSize                      int    `form:"page_size" json:"page_size,omitempty"`
	SortBy                        string `form:"sort_by" json:"sort_by,omitempty"`
	SortOrder                     string `form:"sort_order" json:"sort_order,omitempty"`
}

type GenerationQueuePage struct {
	TaskID                string                              `json:"task_id"`
	DeltaToken            string                              `json:"delta_token,omitempty"`
	NotModified           bool                                `json:"not_modified,omitempty"`
	Conditional           *GenerationConditionalState         `json:"conditional,omitempty"`
	ResourceDescriptors   []GenerationPanelResourceDescriptor `json:"resource_descriptors,omitempty"`
	RecoverySummary       *GenerationRecoverySummary          `json:"recovery_summary,omitempty"`
	ResolvedActionSummary *GenerationResolvedActionSummary    `json:"resolved_action_summary,omitempty"`
	Summary               *GenerationWorkQueueSummary         `json:"summary,omitempty"`
	Page                  int                                 `json:"page"`
	PageSize              int                                 `json:"page_size"`
	Total                 int                                 `json:"total"`
	Items                 []GenerationWorkQueueItem           `json:"items,omitempty"`
	UpdatedAt             time.Time                           `json:"updated_at,omitempty"`
}

type RetryGenerationTasksRequest struct {
	TaskIDs               []string `json:"task_ids,omitempty"`
	Slots                 []string `json:"slots,omitempty"`
	ExecutionQuality      string   `json:"execution_quality,omitempty"`
	ExecutionQualityLabel string   `json:"execution_quality_label,omitempty"`
	QualityGrade          string   `json:"quality_grade,omitempty"`
	QualityGradeLabel     string   `json:"quality_grade_label,omitempty"`
	FallbackOnly          bool     `json:"fallback_only,omitempty"`
	RendererOnly          bool     `json:"renderer_only,omitempty"`
}

type ChildTaskState struct {
	Kind   string `json:"kind"`
	TaskID string `json:"task_id,omitempty"`
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

type WorkflowStageStatus string

const (
	WorkflowStageStatusPending   WorkflowStageStatus = "pending"
	WorkflowStageStatusRunning   WorkflowStageStatus = "running"
	WorkflowStageStatusCompleted WorkflowStageStatus = "completed"
	WorkflowStageStatusSkipped   WorkflowStageStatus = "skipped"
	WorkflowStageStatusDegraded  WorkflowStageStatus = "degraded"
	WorkflowStageStatusFailed    WorkflowStageStatus = "failed"
)

type WorkflowIssueSeverity string

const (
	WorkflowIssueSeverityInfo     WorkflowIssueSeverity = "info"
	WorkflowIssueSeverityWarning  WorkflowIssueSeverity = "warning"
	WorkflowIssueSeverityReview   WorkflowIssueSeverity = "review"
	WorkflowIssueSeverityBlocking WorkflowIssueSeverity = "blocking"
)

type WorkflowStage struct {
	Kind       string              `json:"kind"`
	Status     WorkflowStageStatus `json:"status"`
	TaskID     string              `json:"task_id,omitempty"`
	Error      string              `json:"error,omitempty"`
	StartedAt  time.Time           `json:"started_at,omitempty"`
	FinishedAt *time.Time          `json:"finished_at,omitempty"`
	DurationMS int64               `json:"duration_ms,omitempty"`
}

type WorkflowIssue struct {
	Code     string                `json:"code,omitempty"`
	Severity WorkflowIssueSeverity `json:"severity"`
	Stage    string                `json:"stage,omitempty"`
	Message  string                `json:"message"`
	Detail   string                `json:"detail,omitempty"`
}

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

type GenerationReviewNavigationDelta struct {
	PlatformChanged   bool `json:"platform_changed,omitempty"`
	SlotChanged       bool `json:"slot_changed,omitempty"`
	CapabilityChanged bool `json:"capability_changed,omitempty"`
	SectionChanged    bool `json:"section_changed,omitempty"`
}

type GenerationReviewNavigationTarget struct {
	DispatchKind          string                          `json:"dispatch_kind,omitempty"`
	Conditional           *GenerationConditionalState     `json:"conditional,omitempty"`
	ResourceKind          string                          `json:"resource_kind,omitempty"`
	CacheKey              string                          `json:"cache_key,omitempty"`
	CachePolicy           string                          `json:"cache_policy,omitempty"`
	RevalidateAfterAction bool                            `json:"revalidate_after_action,omitempty"`
	Descriptor            *GenerationNavigationDescriptor `json:"descriptor,omitempty"`
	QueueQuery            *GenerationQueueQuery           `json:"queue_query,omitempty"`
	SessionQuery          *GenerationQueueQuery           `json:"session_query,omitempty"`
	PreviewQuery          *GenerationQueueQuery           `json:"preview_query,omitempty"`
	ActionTarget          *AssetGenerationActionTarget    `json:"action_target,omitempty"`
}

type GenerationNavigationDescriptor struct {
	ResourceKind                 string                             `json:"resource_kind,omitempty"`
	CacheKey                     string                             `json:"cache_key,omitempty"`
	CachePolicy                  string                             `json:"cache_policy,omitempty"`
	SupportsStaleWhileRevalidate bool                               `json:"supports_stale_while_revalidate,omitempty"`
	RevalidateAfterAction        bool                               `json:"revalidate_after_action,omitempty"`
	RefreshScope                 string                             `json:"refresh_scope,omitempty"`
	Invalidates                  []string                           `json:"invalidates,omitempty"`
	DispatchPlan                 *GenerationNavigationDispatchPlan  `json:"dispatch_plan,omitempty"`
	FollowUpReads                []GenerationNavigationFollowUpRead `json:"followup_reads,omitempty"`
	Conditional                  *GenerationConditionalState        `json:"conditional,omitempty"`
}

type GenerationNavigationDispatchPlan struct {
	Strategy           string                             `json:"strategy,omitempty"`
	StopOnNotModified  bool                               `json:"stop_on_not_modified,omitempty"`
	StopOnFirstSuccess bool                               `json:"stop_on_first_success,omitempty"`
	StopOnError        bool                               `json:"stop_on_error,omitempty"`
	FallbackStrategy   string                             `json:"fallback_strategy,omitempty"`
	MaxParallelism     int                                `json:"max_parallelism,omitempty"`
	DedupePolicy       string                             `json:"dedupe_policy,omitempty"`
	WinnerPolicy       string                             `json:"winner_policy,omitempty"`
	RequiresRevalidate bool                               `json:"requires_revalidate,omitempty"`
	Steps              []GenerationNavigationDispatchStep `json:"steps,omitempty"`
}

type GenerationNavigationDispatchStep struct {
	Kind               string                `json:"kind,omitempty"`
	ResponseMode       string                `json:"response_mode,omitempty"`
	CachePreference    string                `json:"cache_preference,omitempty"`
	RequiresRevalidate bool                  `json:"requires_revalidate,omitempty"`
	Query              *GenerationQueueQuery `json:"query,omitempty"`
}

type GenerationNavigationFollowUpRead struct {
	Kind         string                `json:"kind,omitempty"`
	ResponseMode string                `json:"response_mode,omitempty"`
	Query        *GenerationQueueQuery `json:"query,omitempty"`
}

type GenerationReviewNavigationDispatchRequest struct {
	ResponseMode string                            `json:"response_mode,omitempty"`
	PlanMode     string                            `json:"plan_mode,omitempty"`
	Target       *GenerationReviewNavigationTarget `json:"target,omitempty"`
}

type GenerationReviewNavigationDispatchResponse struct {
	TaskID                         string                                  `json:"task_id,omitempty"`
	DispatchKind                   string                                  `json:"dispatch_kind,omitempty"`
	ResponseMode                   string                                  `json:"response_mode,omitempty"`
	PlanMode                       string                                  `json:"plan_mode,omitempty"`
	DeltaToken                     string                                  `json:"delta_token,omitempty"`
	NotModified                    bool                                    `json:"not_modified,omitempty"`
	Conditional                    *GenerationConditionalState             `json:"conditional,omitempty"`
	ResourceDescriptors            []GenerationPanelResourceDescriptor     `json:"resource_descriptors,omitempty"`
	PrimaryRecoveryDescriptor      *GenerationPanelResourceDescriptor      `json:"primary_recovery_descriptor,omitempty"`
	RecommendedRecoveryDescriptors []GenerationPanelResourceDescriptor     `json:"recommended_recovery_descriptors,omitempty"`
	ResolvedActionSummary          *GenerationResolvedActionSummary        `json:"resolved_action_summary,omitempty"`
	ExecutedPlan                   *GenerationNavigationDispatchExecution  `json:"executed_plan,omitempty"`
	FocusedSourceKind              string                                  `json:"focused_source_kind,omitempty"`
	FocusedSourceStep              int                                     `json:"focused_source_step,omitempty"`
	FocusedViaFallback             bool                                    `json:"focused_via_fallback,omitempty"`
	FocusedFallbackReason          string                                  `json:"focused_fallback_reason,omitempty"`
	FocusedResolution              *GenerationNavigationDispatchResolution `json:"focused_resolution,omitempty"`
	Queue                          *GenerationQueuePage                    `json:"queue,omitempty"`
	ReviewSession                  *GenerationReviewSessionResponse        `json:"review_session,omitempty"`
	ReviewPreview                  *GenerationReviewPreviewResponse        `json:"review_preview,omitempty"`
	Action                         *GenerationActionExecutionResult        `json:"action,omitempty"`
	PanelUpdate                    *GenerationReviewPanelUpdate            `json:"panel_update,omitempty"`
}

type GenerationNavigationDispatchExecution struct {
	Strategy       string                                      `json:"strategy,omitempty"`
	StopReason     string                                      `json:"stop_reason,omitempty"`
	Partial        bool                                        `json:"partial,omitempty"`
	CompletedSteps int                                         `json:"completed_steps,omitempty"`
	FailedSteps    int                                         `json:"failed_steps,omitempty"`
	DedupedSteps   int                                         `json:"deduped_steps,omitempty"`
	Steps          []GenerationNavigationDispatchExecutionStep `json:"steps,omitempty"`
}

type GenerationNavigationDispatchExecutionStep struct {
	Kind               string                           `json:"kind,omitempty"`
	ResponseMode       string                           `json:"response_mode,omitempty"`
	CachePreference    string                           `json:"cache_preference,omitempty"`
	RequiresRevalidate bool                             `json:"requires_revalidate,omitempty"`
	Status             string                           `json:"status,omitempty"`
	Error              string                           `json:"error,omitempty"`
	ErrorKind          string                           `json:"error_kind,omitempty"`
	Retryable          bool                             `json:"retryable,omitempty"`
	RetryHint          string                           `json:"retry_hint,omitempty"`
	Winner             bool                             `json:"winner,omitempty"`
	FallbackApplied    bool                             `json:"fallback_applied,omitempty"`
	FallbackReason     string                           `json:"fallback_reason,omitempty"`
	FallbackCandidate  bool                             `json:"fallback_candidate,omitempty"`
	FallbackSourceKind string                           `json:"fallback_source_kind,omitempty"`
	DeduplicationKey   string                           `json:"deduplication_key,omitempty"`
	DeduplicatedFrom   int                              `json:"deduplicated_from,omitempty"`
	Executed           bool                             `json:"executed,omitempty"`
	Skipped            bool                             `json:"skipped,omitempty"`
	DeltaToken         string                           `json:"delta_token,omitempty"`
	NotModified        bool                             `json:"not_modified,omitempty"`
	NoChanges          bool                             `json:"no_changes,omitempty"`
	Queue              *GenerationQueuePage             `json:"queue,omitempty"`
	ReviewSession      *GenerationReviewSessionResponse `json:"review_session,omitempty"`
	ReviewPreview      *GenerationReviewPreviewResponse `json:"review_preview,omitempty"`
}

type GenerationReviewPanelUpdate struct {
	DispatchKind                   string                                  `json:"dispatch_kind,omitempty"`
	ResponseMode                   string                                  `json:"response_mode,omitempty"`
	DeltaToken                     string                                  `json:"delta_token,omitempty"`
	NoChanges                      bool                                    `json:"no_changes,omitempty"`
	Conditional                    *GenerationConditionalState             `json:"conditional,omitempty"`
	FocusedSourceKind              string                                  `json:"focused_source_kind,omitempty"`
	FocusedSourceStep              int                                     `json:"focused_source_step,omitempty"`
	FocusedViaFallback             bool                                    `json:"focused_via_fallback,omitempty"`
	FocusedFallbackReason          string                                  `json:"focused_fallback_reason,omitempty"`
	FocusedResolution              *GenerationNavigationDispatchResolution `json:"focused_resolution,omitempty"`
	FocusedDescriptors             []GenerationPanelResourceDescriptor     `json:"focused_descriptors,omitempty"`
	ChangedDescriptors             []GenerationPanelResourceDescriptor     `json:"changed_descriptors,omitempty"`
	PrimaryRecoveryDescriptor      *GenerationPanelResourceDescriptor      `json:"primary_recovery_descriptor,omitempty"`
	RecommendedRecoveryDescriptors []GenerationPanelResourceDescriptor     `json:"recommended_recovery_descriptors,omitempty"`
	Overview                       *AssetGenerationOverview                `json:"overview,omitempty"`
	QueueSummary                   *GenerationWorkQueueSummary             `json:"queue_summary,omitempty"`
	ReviewSummary                  *GenerationReviewSummary                `json:"review_summary,omitempty"`
	FocusedTarget                  *GenerationReviewTarget                 `json:"focused_target,omitempty"`
	FocusedRenderPreview           *AssetRenderPreviewSlot                 `json:"focused_render_preview,omitempty"`
	FocusedToolbar                 *GenerationReviewToolbarInput           `json:"focused_toolbar,omitempty"`
	ReviewPatch                    *GenerationReviewSessionPatch           `json:"review_patch,omitempty"`
	ReviewSession                  *GenerationReviewSessionResponse        `json:"review_session,omitempty"`
	ReviewPreview                  *GenerationReviewPreviewResponse        `json:"review_preview,omitempty"`
	Action                         *GenerationActionExecutionResult        `json:"action,omitempty"`
}

type GenerationPanelResourceDescriptor struct {
	Role                 string                            `json:"role,omitempty"`
	Platform             string                            `json:"platform,omitempty"`
	Slot                 string                            `json:"slot,omitempty"`
	Capability           string                            `json:"capability,omitempty"`
	SectionKey           string                            `json:"section_key,omitempty"`
	SourceKind           string                            `json:"source_kind,omitempty"`
	SourceStep           int                               `json:"source_step,omitempty"`
	ViaFallback          bool                              `json:"via_fallback,omitempty"`
	FallbackReason       string                            `json:"fallback_reason,omitempty"`
	RecoveryScope        string                            `json:"recovery_scope,omitempty"`
	RecoveryHint         string                            `json:"recovery_hint,omitempty"`
	Retryable            bool                              `json:"retryable,omitempty"`
	RecoverySeverity     string                            `json:"recovery_severity,omitempty"`
	RecoveryUrgency      string                            `json:"recovery_urgency,omitempty"`
	RecoveryCTAKind      string                            `json:"recovery_cta_kind,omitempty"`
	RecoveryActionKey    string                            `json:"recovery_action_key,omitempty"`
	RecoveryTarget       *GenerationReviewNavigationTarget `json:"recovery_target,omitempty"`
	RecoveryDispatchPlan *GenerationNavigationDispatchPlan `json:"recovery_dispatch_plan,omitempty"`
	Descriptor           *GenerationNavigationDescriptor   `json:"descriptor,omitempty"`
}

type GenerationNavigationDispatchResolution struct {
	SourceKind     string `json:"source_kind,omitempty"`
	SourceStep     int    `json:"source_step,omitempty"`
	ViaFallback    bool   `json:"via_fallback,omitempty"`
	FallbackReason string `json:"fallback_reason,omitempty"`
}

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

type GenerationConditionalState struct {
	DeltaToken  string `json:"delta_token,omitempty"`
	ETag        string `json:"etag,omitempty"`
	NotModified bool   `json:"not_modified,omitempty"`
	NoChanges   bool   `json:"no_changes,omitempty"`
}

type AssetGenerationActionImpact struct {
	MatchedItems   int      `json:"matched_items"`
	RetryableItems int      `json:"retryable_items"`
	Platforms      []string `json:"platforms,omitempty"`
	QualityGrades  []string `json:"quality_grades,omitempty"`
	States         []string `json:"states,omitempty"`
}

type GenerationActionAudit struct {
	RequestedActionKey string    `json:"requested_action_key,omitempty"`
	ResolvedActionKey  string    `json:"resolved_action_key,omitempty"`
	ResolutionSource   string    `json:"resolution_source,omitempty"`
	ExecutionPath      string    `json:"execution_path,omitempty"`
	ExecutedAt         time.Time `json:"executed_at,omitempty"`
}
