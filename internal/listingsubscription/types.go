package listingsubscription

import (
	"context"
	"errors"
	"time"
)

const (
	ModuleStoreManagement   = "store_management"
	ModuleTaskImport        = "task_import"
	ModuleRules             = "rules"
	ModuleOperationStrategy = "operation_strategy"
	ModuleStudio            = "studio"
	ModuleOSSStorage        = "oss_storage"
)

const (
	PlanBasic        = "basic"
	PlanProfessional = "professional"
	PlanEnterprise   = "enterprise"
)

const (
	StatusActive   = "active"
	StatusTrialing = "trialing"
	StatusExpired  = "expired"
	StatusDisabled = "disabled"
)

var (
	ErrModuleNotFound            = errors.New("subscription module not found")
	ErrEntitlementNotFound       = errors.New("subscription entitlement not found")
	ErrSubscriptionRequired      = errors.New("subscription required")
	ErrSubscriptionQuotaExceed   = errors.New("subscription quota exceeded")
	ErrUsageInvalidInput         = errors.New("usage ledger invalid input")
	ErrUsageDuplicateIdentity    = errors.New("usage ledger duplicate identity")
	ErrUsageInvalidTransition    = errors.New("usage ledger invalid transition")
	ErrUsageQuotaExceeded        = errors.New("usage ledger quota exceeded")
	ErrUsageLedgerNotConfigured  = errors.New("usage ledger is not configured")
	ErrUsageOutboxUnsafeMetadata = errors.New("usage outbox metadata is unsafe")
	ErrUsageOutboxStorageSnapshotRequired = errors.New("usage outbox storage snapshot is required")
)

type UsageEventStatus string

const (
	UsageEventReserved  UsageEventStatus = "reserved"
	UsageEventCommitted UsageEventStatus = "committed"
	UsageEventReleased  UsageEventStatus = "released"
	UsageEventReversed  UsageEventStatus = "reversed"
)

