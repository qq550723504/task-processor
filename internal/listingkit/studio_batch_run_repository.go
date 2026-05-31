package listingkit

import (
	"context"
	"slices"
	"sync"
	"time"

	"task-processor/internal/listingkit/tenantctx"

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
	UpdateStudioBatchRun(ctx context.Context, run *StudioBatchRunRecord) error
	UpdateStudioBatchRunItem(ctx context.Context, item *StudioBatchRunItemRecord) error
}

type MemStudioBatchRunRepository struct {
	mu    sync.Mutex
	runs  map[string]StudioBatchRunRecord
	items map[string]StudioBatchRunItemRecord
}

func NewMemStudioBatchRunRepository() *MemStudioBatchRunRepository {
	return &MemStudioBatchRunRepository{
		runs:  map[string]StudioBatchRunRecord{},
		items: map[string]StudioBatchRunItemRecord{},
	}
}

func (r *MemStudioBatchRunRepository) CreateStudioBatchRun(ctx context.Context, run *StudioBatchRunRecord, items []StudioBatchRunItemRecord) error {
	if run == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	runRow := *run
	applyStudioBatchRunScopeDefaults(ctx, &runRow.TenantID, &runRow.UserID)
	r.runs[runRow.ID] = runRow

	for _, item := range items {
		itemRow := item
		itemRow.TenantID = runRow.TenantID
		itemRow.UserID = runRow.UserID
		if itemRow.RunID == "" {
			itemRow.RunID = runRow.ID
		}
		r.items[itemRow.ID] = itemRow
	}
	return nil
}

func (r *MemStudioBatchRunRepository) GetStudioBatchRun(ctx context.Context, runID string) (*StudioBatchRunRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.runs[runID]
	if !ok || !matchesStudioBatchRunScope(ctx, record.TenantID, record.UserID) {
		return nil, gorm.ErrRecordNotFound
	}
	cloned := record
	return &cloned, nil
}

