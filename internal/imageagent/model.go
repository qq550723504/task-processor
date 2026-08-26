package imageagent

import "time"

type RunMode string

const (
	RunModeManual    RunMode = "manual"
	RunModeAssisted  RunMode = "assisted"
	RunModeAutomatic RunMode = "automatic"
)

type RunStatus string

const (
	RunStatusPlanning              RunStatus = "planning"
	RunStatusAwaitingPlanApproval  RunStatus = "awaiting_plan_approval"
	RunStatusExecuting             RunStatus = "executing"
	RunStatusEvaluating            RunStatus = "evaluating"
	RunStatusRepairing             RunStatus = "repairing"
	RunStatusAwaitingFinalApproval RunStatus = "awaiting_final_approval"
	RunStatusBlocked               RunStatus = "blocked"
	RunStatusCompleted             RunStatus = "completed"
	RunStatusFailed                RunStatus = "failed"
	RunStatusCancelled             RunStatus = "cancelled"
)

type SlotStatus string

const (
	SlotStatusPending    SlotStatus = "pending"
	SlotStatusExecuting  SlotStatus = "executing"
	SlotStatusEvaluating SlotStatus = "evaluating"
	SlotStatusAccepted   SlotStatus = "accepted"
	SlotStatusRejected   SlotStatus = "rejected"
	SlotStatusBlocked    SlotStatus = "blocked"
)

type SlotRole string

const (
	SlotRoleMain         SlotRole = "main"
	SlotRoleScene        SlotRole = "scene"
	SlotRoleDetail       SlotRole = "detail"
	SlotRoleSellingPoint SlotRole = "selling_point"
	SlotRoleSize         SlotRole = "size"
)

type Slot struct {
	ID                string
	Role              SlotRole
	SourceAssetIDs    []string
	StyleReferenceIDs []string
	Brief             string
	IdempotencyKey    string
	Status            SlotStatus
}

type Plan struct {
	Revision          int64
	ParentRevision    int64
	IdempotencyKey    string
	SourceAssetIDs    []string
	StyleReferenceIDs []string
	Slots             []Slot
	CreatedBy         string
}

type Run struct {
	ID                 string
	BusinessTaskID     string
	TenantID           string
	UserID             string
	Mode               RunMode
	IdempotencyKey     string
	Status             RunStatus
	CurrentNode        string
	ActivePlanRevision int64
	Version            int64
	Budget             Budget
	Usage              BudgetUsage
	Block              *Block
}

type Block struct {
	Code    string
	Message string
	SlotID  string
}

type AssetRef struct {
	ID   string
	URL  string
	Kind string
}

type AuthorizedAssetType string

const (
	AuthorizedAssetSource AuthorizedAssetType = "source"
	AuthorizedAssetStyle  AuthorizedAssetType = "style"
)

// AuthorizedAsset is a server-resolved, run-scoped selectable input. It is
// deliberately separate from generated candidates.
type AuthorizedAsset struct {
	ID         string
	Type       AuthorizedAssetType
	DisplayURL string
	Label      string
	Width      int
	Height     int
}

type AssetCatalog struct {
	Assets []AuthorizedAsset
}

type PendingCommandReceipt struct {
	ActionID     string
	Kind         string
	Phase        string
	Status       string
	PlanRevision int64
	SlotID       string
}

type ProductContextRef struct {
	ProductID   string
	Title       string
	ProductType string
	Attributes  map[string]string
}

type AssetCandidate struct {
	AssetID       string
	URL           string
	SourceAssetID string
	Metadata      map[string]string
}

type Budget struct {
	MaxImages                int
	MaxAgentSteps            int
	MaxModelCalls            int
	MaxRepairAttemptsPerSlot int
	MaxCostMicros            int64
	MaxElapsed               time.Duration
}

type BudgetUsage struct {
	Images              int
	AgentSteps          int
	ModelCalls          int
	EstimatedCostMicros int64
	Elapsed             time.Duration
}
