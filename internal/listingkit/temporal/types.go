package temporal

import "time"

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
