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
	RetrySlot(context.Context, RetrySlotCommand) error
	ApproveResults(context.Context, ApproveResultsCommand) error
	Cancel(context.Context, CancelRunCommand) error
}