type UsageEvent struct {
	EventID        string
	TenantID       string
	ModuleCode     string
	Metric         string
	Quantity       int64
	PeriodKey      string
	SourceType     string
	SourceID       string
	IdempotencyKey string
	Status         UsageEventStatus
	OccurredAt     time.Time
	ReversalOf     string
	Metadata       map[string]string
	// StorageSnapshot is the post-commit retained-byte gauge used only for
	// storage_bytes_current outbox projection. It is derived from the ledger
	// bucket and is intentionally not caller-controlled input.
	StorageSnapshot *int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ReserveUsageInput struct {
	TenantID       string
	ModuleCode     string
	Metric         string
	Quantity       int64
	PeriodKey      string
	SourceType     string
	SourceID       string
	IdempotencyKey string
	OccurredAt     time.Time
	Metadata       map[string]string
}

type ReserveUsageResult struct {
	Event          UsageEvent
	Existing       bool
	CommittedUsage int64
	ReservedUsage  int64
	Limit          *int64
}

type UsageOutboxItem struct {
	ID            int64
	EventID       string
	Destination   string
	Status        string
	Attempts      int
	NextAttemptAt *time.Time
	LastError     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// OpenMeterUsageOutboxPayload is the redacted payload boundary for an
// asynchronous OpenMeter projection. It intentionally contains no request
// bodies, credentials, authorization headers, or provider configuration.
type OpenMeterUsageOutboxPayload struct {
	EventID    string
	TenantID   string
	Metric     string
	Quantity   int64
	OccurredAt time.Time
	Metadata   map[string]string
}

type Module struct {
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	SortOrder   int       `json:"sort_order"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Plan struct {
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	SortOrder   int       `json:"sort_order"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PlanModule struct {
	PlanCode   string         `json:"plan_code"`
	ModuleCode string         `json:"module_code"`
	Limits     map[string]int `json:"limits,omitempty"`
	SortOrder  int            `json:"sort_order"`
}

type PlanBundle struct {
	Plan    Plan         `json:"plan"`
	Modules []PlanModule `json:"modules"`
}

type TenantSubscription struct {
	ID        int64      `json:"id"`
	TenantID  string     `json:"tenant_id"`
	PlanCode  string     `json:"plan_code"`
	Status    string     `json:"status"`
	StartsAt  *time.Time `json:"starts_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type Entitlement struct {
	ID         int64          `json:"id"`
	TenantID   string         `json:"tenant_id"`
	ModuleCode string         `json:"module_code"`
	Status     string         `json:"status"`
	StartsAt   *time.Time     `json:"starts_at,omitempty"`
	ExpiresAt  *time.Time     `json:"expires_at,omitempty"`
	Limits     map[string]int `json:"limits,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type UsageCounter struct {
	ID         int64     `json:"id"`
	TenantID   string    `json:"tenant_id"`
	ModuleCode string    `json:"module_code"`
	PeriodKey  string    `json:"period_key"`
	Metric     string    `json:"metric"`
	Used       int       `json:"used"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type AuditLog struct {
	ID         int64     `json:"id"`
	TenantID   string    `json:"tenant_id"`
	ModuleCode string    `json:"module_code,omitempty"`
	Action     string    `json:"action"`
	ActorID    string    `json:"actor_id,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	Payload    string    `json:"payload,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type EntitlementInput struct {
	Status    string         `json:"status"`
	StartsAt  *time.Time     `json:"starts_at,omitempty"`
	ExpiresAt *time.Time     `json:"expires_at,omitempty"`
	Limits    map[string]int `json:"limits,omitempty"`
}

type UsageAdjustmentInput struct {
	PeriodKey string `json:"period_key"`
	Metric    string `json:"metric"`
	Used      int    `json:"used"`
	Reason    string `json:"reason,omitempty"`
}

type PlanApplyInput struct {
	PlanCode  string     `json:"plan_code"`
	Status    string     `json:"status"`
	StartsAt  *time.Time `json:"starts_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type PlanInput struct {
	Code        string            `json:"code"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	SortOrder   int               `json:"sort_order"`
	Active      bool              `json:"active"`
	Modules     []PlanModuleInput `json:"modules,omitempty"`
}

type PlanModuleInput struct {
	ModuleCode string         `json:"module_code,omitempty"`
	Limits     map[string]int `json:"limits,omitempty"`
	SortOrder  int            `json:"sort_order"`
}

type EntitlementView struct {
	Module      Module         `json:"module"`
	Entitlement *Entitlement   `json:"entitlement,omitempty"`
	Usage       []UsageCounter `json:"usage"`
	Allowed     bool           `json:"allowed"`
	Reason      string         `json:"reason,omitempty"`
	Limits      map[string]int `json:"limits,omitempty"`
	Used        map[string]int `json:"used,omitempty"`
}

type Summary struct {
	TenantID     string              `json:"tenant_id"`
	Modules      []Module            `json:"modules"`
	Entitlements []EntitlementView   `json:"entitlements"`
	Subscription *TenantSubscription `json:"subscription,omitempty"`
	CurrentPlan  *PlanBundle         `json:"current_plan,omitempty"`
}

type TenantOverview struct {
	TenantID          string     `json:"tenant_id"`
	TenantDisplayName string     `json:"tenant_display_name,omitempty"`
	EntitlementCount  int        `json:"entitlement_count"`
	ActiveCount       int        `json:"active_count"`
	UpdatedAt         *time.Time `json:"updated_at,omitempty"`
}

type GuardResult struct {
	Allowed    bool
	Reason     string
	ModuleCode string
	Metric     string
	Limit      int
	Used       int
}

type Repository interface {
	ListModules(ctx context.Context) ([]Module, error)
	UpsertDefaultModules(ctx context.Context, modules []Module) error
	ListPlans(ctx context.Context) ([]PlanBundle, error)
	UpsertDefaultPlans(ctx context.Context, plans []PlanBundle) error
	UpsertPlan(ctx context.Context, plan Plan, modules []PlanModule) (*PlanBundle, error)
	UpsertPlanModule(ctx context.Context, module PlanModule) (*PlanBundle, error)
	DeletePlanModule(ctx context.Context, planCode, moduleCode string) (*PlanBundle, error)
	GetTenantSubscription(ctx context.Context, tenantID string) (*TenantSubscription, error)
	ListTenantSubscriptionsByPlan(ctx context.Context, planCode string) ([]TenantSubscription, error)
	UpsertTenantSubscription(ctx context.Context, subscription *TenantSubscription) (*TenantSubscription, error)
	GetEntitlement(ctx context.Context, tenantID, moduleCode string) (*Entitlement, error)
	ListEntitlements(ctx context.Context, tenantID string) ([]Entitlement, error)
	ListTenantOverviews(ctx context.Context) ([]TenantOverview, error)
	UpsertEntitlement(ctx context.Context, entitlement *Entitlement) (*Entitlement, error)
	ListUsage(ctx context.Context, tenantID string) ([]UsageCounter, error)
	IncrementUsage(ctx context.Context, tenantID, moduleCode, periodKey, metric string, amount int) (*UsageCounter, error)
	SetUsage(ctx context.Context, tenantID, moduleCode, periodKey, metric string, used int) (*UsageCounter, error)
	CreateAuditLog(ctx context.Context, log AuditLog) (*AuditLog, error)
	ListAuditLogs(ctx context.Context, tenantID string, limit int) ([]AuditLog, error)
	ListPlanAuditLogs(ctx context.Context, planCode string, limit int) ([]AuditLog, error)
}

type UsageLedger interface {
	Reserve(ctx context.Context, input ReserveUsageInput) (ReserveUsageResult, error)
	Commit(ctx context.Context, eventID string) (UsageEvent, error)
	Release(ctx context.Context, eventID, reason string) (UsageEvent, error)
	Reverse(ctx context.Context, eventID, idempotencyKey, reason string) (UsageEvent, error)
	Get(ctx context.Context, tenantID, idempotencyKey string) (*UsageEvent, error)
	ListPendingOutbox(ctx context.Context, limit int) ([]UsageOutboxItem, error)
}
