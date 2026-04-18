package reviewstore

import (
	"context"

	"gorm.io/gorm"
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
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *GormRepository) ListReviews(ctx context.Context, taskID string) ([]ReviewRecord, error) {
	var out []ReviewRecord
	if err := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("reviewed_at asc, id asc").
		Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}
