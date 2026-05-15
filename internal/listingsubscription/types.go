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
	StatusActive   = "active"
	StatusTrialing = "trialing"
	StatusExpired  = "expired"
	StatusDisabled = "disabled"
)

var (
	ErrModuleNotFound          = errors.New("subscription module not found")
	ErrEntitlementNotFound     = errors.New("subscription entitlement not found")
	ErrSubscriptionRequired    = errors.New("subscription required")
	ErrSubscriptionQuotaExceed = errors.New("subscription quota exceeded")
)

type Module struct {
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	SortOrder   int       `json:"sort_order"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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
	TenantID     string            `json:"tenant_id"`
	Modules      []Module          `json:"modules"`
	Entitlements []EntitlementView `json:"entitlements"`
}

type TenantOverview struct {
	TenantID         string     `json:"tenant_id"`
	EntitlementCount int        `json:"entitlement_count"`
	ActiveCount      int        `json:"active_count"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
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
	GetEntitlement(ctx context.Context, tenantID, moduleCode string) (*Entitlement, error)
	ListEntitlements(ctx context.Context, tenantID string) ([]Entitlement, error)
	ListTenantOverviews(ctx context.Context) ([]TenantOverview, error)
	UpsertEntitlement(ctx context.Context, entitlement *Entitlement) (*Entitlement, error)
	ListUsage(ctx context.Context, tenantID string) ([]UsageCounter, error)
	IncrementUsage(ctx context.Context, tenantID, moduleCode, periodKey, metric string, amount int) (*UsageCounter, error)
	SetUsage(ctx context.Context, tenantID, moduleCode, periodKey, metric string, used int) (*UsageCounter, error)
	CreateAuditLog(ctx context.Context, log AuditLog) (*AuditLog, error)
	ListAuditLogs(ctx context.Context, tenantID string, limit int) ([]AuditLog, error)
}
