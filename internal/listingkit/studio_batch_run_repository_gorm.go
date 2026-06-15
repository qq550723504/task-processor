package listingkit

import (
	"context"

	"gorm.io/gorm"
)

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
