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
	AssetCatalog   AssetCatalog
}

type SlotExecutionResult struct {
	SlotID     string
	Attempt    int
	Candidates []AssetCandidate
}

type PublishApprovedInput struct {
	RunID             string
	TenantID          string
	UserID            string
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
	AssetCatalog       AssetCatalog
}

// RunProjection is the complete application snapshot exposed to transports.
// It deliberately contains only image-agent application contracts.
type RunProjection struct {
	Run               Run
	Plan              Plan
	Slots             []SlotProjection
	ResultDigest      string
	Actions           []Action
	LastEventID       int64
	ProjectionVersion int64
	AssetCatalog      AssetCatalog
	PendingCommand    *PendingCommandReceipt
	CommandIngress    CommandIngress
}

type SlotProjection struct {
	Slot       Slot
	Attempt    int
	Candidates []AssetCandidate
	ErrorCode  string
}

// WorkflowProjection is the workflow-owned portion of the application
// snapshot. Temporal adapters map query results into this contract.
type WorkflowProjection struct {
	Status           RunStatus
	Block            *Block
	Plan             Plan
	Slots            []SlotProjection
	CompletedSlotIDs []string
	ResultDigest     string
	PendingCommand   *PendingCommandReceipt
	CommandIngress   CommandIngress
}

type AssetCatalogScope struct {
	TenantID       string
	OwnerUserID    string
	BusinessTaskID string
	RunID          string
}

type AuthorizedAssetCatalog interface {
	Resolve(context.Context, AssetCatalogScope) (AssetCatalog, error)
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

type ResumeCommand struct {
	RunID    string
	ActorID  string
	ActionID string
	Identity ExecutionIdentity
}

type CommandAcknowledgement struct {
	RunID        string
	PlanRevision int64
	ActionID     string
	Status       RunStatus
}

type WorkflowClient interface {
	StartManual(context.Context, WorkflowStart) error
	GetProjection(context.Context, RunScope, ExecutionIdentity) (WorkflowProjection, error)
	ReplacePlan(context.Context, ReplacePlanCommand) error
	RetrySlot(context.Context, RetrySlotCommand) error
	ApproveResults(context.Context, ApproveResultsCommand) error
	Cancel(context.Context, CancelRunCommand) error
	Resume(context.Context, ResumeCommand) (CommandAcknowledgement, error)
}