func (r *MemStudioBatchRunRepository) ListUnfinishedStudioBatchRuns(ctx context.Context) ([]StudioBatchRunRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	runs := make([]StudioBatchRunRecord, 0)
	for _, run := range r.runs {
		if !matchesStudioBatchRunScope(ctx, run.TenantID, run.UserID) {
			continue
		}
		if run.Status != StudioBatchRunStatusPending && run.Status != StudioBatchRunStatusRunning {
			continue
		}
		runs = append(runs, run)
	}
	slices.SortStableFunc(runs, func(a, b StudioBatchRunRecord) int {
		if a.CreatedAt.Before(b.CreatedAt) {
			return -1
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return runs, nil
}

func (r *MemStudioBatchRunRepository) ListStudioBatchRunItems(ctx context.Context, runID string) ([]StudioBatchRunItemRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	run, ok := r.runs[runID]
	if !ok || !matchesStudioBatchRunScope(ctx, run.TenantID, run.UserID) {
		return nil, gorm.ErrRecordNotFound
	}

	items := make([]StudioBatchRunItemRecord, 0)
	for _, item := range r.items {
		if item.RunID != runID || !matchesStudioBatchRunScope(ctx, item.TenantID, item.UserID) {
			continue
		}
		items = append(items, item)
	}
	slices.SortStableFunc(items, func(a, b StudioBatchRunItemRecord) int {
		if a.Position < b.Position {
			return -1
		}
		if a.Position > b.Position {
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return items, nil
}

func (r *MemStudioBatchRunRepository) UpdateStudioBatchRun(ctx context.Context, run *StudioBatchRunRecord) error {
	if run == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.runs[run.ID]
	if !ok || !matchesStudioBatchRunScope(ctx, existing.TenantID, existing.UserID) {
		return gorm.ErrRecordNotFound
	}
	row := *run
	if row.TenantID == "" {
		row.TenantID = existing.TenantID
	}
	if row.UserID == "" {
		row.UserID = existing.UserID
	}
	r.runs[row.ID] = row
	return nil
}

func (r *MemStudioBatchRunRepository) UpdateStudioBatchRunItem(ctx context.Context, item *StudioBatchRunItemRecord) error {
	if item == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.items[item.ID]
	if !ok || !matchesStudioBatchRunScope(ctx, existing.TenantID, existing.UserID) {
		return gorm.ErrRecordNotFound
	}
	row := *item
	if row.TenantID == "" {
		row.TenantID = existing.TenantID
	}
	if row.UserID == "" {
		row.UserID = existing.UserID
	}
	if row.RunID == "" {
		row.RunID = existing.RunID
	}
	r.items[row.ID] = row
	return nil
}

type GormStudioBatchRunRepository struct {
	db *gorm.DB
}

func NewGormStudioBatchRunRepository(db *gorm.DB) *GormStudioBatchRunRepository {
	return &GormStudioBatchRunRepository{db: db}
}

func AutoMigrateStudioBatchRunRepository(db *gorm.DB) error {
	return db.AutoMigrate(&StudioBatchRunRecord{}, &StudioBatchRunItemRecord{})
}

func (r *GormStudioBatchRunRepository) CreateStudioBatchRun(ctx context.Context, run *StudioBatchRunRecord, items []StudioBatchRunItemRecord) error {
	if run == nil {
		return nil
	}

	runRow := *run
	applyStudioBatchRunScopeDefaults(ctx, &runRow.TenantID, &runRow.UserID)

	itemRows := make([]StudioBatchRunItemRecord, 0, len(items))
	for _, item := range items {
		itemRow := item
		itemRow.TenantID = runRow.TenantID
		itemRow.UserID = runRow.UserID
		if itemRow.RunID == "" {
			itemRow.RunID = runRow.ID
		}
		itemRows = append(itemRows, itemRow)
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&runRow).Error; err != nil {
			return err
		}
		if len(itemRows) == 0 {
			return nil
		}
		return tx.Create(&itemRows).Error
	})
}

func (r *GormStudioBatchRunRepository) GetStudioBatchRun(ctx context.Context, runID string) (*StudioBatchRunRecord, error) {
	var record StudioBatchRunRecord
	err := applyStudioBatchRunAccessScope(r.db.WithContext(ctx), ctx).
		Where("id = ?", runID).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *GormStudioBatchRunRepository) ListUnfinishedStudioBatchRuns(ctx context.Context) ([]StudioBatchRunRecord, error) {
	var runs []StudioBatchRunRecord
	if err := applyStudioBatchRunAccessScope(r.db.WithContext(ctx), ctx).
		Where("status IN ?", []StudioBatchRunStatus{
			StudioBatchRunStatusPending,
			StudioBatchRunStatusRunning,
		}).
		Order("created_at ASC, id ASC").
		Find(&runs).Error; err != nil {
		return nil, err
	}
	return runs, nil
}

func (r *GormStudioBatchRunRepository) ListStudioBatchRunItems(ctx context.Context, runID string) ([]StudioBatchRunItemRecord, error) {
	if _, err := r.GetStudioBatchRun(ctx, runID); err != nil {
		return nil, err
	}

	var items []StudioBatchRunItemRecord
	if err := applyStudioBatchRunAccessScope(r.db.WithContext(ctx), ctx).
		Where("run_id = ?", runID).
		Order("position ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormStudioBatchRunRepository) UpdateStudioBatchRun(ctx context.Context, run *StudioBatchRunRecord) error {
	if run == nil {
		return nil
	}

	row := *run
	applyStudioBatchRunScopeDefaults(ctx, &row.TenantID, &row.UserID)
	result := applyStudioBatchRunAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchRunRecord{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"mode":              row.Mode,
			"failure_policy":    row.FailurePolicy,
			"status":            row.Status,
			"current_batch_id":  row.CurrentBatchID,
			"current_index":     row.CurrentIndex,
			"total_batches":     row.TotalBatches,
			"completed_batches": row.CompletedBatches,
			"succeeded_batches": row.SucceededBatches,
			"failed_batches":    row.FailedBatches,
			"last_error":        row.LastError,
			"cancel_requested":  row.CancelRequested,
			"started_at":        row.StartedAt,
			"finished_at":       row.FinishedAt,
			"updated_at":        row.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *GormStudioBatchRunRepository) UpdateStudioBatchRunItem(ctx context.Context, item *StudioBatchRunItemRecord) error {
	if item == nil {
		return nil
	}

	row := *item
	applyStudioBatchRunScopeDefaults(ctx, &row.TenantID, &row.UserID)
	result := applyStudioBatchRunAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchRunItemRecord{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"run_id":        row.RunID,
			"batch_id":      row.BatchID,
			"position":      row.Position,
			"status":        row.Status,
			"session_id":    row.SessionID,
			"async_job_id":  row.AsyncJobID,
			"error_message": row.ErrorMessage,
			"started_at":    row.StartedAt,
			"finished_at":   row.FinishedAt,
			"updated_at":    row.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
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
