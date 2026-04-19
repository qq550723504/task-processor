package listingkit

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	"task-processor/internal/catalog"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

var ErrTaskNotFound = errors.New("task not found")
var ErrTaskNotPending = errors.New("task is not pending")
var ErrGenerationTaskNotFound = errors.New("generation task not found")
var ErrGenerationTaskNotRetryable = errors.New("generation task is not retryable")
var ErrGenerationActionNotFound = errors.New("generation action not found")

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
)

type GenerateRequest struct {
	ImageURLs          []string         `json:"image_urls,omitempty"`
	Text               string           `json:"text,omitempty"`
	ProductURL         string           `json:"product_url,omitempty"`
	Platforms          []string         `json:"platforms,omitempty"`
	Country            string           `json:"country,omitempty"`
	Language           string           `json:"language,omitempty"`
	SheinStoreID       int64            `json:"shein_store_id,omitempty"`
	TargetCategoryHint string           `json:"target_category_hint,omitempty"`
	BrandHint          string           `json:"brand_hint,omitempty"`
	Options            *GenerateOptions `json:"options,omitempty"`
}

type GenerateOptions struct {
	ProcessImages bool `json:"process_images"`
}

type Task struct {
	ID         string            `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Request    *GenerateRequest  `json:"request" gorm:"type:text"`
	Status     TaskStatus        `json:"status" gorm:"type:varchar(20);index"`
	Result     *ListingKitResult `json:"result,omitempty" gorm:"type:text"`
	Error      string            `json:"error,omitempty" gorm:"type:text"`
	CreatedAt  time.Time         `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time         `json:"updated_at" gorm:"autoUpdateTime"`
	RetryCount int               `json:"retry_count" gorm:"default:0"`
}

