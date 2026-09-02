package imageagent

import (
	"fmt"
	"time"
)

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

type ImagePolicyContext struct {
	Country       string
	Family        string
	SceneCategory string
}

type Run struct {
	ID             string
	BusinessTaskID string
	// TargetPlatform is the immutable marketplace selected when the run starts.
	TargetPlatform     string
	ImagePolicyContext ImagePolicyContext
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
	MaxConcurrentSlots int
	StartedAt          time.Time
}

const (
	DefaultMaxConcurrentSlots = 4
	MaxConcurrentSlots        = 10
)

func ValidateMaxConcurrentSlots(value int) error {
	if value < 0 || value > MaxConcurrentSlots {
		return fmt.Errorf("%w: max concurrent slots must be zero or between 1 and %d", ErrValidation, MaxConcurrentSlots)
	}
	return nil
}

func NormalizeMaxConcurrentSlots(value int) int {
	if value <= 0 {
		return DefaultMaxConcurrentSlots
	}
	if value > MaxConcurrentSlots {
		return MaxConcurrentSlots
	}
	return value
}

type Block struct {
	Code    string
	Message string
	SlotID  string
}

type RecoverableEffect struct {
	SlotID  string
	Attempt int
	Code    string
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
	URL        string
	SourceURL  string
	DisplayURL string
	Label      string
	Width      int
	Height     int
	Metadata   map[string]string
}

type AssetCatalog struct {
	Manifest       CatalogManifest
	Assets         []AuthorizedAsset
	ProductContext ProductContextRef
}

type CatalogManifest struct {
	Version   int64
	Hash      string
	CreatedAt time.Time
}

type PendingCommandReceipt struct {
	ActionID        string
	Kind            string
	Phase           string
	Status          string
	PlanRevision    int64
	SlotID          string
	FailureCode     string
	FailureCategory string
	FailureMessage  string
	LastFailedAt    *time.Time
	Attempt         int
}

type CommandIngress struct {
	Used      int
	Limit     int
	Exhausted bool
	Reason    string
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
	DurableAsset  DurableAssetIdentity `json:"-"`
}

type Budget struct {
	MaxImages                int
	MaxAgentSteps            int
	MaxModelCalls            int
	MaxRepairAttemptsPerSlot int
	MaxCostMicros            int64
	MaxElapsed               time.Duration
	EnabledLimits            BudgetLimitSet
}

type BudgetUsage struct {
	Images              int
	AgentSteps          int
	ModelCalls          int
	EstimatedCostMicros int64
	Elapsed             time.Duration
}
