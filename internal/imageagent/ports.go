package imageagent

import "context"

// ExecutionIdentity is the tenant/user identity verified at the command edge
// and durably captured in workflow input.
type ExecutionIdentity struct {
	TenantID string
	UserID   string
}

type SlotExecutor interface {
	ExecuteSlot(context.Context, SlotExecutionInput) (SlotExecutionResult, error)
}

type ApprovedAssetPublisher interface {
	PublishApproved(context.Context, PublishApprovedInput) error
}

type SlotExecutionInput struct {
	RunID          string
	TenantID       string
	UserID         string
	PlanRevision   int64
	Slot           Slot
	Attempt        int
	IdempotencyKey string
}

type SlotExecutionResult struct {
	SlotID     string
	Attempt    int
	Candidates []AssetCandidate
}

type PublishApprovedInput struct {
	RunID             string
	TenantID          string
	PlanRevision      int64
	CandidateAssetIDs []string
	IdempotencyKey    string
}

type StartRunInput struct {
	RunID              string
	BusinessTaskID     string
	Mode               RunMode
	IdempotencyKey     string
	Plan               Plan
	Budget             Budget
	MaxConcurrentSlots int
}

type WorkflowStart struct {
	Run                Run
	Plan               Plan
	Identity           ExecutionIdentity
	MaxConcurrentSlots int
}

// RunProjection is the complete application snapshot exposed to transports.
// It deliberately contains only image-agent application contracts.
type RunProjection struct {
	Run          Run              `json:"run"`
	Plan         Plan             `json:"plan"`
	Slots        []SlotProjection `json:"slots"`
	ResultDigest string           `json:"result_digest,omitempty"`
	Actions      []Action         `json:"actions"`
	LastEventID  int64            `json:"last_event_id"`
}

type SlotProjection struct {
	Slot       Slot             `json:"slot"`
	Attempt    int              `json:"attempt"`
	Candidates []AssetCandidate `json:"candidates"`
	ErrorCode  string           `json:"error_code,omitempty"`
}

// WorkflowProjection is the workflow-owned portion of the application
// snapshot. Temporal adapters map query results into this contract.
type WorkflowProjection struct {
	Status           RunStatus        `json:"status"`
	Block            *Block           `json:"block,omitempty"`
	Plan             Plan             `json:"plan"`
	Slots            []SlotProjection `json:"slots"`
	CompletedSlotIDs []string         `json:"completed_slot_ids"`
	ResultDigest     string           `json:"result_digest,omitempty"`
}

type ReplacePlanCommand struct {
	RunID            string
	ExpectedRevision int64
	Plan             Plan
	ActorID          string
	ActionID         string
	Identity         ExecutionIdentity
}

type RetrySlotCommand struct {
	RunID        string
	PlanRevision int64
	SlotID       string
	ActorID      string
	ActionID     string
	Identity     ExecutionIdentity
}

type ApproveResultsCommand struct {
	RunID        string
	PlanRevision int64
	ResultDigest string
	ActorID      string
	ActionID     string
	Identity     ExecutionIdentity
}

type CancelRunCommand struct {
	RunID        string
	PlanRevision int64
	ActorID      string
	ActionID     string
	Identity     ExecutionIdentity
}

type WorkflowClient interface {
	StartManual(context.Context, WorkflowStart) error
	GetProjection(context.Context, RunScope, ExecutionIdentity) (WorkflowProjection, error)
	ReplacePlan(context.Context, ReplacePlanCommand) error
	RetrySlot(context.Context, RetrySlotCommand) error
	ApproveResults(context.Context, ApproveResultsCommand) error
	Cancel(context.Context, CancelRunCommand) error
}
