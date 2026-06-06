package listingkit

import (
	"time"

	assetgeneration "task-processor/internal/asset/generation"
	listinggeneration "task-processor/internal/listingkit/generation"
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

func buildAssetGenerationSummary(tasks []assetgeneration.Task) *AssetGenerationSummary {
	stats := listinggeneration.SummarizeTasks(tasks)
	return &AssetGenerationSummary{
		TotalTasks:          stats.TotalTasks,
		PlannedTasks:        stats.PlannedTasks,
		CompletedTasks:      stats.CompletedTasks,
		FailedTasks:         stats.FailedTasks,
		RendererBackedTasks: stats.RendererBackedTasks,
		FallbackTasks:       stats.FallbackTasks,
		RetryableTasks:      stats.RetryableTasks,
		Platforms:           stats.Platforms,
	}
}
