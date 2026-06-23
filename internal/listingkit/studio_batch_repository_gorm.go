package listingkit

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type GormStudioBatchRepository struct {
	db *gorm.DB
}

func NewGormStudioBatchRepository(db *gorm.DB) *GormStudioBatchRepository {
	return &GormStudioBatchRepository{db: db}
}

func AutoMigrateStudioBatchRepository(db *gorm.DB) error {
	return db.AutoMigrate(
		&StudioBatchRecord{},
		&StudioBatchItemRecord{},
		&StudioGenerationAttemptRecord{},
		&StudioMaterializedDesignRecord{},
	)
}

func (r *GormStudioBatchRepository) CreateStudioBatchGraph(ctx context.Context, batch *StudioBatchRecord, items []StudioBatchItemRecord, attempts []StudioGenerationAttemptRecord, designs []StudioMaterializedDesignRecord) error {
	if batch == nil {
		return nil
	}

	batchRow, itemRows, attemptRows, designRows, err := prepareStudioBatchGraph(ctx, batch, items, attempts, designs)
	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&batchRow).Error; err != nil {
			return err
		}
		if len(itemRows) > 0 {
			if err := tx.Create(&itemRows).Error; err != nil {
				return err
			}
		}
		if len(attemptRows) > 0 {
			if err := tx.Create(&attemptRows).Error; err != nil {
				return err
			}
		}
		if len(designRows) > 0 {
			if err := tx.Create(&designRows).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *GormStudioBatchRepository) ReplaceStudioBatchGenerationGraph(ctx context.Context, batch *StudioBatchRecord, items []StudioBatchItemRecord) error {
	if batch == nil {
		return nil
	}

	batchRow, itemRows, _, _, err := prepareStudioBatchGraph(ctx, batch, items, nil, nil)
	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing StudioBatchRecord
		findErr := applyStudioBatchAccessScope(tx, ctx).Where("id = ?", batchRow.ID).First(&existing).Error
		switch {
		case errors.Is(findErr, gorm.ErrRecordNotFound):
			if err := tx.Create(&batchRow).Error; err != nil {
				return err
			}
		case findErr != nil:
			return findErr
		default:
			if err := applyStudioBatchAccessScope(tx, ctx).
				Model(&StudioBatchRecord{}).
				Where("id = ?", batchRow.ID).
				Updates(map[string]any{
					"status":                 batchRow.Status,
					"prompt":                 batchRow.Prompt,
					"prompt_mode":            batchRow.PromptMode,
					"grouped_image_mode":     batchRow.GroupedImageMode,
					"selection":              batchRow.Selection,
					"grouped_selections":     batchRow.GroupedSelections,
					"style_count":            batchRow.StyleCount,
					"variation_intensity":    batchRow.VariationIntensity,
					"artwork_model":          batchRow.ArtworkModel,
					"selected_sds_images":    batchRow.SelectedSDSImages,
					"transparent_background": batchRow.TransparentBackground,
					"shein_store_id":         batchRow.SheinStoreID,
					"updated_at":             batchRow.UpdatedAt,
				}).Error; err != nil {
				return err
			}
		}

		if err := applyStudioBatchAccessScope(tx, ctx).
			Where("batch_id = ?", batchRow.ID).
			Delete(&StudioGenerationAttemptRecord{}).Error; err != nil {
			return err
		}
		if err := applyStudioBatchAccessScope(tx, ctx).
			Where("batch_id = ?", batchRow.ID).
			Delete(&StudioMaterializedDesignRecord{}).Error; err != nil {
			return err
		}
		if err := applyStudioBatchAccessScope(tx, ctx).
			Where("batch_id = ?", batchRow.ID).
			Delete(&StudioBatchItemRecord{}).Error; err != nil {
			return err
		}
		if len(itemRows) == 0 {
			return nil
		}
		return tx.Create(&itemRows).Error
	})
}

func (r *GormStudioBatchRepository) CreateStudioBatchItems(ctx context.Context, batchID string, items []StudioBatchItemRecord) error {
	batch, err := r.GetStudioBatch(ctx, batchID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}

	rows := make([]StudioBatchItemRecord, 0, len(items))
	for _, item := range items {
		row := item
		row.BatchID = batch.ID
		row.TenantID = batch.TenantID
		row.UserID = batch.UserID
		rows = append(rows, row)
	}
	return r.db.WithContext(ctx).Create(&rows).Error
}

func (r *GormStudioBatchRepository) CreateStudioGenerationAttempt(ctx context.Context, attempt *StudioGenerationAttemptRecord) error {
	if attempt == nil {
		return nil
	}

	item, err := r.GetStudioBatchItem(ctx, attempt.ItemID)
	if err != nil {
		return err
	}
	row := *attempt
	row.BatchID = item.BatchID
	row.TenantID = item.TenantID
	row.UserID = item.UserID
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *GormStudioBatchRepository) ClaimStudioBatchItem(ctx context.Context, itemID string, fromStatus StudioBatchItemStatus, toStatus StudioBatchItemStatus, updatedAt time.Time) (*StudioBatchItemRecord, bool, error) {
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchItemRecord{}).
		Where("id = ? AND status = ?", itemID, fromStatus).
		Updates(map[string]any{
			"status":     toStatus,
			"last_error": "",
			"updated_at": updatedAt,
		})
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		item, err := r.GetStudioBatchItem(ctx, itemID)
		if err != nil {
			return nil, false, err
		}
		return item, false, nil
	}
	item, err := r.GetStudioBatchItem(ctx, itemID)
	if err != nil {
		return nil, false, err
	}
	return item, true, nil
}

func (r *GormStudioBatchRepository) GetStudioBatch(ctx context.Context, batchID string) (*StudioBatchRecord, error) {
	var record StudioBatchRecord
	err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Where("id = ?", batchID).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *GormStudioBatchRepository) GetStudioBatchItem(ctx context.Context, itemID string) (*StudioBatchItemRecord, error) {
	var record StudioBatchItemRecord
	err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Where("id = ?", itemID).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *GormStudioBatchRepository) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetailGraph, error) {
	batch, err := r.GetStudioBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}

	var items []StudioBatchItemRecord
	if err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Where("batch_id = ?", batchID).
		Order("created_at ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}

	itemIDs := make([]string, 0, len(items))
	for _, item := range items {
		itemIDs = append(itemIDs, item.ID)
	}

	attempts := make([]StudioGenerationAttemptRecord, 0)
	designs := make([]StudioMaterializedDesignRecord, 0)
	if len(itemIDs) > 0 {
		if err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
			Where("item_id IN ?", itemIDs).
			Order("attempt_no ASC, id ASC").
			Find(&attempts).Error; err != nil {
			return nil, err
		}

		if err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
			Where("item_id IN ?", itemIDs).
			Order("sort_order ASC, created_at ASC, id ASC").
			Find(&designs).Error; err != nil {
			return nil, err
		}
	}

	return buildStudioBatchDetailGraph(batch, items, attempts, designs), nil
}

func (r *GormStudioBatchRepository) ListStudioMaterializedDesignsByIDs(ctx context.Context, batchID string, designIDs []string) ([]StudioMaterializedDesignRecord, error) {
	if _, err := r.GetStudioBatch(ctx, batchID); err != nil {
		return nil, err
	}
	if len(designIDs) == 0 {
		return nil, nil
	}

	var designs []StudioMaterializedDesignRecord
	if err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Where("batch_id = ? AND id IN ?", batchID, designIDs).
		Order("sort_order ASC, created_at ASC, id ASC").
		Find(&designs).Error; err != nil {
		return nil, err
	}
	return designs, nil
}
