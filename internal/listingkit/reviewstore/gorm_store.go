package reviewstore

import (
	"context"

	"gorm.io/gorm"

	"task-processor/internal/listingkit/tenantctx"
)

type GormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) Repository {
	return &GormRepository{db: db}
}

func (r *GormRepository) SaveReview(ctx context.Context, record *ReviewRecord) error {
	if record == nil {
		return nil
	}
	if record.TenantID == "" {
		record.TenantID = tenantctx.TenantIDFromContext(ctx)
	}
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *GormRepository) ListReviews(ctx context.Context, taskID string) ([]ReviewRecord, error) {
	var out []ReviewRecord
	db := r.db.WithContext(ctx)
	if tenantID, ok := tenantctx.TenantScopeFromContext(ctx); ok {
		if tenantID == tenantctx.DefaultTenantID {
			db = db.Where("(tenant_id = ? OR tenant_id = '' OR tenant_id IS NULL)", tenantID)
		} else {
			db = db.Where("tenant_id = ?", tenantID)
		}
	}
	if err := db.
		Where("task_id = ?", taskID).
		Order("reviewed_at asc, id asc").
		Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}
