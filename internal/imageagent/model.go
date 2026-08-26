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
	ID                string     `json:"id"`
	Role              SlotRole   `json:"role"`
	SourceAssetIDs    []string   `json:"source_asset_ids"`
	StyleReferenceIDs []string   `json:"style_reference_ids,omitempty"`
	Brief             string     `json:"brief,omitempty"`
	IdempotencyKey    string     `json:"idempotency_key"`
	Status            SlotStatus `json:"status"`
}

type Plan struct {
	Revision          int64    `json:"revision"`
	ParentRevision    int64    `json:"parent_revision"`
	IdempotencyKey    string   `json:"idempotency_key"`
	SourceAssetIDs    []string `json:"source_asset_ids"`
	StyleReferenceIDs []string `json:"style_reference_ids,omitempty"`
	Slots             []Slot   `json:"slots"`
	CreatedBy         string   `json:"created_by,omitempty"`
}

type Run struct {
	ID                 string      `json:"id"`
	BusinessTaskID     string      `json:"business_task_id,omitempty"`
	TenantID           string      `json:"tenant_id"`
	UserID             string      `json:"user_id"`
	Mode               RunMode     `json:"mode"`
	IdempotencyKey     string      `json:"idempotency_key"`
	Status             RunStatus   `json:"status"`
	CurrentNode        string      `json:"current_node"`
	ActivePlanRevision int64       `json:"active_plan_revision"`
	Version            int64       `json:"version"`
	Budget             Budget      `json:"budget"`
	Usage              BudgetUsage `json:"usage"`
	Block              *Block      `json:"block,omitempty"`
}

type Block struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	SlotID  string `json:"slot_id,omitempty"`
}

type AssetRef struct {
	ID   string
	URL  string
	Kind string
}

type ProductContextRef struct {
	ProductID   string
	Title       string
	ProductType string
	Attributes  map[string]string
}

type AssetCandidate struct {
	AssetID       string            `json:"asset_id"`
	URL           string            `json:"url"`
	SourceAssetID string            `json:"source_asset_id,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
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
