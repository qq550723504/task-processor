package listingkit

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"task-processor/internal/listingkit/tenantctx"

	"gorm.io/gorm"
)

type StudioAsyncJobStatus string

const (
	StudioAsyncJobStatusRunning   StudioAsyncJobStatus = "running"
	StudioAsyncJobStatusSucceeded StudioAsyncJobStatus = "succeeded"
	StudioAsyncJobStatusFailed    StudioAsyncJobStatus = "failed"
)

type StudioAsyncJobRecord struct {
	ID             string               `json:"job_id" gorm:"primaryKey;type:varchar(64)"`
	TenantID       string               `json:"-" gorm:"type:varchar(64);index"`
	UserID         string               `json:"-" gorm:"type:varchar(128);index"`
	Path           string               `json:"path" gorm:"type:varchar(128);not null"`
	Status         StudioAsyncJobStatus `json:"status" gorm:"type:varchar(32);index;not null"`
	ResultJSON     string               `json:"-" gorm:"type:text"`
	Error          string               `json:"error,omitempty" gorm:"type:text"`
	UpstreamStatus int                  `json:"upstream_status,omitempty" gorm:"not null;default:0"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
	FinishedAt     *time.Time           `json:"finished_at,omitempty"`
}

func (StudioAsyncJobRecord) TableName() string {
	return "listingkit_studio_async_jobs"
}

func (r *StudioAsyncJobRecord) DecodeResult() (any, error) {
	if r == nil || r.ResultJSON == "" {
		return nil, nil
	}
	var result any
	if err := json.Unmarshal([]byte(r.ResultJSON), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *StudioAsyncJobRecord) EncodeResult(result any) error {
	if r == nil {
		return nil
	}
	if result == nil {
		r.ResultJSON = ""
		return nil
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	r.ResultJSON = string(data)
	return nil
}

type StudioAsyncJobRepository interface {
	CreateStudioAsyncJob(ctx context.Context, record *StudioAsyncJobRecord) error
	GetStudioAsyncJob(ctx context.Context, jobID string) (*StudioAsyncJobRecord, error)
	UpdateStudioAsyncJob(ctx context.Context, record *StudioAsyncJobRecord) error
}

type MemStudioAsyncJobRepository struct {
	mu      sync.Mutex
	records map[string]StudioAsyncJobRecord
}

func NewMemStudioAsyncJobRepository() *MemStudioAsyncJobRepository {
	return &MemStudioAsyncJobRepository{records: map[string]StudioAsyncJobRecord{}}
}

func (r *MemStudioAsyncJobRepository) CreateStudioAsyncJob(ctx context.Context, record *StudioAsyncJobRecord) error {
	if record == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cloned := *record
	if cloned.TenantID == "" {
		cloned.TenantID = tenantctx.TenantIDFromContext(ctx)
	}
	if cloned.UserID == "" {
		cloned.UserID = RequestUserIDFromContext(ctx)
	}
	r.records[cloned.ID] = cloned
	return nil
}

func (r *MemStudioAsyncJobRepository) GetStudioAsyncJob(ctx context.Context, jobID string) (*StudioAsyncJobRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	record, ok := r.records[jobID]
	if !ok || !matchesStudioAsyncJobScope(ctx, record.TenantID, record.UserID) {
		return nil, gorm.ErrRecordNotFound
	}
	cloned := record
	return &cloned, nil
}

func (r *MemStudioAsyncJobRepository) UpdateStudioAsyncJob(ctx context.Context, record *StudioAsyncJobRecord) error {
	if record == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.records[record.ID]
	if !ok || !matchesStudioAsyncJobScope(ctx, existing.TenantID, existing.UserID) {
		return gorm.ErrRecordNotFound
	}
	cloned := *record
	if cloned.TenantID == "" {
		cloned.TenantID = existing.TenantID
	}
	if cloned.UserID == "" {
		cloned.UserID = existing.UserID
	}
	r.records[cloned.ID] = cloned
	return nil
}

type GormStudioAsyncJobRepository struct {
	db *gorm.DB
}

func NewGormStudioAsyncJobRepository(db *gorm.DB) *GormStudioAsyncJobRepository {
	return &GormStudioAsyncJobRepository{db: db}
}

func AutoMigrateStudioAsyncJobRepository(db *gorm.DB) error {
	return db.AutoMigrate(&StudioAsyncJobRecord{})
}

func (r *GormStudioAsyncJobRepository) CreateStudioAsyncJob(ctx context.Context, record *StudioAsyncJobRecord) error {
	if record == nil {
		return nil
	}
	row := *record
	if row.TenantID == "" {
		row.TenantID = tenantctx.TenantIDFromContext(ctx)
	}
	if row.UserID == "" {
		row.UserID = RequestUserIDFromContext(ctx)
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *GormStudioAsyncJobRepository) GetStudioAsyncJob(ctx context.Context, jobID string) (*StudioAsyncJobRecord, error) {
	var record StudioAsyncJobRecord
	err := applyStudioAsyncJobAccessScope(r.db.WithContext(ctx), ctx).
		Where("id = ?", jobID).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *GormStudioAsyncJobRepository) UpdateStudioAsyncJob(ctx context.Context, record *StudioAsyncJobRecord) error {
	if record == nil {
		return nil
	}
	row := *record
	if row.TenantID == "" {
		row.TenantID = tenantctx.TenantIDFromContext(ctx)
	}
	if row.UserID == "" {
		row.UserID = RequestUserIDFromContext(ctx)
	}
	return applyStudioAsyncJobAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioAsyncJobRecord{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"path":            row.Path,
			"status":          row.Status,
			"result_json":     row.ResultJSON,
			"error":           row.Error,
			"upstream_status": row.UpstreamStatus,
			"finished_at":     row.FinishedAt,
			"updated_at":      row.UpdatedAt,
		}).Error
}

func applyStudioAsyncJobAccessScope(db *gorm.DB, ctx context.Context) *gorm.DB {
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

func matchesStudioAsyncJobScope(ctx context.Context, tenantID string, userID string) bool {
	if !tenantctx.MatchesTenant(tenantID, tenantctx.TenantIDFromContext(ctx)) {
		return false
	}
	if !OwnerScopeEnabled() {
		return true
	}
	requestUserID := RequestUserIDFromContext(ctx)
	return requestUserID == "" || requestUserID == userID
}
