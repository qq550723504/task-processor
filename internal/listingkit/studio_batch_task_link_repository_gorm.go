package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type GormStudioBatchTaskLinkRepository struct {
	db *gorm.DB
}

func NewGormStudioBatchTaskLinkRepository(db *gorm.DB) *GormStudioBatchTaskLinkRepository {
	return &GormStudioBatchTaskLinkRepository{db: db}
}

func AutoMigrateStudioBatchTaskLinkRepository(db *gorm.DB) error {
	return db.AutoMigrate(&StudioBatchTaskLinkRecord{})
}

func (r *GormStudioBatchTaskLinkRepository) GetStudioBatchTaskLinkByCandidateKey(ctx context.Context, candidateKey string) (*StudioBatchTaskLinkRecord, error) {
	var record StudioBatchTaskLinkRecord
	err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Where("candidate_key = ?", candidateKey).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *GormStudioBatchTaskLinkRepository) CreateStudioBatchTaskLink(ctx context.Context, link *StudioBatchTaskLinkRecord) error {
	if link == nil {
		return nil
	}

	row := *link
	applyStudioBatchTaskLinkCreateScope(ctx, &row)
	if strings.TrimSpace(row.ID) == "" {
		return fmt.Errorf("studio batch task link id is required")
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *GormStudioBatchTaskLinkRepository) UpdateStudioBatchTaskLink(ctx context.Context, link *StudioBatchTaskLinkRecord) error {
	if link == nil {
		return nil
	}

	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchTaskLinkRecord{}).
		Where("id = ?", link.ID).
		Updates(map[string]any{
			"listingkit_task_id": link.ListingKitTaskID,
			"status":             link.Status,
			"reason_code":        link.ReasonCode,
			"message":            link.Message,
			"updated_at":         link.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *GormStudioBatchTaskLinkRepository) ListStudioBatchTaskLinksByBatchID(ctx context.Context, batchID string) ([]StudioBatchTaskLinkRecord, error) {
	var links []StudioBatchTaskLinkRecord
	if err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Where("batch_id = ?", batchID).
		Order("id ASC").
		Find(&links).Error; err != nil {
		return nil, err
	}
	return links, nil
}

func (r *GormStudioBatchTaskLinkRepository) ClaimStudioBatchTaskCandidate(ctx context.Context, candidateKey string, fromStatus string, toStatus string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error) {
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchTaskLinkRecord{}).
		Where("candidate_key = ? AND status = ?", candidateKey, fromStatus).
		Updates(map[string]any{
			"status":     toStatus,
			"updated_at": updatedAt,
		})
	if result.Error != nil {
		return nil, false, result.Error
	}
	link, err := r.GetStudioBatchTaskLinkByCandidateKey(ctx, candidateKey)
	if err != nil {
		return nil, false, err
	}
	return link, result.RowsAffected > 0, nil
}