type TaskResult struct {
	TaskID      string            `json:"task_id"`
	Status      TaskStatus        `json:"status"`
	Result      *ListingKitResult `json:"result,omitempty"`
	Error       string            `json:"error,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
}

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
	TaskID                   string     `json:"task_id,omitempty"`
	GenerationTask           string     `json:"generation_task,omitempty"`
	Platform                 string     `json:"platform,omitempty"`
	Slot                     string     `json:"slot,omitempty"`
	Purpose                  string     `json:"purpose,omitempty"`
	IdealKind                string     `json:"ideal_kind,omitempty"`
	State                    string     `json:"state,omitempty"`
	SatisfiedBy              string     `json:"satisfied_by,omitempty"`
	IsFallback               bool       `json:"is_fallback,omitempty"`
	Retryable                bool       `json:"retryable,omitempty"`
	RecipeID                 string     `json:"recipe_id,omitempty"`
	TemplateLabel            string     `json:"template_label,omitempty"`
	RenderProfile            string     `json:"render_profile,omitempty"`
	AssetID                  string     `json:"asset_id,omitempty"`
	ExecutionMode            string     `json:"execution_mode,omitempty"`
	ExecutionState           string     `json:"execution_status,omitempty"`
	RetryHint                string     `json:"retry_hint,omitempty"`
	StateReason              string     `json:"state_reason,omitempty"`
	SelectedAssetID          string     `json:"selected_asset_id,omitempty"`
	TargetAssetKind          string     `json:"target_asset_kind,omitempty"`
	ExecutionQuality         string     `json:"execution_quality,omitempty"`
	ExecutionQualityLabel    string     `json:"execution_quality_label,omitempty"`
	QualityGrade             string     `json:"quality_grade,omitempty"`
	QualityGradeLabel        string     `json:"quality_grade_label,omitempty"`
	RenderPreviewAvailable   bool       `json:"render_preview_available,omitempty"`
	RenderPreviewFormat      string     `json:"render_preview_format,omitempty"`
	RenderPreviewVisualMode  string     `json:"render_preview_visual_mode,omitempty"`
	RenderPreviewLayerTypes  []string   `json:"render_preview_layer_types,omitempty"`
	RenderPreviewRegions     []string   `json:"render_preview_regions,omitempty"`
	RenderPreviewStyleTokens []string   `json:"render_preview_style_tokens,omitempty"`
	PreviewCapabilities      []string   `json:"preview_capabilities,omitempty"`
	ReviewDecision           string     `json:"review_decision,omitempty"`
	ReviewStatus             string     `json:"review_status,omitempty"`
	ReviewBlocked            bool       `json:"review_blocked,omitempty"`
	ReviewedAt               *time.Time `json:"reviewed_at,omitempty"`
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

type ListingKitResult struct {
	TaskID                      string                           `json:"task_id"`
	Status                      string                           `json:"status"`
	Platforms                   []string                         `json:"platforms,omitempty"`
	Country                     string                           `json:"country,omitempty"`
	Language                    string                           `json:"language,omitempty"`
	CatalogProduct              *catalog.Product                 `json:"catalog_product,omitempty"`
	AssetBundle                 *asset.Bundle                    `json:"asset_bundle,omitempty"`
	AssetInventorySummary       *asset.InventorySummary          `json:"asset_inventory_summary,omitempty"`
	AssetRenderPreviews         []AssetRenderPreview             `json:"asset_render_previews,omitempty"`
	PlatformAssetRenderPreviews []PlatformAssetRenderPreviews    `json:"platform_asset_render_previews,omitempty"`
	AssetGenerationSummary      *AssetGenerationSummary          `json:"asset_generation_summary,omitempty"`
	AssetGenerationTasks        []assetgeneration.Task           `json:"asset_generation_tasks,omitempty"`
	AssetGenerationQueue        *GenerationWorkQueue             `json:"asset_generation_queue,omitempty"`
	AssetGenerationOverview     *AssetGenerationOverview         `json:"asset_generation_overview,omitempty"`
	ReviewSummary               *GenerationReviewSummary         `json:"review_summary,omitempty"`
	ReviewRecords               []GenerationReviewRecord         `json:"review_records,omitempty"`
	CanonicalProduct            *productenrich.CanonicalProduct  `json:"canonical_product,omitempty"`
	ImageAssets                 *productimage.ImageProcessResult `json:"image_assets,omitempty"`
	Amazon                      *AmazonPackage                   `json:"amazon,omitempty"`
	Shein                       *sheinpub.Package                `json:"shein,omitempty"`
	Temu                        *TemuPackage                     `json:"temu,omitempty"`
	Walmart                     *WalmartPackage                  `json:"walmart,omitempty"`
	Summary                     *GenerationSummary               `json:"summary,omitempty"`
	Revision                    *ListingKitRevisionSummary       `json:"revision,omitempty"`
	RevisionHistoryTotal        int                              `json:"revision_history_total,omitempty"`
	RevisionHistory             []ListingKitRevisionRecord       `json:"revision_history,omitempty"`
	ChildTasks                  []ChildTaskState                 `json:"child_tasks,omitempty"`
	CreatedAt                   time.Time                        `json:"created_at"`
	UpdatedAt                   time.Time                        `json:"updated_at"`
}

type GenerationSummary struct {
	SourceType   string   `json:"source_type,omitempty"`
	ImageCount   int      `json:"image_count"`
	VariantCount int      `json:"variant_count"`
	NeedsReview  bool     `json:"needs_review"`
	Warnings     []string `json:"warnings,omitempty"`
}

type GenerationRecoverySummary struct {
	Title                  string                              `json:"title,omitempty"`
	Summary                string                              `json:"summary,omitempty"`
	Severity               string                              `json:"severity,omitempty"`
	Urgency                string                              `json:"urgency,omitempty"`
	CTAKind                string                              `json:"cta_kind,omitempty"`
	ActionKey              string                              `json:"action_key,omitempty"`
	RecommendedCount       int                                 `json:"recommended_count"`
	PrimaryDescriptor      *GenerationPanelResourceDescriptor  `json:"primary_descriptor,omitempty"`
	RecommendedDescriptors []GenerationPanelResourceDescriptor `json:"recommended_descriptors,omitempty"`
}

type GenerationResolvedActionSummary struct {
	SourceKind       string                            `json:"source_kind,omitempty"`
	Title            string                            `json:"title,omitempty"`
	Summary          string                            `json:"summary,omitempty"`
	CTAKind          string                            `json:"cta_kind,omitempty"`
	ActionKey        string                            `json:"action_key,omitempty"`
	NavigationTarget *GenerationReviewNavigationTarget `json:"navigation_target,omitempty"`
	ActionTarget     *AssetGenerationActionTarget      `json:"action_target,omitempty"`
	RecoverySummary  *GenerationRecoverySummary        `json:"recovery_summary,omitempty"`
}

type AmazonPackage struct {
	Draft       *amazonlisting.AmazonListingDraft `json:"draft,omitempty"`
	ImageBundle *common.PublishImageBundle        `json:"image_bundle,omitempty"`
}

type TemuPackage struct {
	GoodsName          string                     `json:"goods_name,omitempty"`
	CategoryPath       []string                   `json:"category_path,omitempty"`
	ShortDescription   string                     `json:"short_description,omitempty"`
	BulletPoints       []string                   `json:"bullet_points,omitempty"`
	Attributes         map[string]string          `json:"attributes,omitempty"`
	SkcList            []TemuSKCPackage           `json:"skc_list,omitempty"`
	BatchSkuInfo       *TemuBatchSKUInfo          `json:"batch_sku_info,omitempty"`
	Images             *PlatformImageSet          `json:"images,omitempty"`
	ImageBundle        *common.PublishImageBundle `json:"image_bundle,omitempty"`
	Metadata           map[string]string          `json:"metadata,omitempty"`
	CategoryDisclaimer []string                   `json:"category_disclaimer,omitempty"`
	ReviewNotes        []string                   `json:"review_notes,omitempty"`
}

type WalmartPackage struct {
	ProductName      string                     `json:"product_name,omitempty"`
	Brand            string                     `json:"brand,omitempty"`
	ProductType      string                     `json:"product_type,omitempty"`
	ShortDescription string                     `json:"short_description,omitempty"`
	LongDescription  string                     `json:"long_description,omitempty"`
	KeyFeatures      []string                   `json:"key_features,omitempty"`
	Attributes       map[string]string          `json:"attributes,omitempty"`
	Variants         []PlatformVariant          `json:"variants,omitempty"`
	Images           *PlatformImageSet          `json:"images,omitempty"`
	ImageBundle      *common.PublishImageBundle `json:"image_bundle,omitempty"`
	Metadata         map[string]string          `json:"metadata,omitempty"`
	ReviewNotes      []string                   `json:"review_notes,omitempty"`
}

type PlatformVariant = common.Variant
type PlatformPrice = common.Price
type PlatformImageSet = common.ImageSet
type PlatformAttribute = common.Attribute
type PlatformSite = common.Site

type SheinSKCPackage = sheinpub.SKCPackage

type TemuSKCPackage struct {
	Priority        int               `json:"priority,omitempty"`
	ColorImageURL   string            `json:"color_image_url,omitempty"`
	Spec            []TemuSpecPackage `json:"spec,omitempty"`
	CarouselGallery []string          `json:"carousel_gallery,omitempty"`
	SKUs            []PlatformVariant `json:"skus,omitempty"`
}

type TemuSpecPackage struct {
	Name       string `json:"name,omitempty"`
	Value      string `json:"value,omitempty"`
	ParentName string `json:"parent_name,omitempty"`
}

type TemuBatchSKUInfo struct {
	Currency  string `json:"currency,omitempty"`
	Quantity  string `json:"quantity,omitempty"`
	OutSkuSN  string `json:"out_sku_sn,omitempty"`
	Weight    string `json:"weight,omitempty"`
	Length    string `json:"length,omitempty"`
	Width     string `json:"width,omitempty"`
	Height    string `json:"height,omitempty"`
	Price     string `json:"price,omitempty"`
	CostPrice string `json:"cost_price,omitempty"`
}

func (r GenerateRequest) Value() (driver.Value, error) { return json.Marshal(r) }

func (r *GenerateRequest) Scan(value any) error {
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, r)
}

func (r ListingKitResult) Value() (driver.Value, error) { return json.Marshal(r) }

func (r *ListingKitResult) Scan(value any) error {
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, r)
}
