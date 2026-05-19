package temporal

import (
	"time"

	"task-processor/internal/listingkit"
)

// SheinPublishWorkflowInput carries the input contract for the SHEIN submit
// publish workflow PoC.
type SheinPublishWorkflowInput struct {
	TaskID          string    `json:"task_id"`
	Platform        string    `json:"platform"`
	Action          string    `json:"action"`
	RequestID       string    `json:"request_id"`
	ConfirmedFinal  bool      `json:"confirmed_final"`
	RequestedAt     time.Time `json:"requested_at"`
	TriggeredByUser string    `json:"triggered_by_user,omitempty"`
}

// SheinPublishStateQueryResult is returned by the SHEIN publish state query.
type SheinPublishStateQueryResult struct {
	TaskID          string     `json:"task_id"`
	Action          string     `json:"action"`
	RequestID       string     `json:"request_id,omitempty"`
	CurrentPhase    string     `json:"current_phase,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	RemoteStatus    string     `json:"remote_status,omitempty"`
	WorkflowRunning bool       `json:"workflow_running"`
}

// StandardProductWorkflowInput is the future Temporal contract boundary for
// the standard product layer. It intentionally carries the original generate
// request and leaves platform-specific adaptation to later workflows.
type StandardProductWorkflowInput struct {
	TaskID          string                      `json:"task_id"`
	Request         *listingkit.GenerateRequest `json:"request,omitempty"`
	RequestedAt     time.Time                   `json:"requested_at"`
	TriggeredByUser string                      `json:"triggered_by_user,omitempty"`
}

// StandardProductWorkflowResult persists the standard layer output that later
// platform adaptation workflows can resume from without re-running upstream
// canonical/SDS generation.
type StandardProductWorkflowResult struct {
	TaskID    string                              `json:"task_id"`
	Snapshot  *listingkit.StandardProductSnapshot `json:"snapshot,omitempty"`
	Completed bool                                `json:"completed"`
}

// PlatformAdaptWorkflowInput is the generic boundary for platform-specific
// adaptation. Concrete platform workflows should consume the persisted
// standard-product snapshot instead of re-running canonical generation.
type PlatformAdaptWorkflowInput struct {
	TaskID          string                              `json:"task_id"`
	Platform        string                              `json:"platform"`
	Snapshot        *listingkit.StandardProductSnapshot `json:"snapshot,omitempty"`
	RequestedAt     time.Time                           `json:"requested_at"`
	TriggeredByUser string                              `json:"triggered_by_user,omitempty"`
}
