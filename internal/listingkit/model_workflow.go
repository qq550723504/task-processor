package listingkit

import "time"

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
