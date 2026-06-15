package store

import (
	"context"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

func (r *GormSheinSyncRepository) SaveSyncJob(ctx context.Context, job *listingkit.SheinSyncJobRecord) error {
	if job == nil {
		return nil
	}

	row := *job
	now := time.Now().UTC()
	row.UpdatedAt = now
	if row.ID <= 0 {
		if row.CreatedAt.IsZero() {
			row.CreatedAt = now
		}
		if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
		*job = row
		return nil
	}

	result := r.db.WithContext(ctx).
		Model(&listingkit.SheinSyncJobRecord{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"tenant_id":         row.TenantID,
			"store_id":          row.StoreID,
			"trigger_mode":      row.TriggerMode,
			"status":            row.Status,
			"started_at":        row.StartedAt,
			"finished_at":       row.FinishedAt,
			"fetched_count":     row.FetchedCount,
			"inserted_count":    row.InsertedCount,
			"updated_count":     row.UpdatedCount,
			"deactivated_count": row.DeactivatedCount,
			"skipped_count":     row.SkippedCount,
			"error_summary":     row.ErrorSummary,
			"updated_at":        row.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	job.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *GormSheinSyncRepository) ListSyncJobs(ctx context.Context, query *listingkit.SheinSyncJobQuery) ([]listingkit.SheinSyncJobRecord, int64, error) {
	page, pageSize := sheinSyncJobQueryPage(query)
	db := r.db.WithContext(ctx).Model(&listingkit.SheinSyncJobRecord{})
	db = applySheinSyncJobFilters(db, query)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []listingkit.SheinSyncJobRecord
	if err := db.
		Order("started_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
