package listingkit

import (
	"context"
	"time"

	"task-processor/internal/shared/tenantctx"

	"gorm.io/gorm"
)

type StudioBatchRunStatus string

const (
	StudioBatchRunStatusPending            StudioBatchRunStatus = "pending"
	StudioBatchRunStatusRunning            StudioBatchRunStatus = "running"
	StudioBatchRunStatusSucceeded          StudioBatchRunStatus = "succeeded"
	StudioBatchRunStatusPartiallySucceeded StudioBatchRunStatus = "partially_succeeded"
	StudioBatchRunStatusFailed             StudioBatchRunStatus = "failed"
	StudioBatchRunStatusCancelled          StudioBatchRunStatus = "cancelled"
)

type StudioBatchRunItemStatus string

const (
	StudioBatchRunItemStatusPending   StudioBatchRunItemStatus = "pending"
	StudioBatchRunItemStatusRunning   StudioBatchRunItemStatus = "running"
	StudioBatchRunItemStatusSucceeded StudioBatchRunItemStatus = "succeeded"
	StudioBatchRunItemStatusFailed    StudioBatchRunItemStatus = "failed"
	StudioBatchRunItemStatusCancelled StudioBatchRunItemStatus = "cancelled"
)

type StudioBatchRunMode string

const (
	StudioBatchRunModeGenerate    StudioBatchRunMode = "generate"
	StudioBatchRunModeCreateTasks StudioBatchRunMode = "create_tasks"
)

type StudioBatchRunFailurePolicy string

const (
	StudioBatchRunFailurePolicyContinueOnError StudioBatchRunFailurePolicy = "continue_on_error"
	StudioBatchRunFailurePolicyStopOnError     StudioBatchRunFailurePolicy = "stop_on_error"
)

type StudioBatchRunRecord struct {
	ID               string                      `json:"id" gorm:"primaryKey;type:varchar(64)"`
	TenantID         string                      `json:"-" gorm:"type:varchar(64);index"`
	UserID           string                      `json:"-" gorm:"type:varchar(128);index"`
	Mode             StudioBatchRunMode          `json:"mode" gorm:"type:varchar(32);not null"`
	FailurePolicy    StudioBatchRunFailurePolicy `json:"failure_policy" gorm:"type:varchar(32);not null"`
	Status           StudioBatchRunStatus        `json:"status" gorm:"type:varchar(32);index;not null"`
	CurrentBatchID   string                      `json:"current_batch_id,omitempty" gorm:"type:varchar(64);index"`
	CurrentIndex     int                         `json:"current_index" gorm:"not null;default:0"`
	TotalBatches     int                         `json:"total_batches" gorm:"not null;default:0"`
	CompletedBatches int                         `json:"completed_batches" gorm:"not null;default:0"`
	SucceededBatches int                         `json:"succeeded_batches" gorm:"not null;default:0"`
	FailedBatches    int                         `json:"failed_batches" gorm:"not null;default:0"`
	LastError        string                      `json:"last_error,omitempty" gorm:"type:text"`
	CancelRequested  bool                        `json:"cancel_requested" gorm:"not null;default:false"`
	StartedAt        *time.Time                  `json:"started_at,omitempty"`
	FinishedAt       *time.Time                  `json:"finished_at,omitempty"`
	CreatedAt        time.Time                   `json:"created_at"`
	UpdatedAt        time.Time                   `json:"updated_at"`
}

func (StudioBatchRunRecord) TableName() string {
	return "listingkit_studio_batch_runs"
}

type StudioBatchRunItemRecord struct {
	ID           string                   `json:"id" gorm:"primaryKey;type:varchar(96)"`
	TenantID     string                   `json:"-" gorm:"type:varchar(64);index"`
	UserID       string                   `json:"-" gorm:"type:varchar(128);index"`
	RunID        string                   `json:"run_id" gorm:"type:varchar(64);index:idx_listingkit_studio_batch_run_items_run_position,priority:1"`
	BatchID      string                   `json:"batch_id" gorm:"type:varchar(64);index"`
	Position     int                      `json:"position" gorm:"index:idx_listingkit_studio_batch_run_items_run_position,priority:2"`
	Status       StudioBatchRunItemStatus `json:"status" gorm:"type:varchar(32);index;not null"`
	SessionID    string                   `json:"session_id,omitempty" gorm:"type:varchar(64);index"`
	AsyncJobID   string                   `json:"async_job_id,omitempty" gorm:"type:varchar(64);index"`
	ErrorMessage string                   `json:"error_message,omitempty" gorm:"type:text"`
	StartedAt    *time.Time               `json:"started_at,omitempty"`
	FinishedAt   *time.Time               `json:"finished_at,omitempty"`
	CreatedAt    time.Time                `json:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"`
}

func (StudioBatchRunItemRecord) TableName() string {
	return "listingkit_studio_batch_run_items"
}

type StudioBatchRunRepository interface {
	CreateStudioBatchRun(ctx context.Context, run *StudioBatchRunRecord, items []StudioBatchRunItemRecord) error
	GetStudioBatchRun(ctx context.Context, runID string) (*StudioBatchRunRecord, error)
	ListUnfinishedStudioBatchRuns(ctx context.Context) ([]StudioBatchRunRecord, error)
	ListStudioBatchRunItems(ctx context.Context, runID string) ([]StudioBatchRunItemRecord, error)
	ListStudioBatchRunItemsByBatchID(ctx context.Context, batchID string) ([]StudioBatchRunItemRecord, error)
	UpdateStudioBatchRun(ctx context.Context, run *StudioBatchRunRecord) error
	UpdateStudioBatchRunItem(ctx context.Context, item *StudioBatchRunItemRecord) error
}

func applyStudioBatchRunScopeDefaults(ctx context.Context, tenantID *string, userID *string) {
	if tenantID != nil && *tenantID == "" {
		*tenantID = tenantctx.TenantIDFromContext(ctx)
	}
	if userID != nil && *userID == "" {
		*userID = RequestUserIDFromContext(ctx)
	}
}

func applyStudioBatchRunAccessScope(db *gorm.DB, ctx context.Context) *gorm.DB {
	tenantID, ok := tenantctx.TenantScopeFromContext(ctx)
	if ok {
		if tenantID == tenantctx.DefaultTenantID {
			db = db.Where("(tenant_id = ? OR tenant_id = '' OR tenant_id IS NULL)", tenantID)
		} else {
			db = db.Where("tenant_id = ?", tenantID)
		}
	}
	if OwnerScopeEnabled() {
		if userID := RequestUserIDFromContext(ctx); userID != "" {
			db = db.Where("user_id = ?", userID)
		}
	}
	return db
}

func matchesStudioBatchRunScope(ctx context.Context, tenantID string, userID string) bool {
	if !tenantctx.MatchesTenant(tenantID, tenantctx.TenantIDFromContext(ctx)) {
		return false
	}
	if !OwnerScopeEnabled() {
		return true
	}
	requestUserID := RequestUserIDFromContext(ctx)
	return requestUserID == "" || requestUserID == userID
}
