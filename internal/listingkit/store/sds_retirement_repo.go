package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

func (r *taskRepository) CreateSDSRetirementRun(ctx context.Context, run *listingkit.SDSRetirementRunRecord, items []listingkit.SDSRetirementItemRecord) error {
	if run == nil {
		return fmt.Errorf("SDS retirement run is required")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(run).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].RunID = run.ID
			if err := tx.Create(&items[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *taskRepository) GetSDSRetirementRun(ctx context.Context, runID string) (*listingkit.SDSRetirementRunRecord, []listingkit.SDSRetirementItemRecord, error) {
	var run listingkit.SDSRetirementRunRecord
	if err := r.db.WithContext(ctx).Where("id = ?", runID).First(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, listingkit.ErrTaskNotFound
		}
		return nil, nil, err
	}
	var items []listingkit.SDSRetirementItemRecord
	if err := r.db.WithContext(ctx).Where("run_id = ?", runID).Order("created_at ASC, id ASC").Find(&items).Error; err != nil {
		return nil, nil, err
	}
	return &run, items, nil
}

func (r *taskRepository) UpdateSDSRetirementItems(ctx context.Context, runID string, updates []listingkit.SDSRetirementItemSelectionUpdate) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, update := range updates {
			status := listingkit.SDSRetirementItemStatusPending
			if update.Selected {
				status = listingkit.SDSRetirementItemStatusSelected
			}
			result := tx.Model(&listingkit.SDSRetirementItemRecord{}).
				Where("run_id = ? AND id = ?", runID, update.ItemID).
				Updates(map[string]any{
					"selected":       update.Selected,
					"site_selection": update.SiteSelection,
					"status":         status,
					"updated_at":     time.Now().UTC(),
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
		}
		return nil
	})
}

func (r *taskRepository) SaveSDSRetirementExecution(ctx context.Context, run *listingkit.SDSRetirementRunRecord, items []listingkit.SDSRetirementItemRecord) error {
	if run == nil {
		return fmt.Errorf("SDS retirement run is required")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(run).Error; err != nil {
			return err
		}
		for i := range items {
			if err := tx.Save(&items[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *taskRepository) MarkSyncedProductOffShelf(ctx context.Context, syncedProductID int64, now time.Time) error {
	if syncedProductID <= 0 {
		return fmt.Errorf("synced product id must be positive")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	result := r.db.WithContext(ctx).
		Model(&listingkit.SheinSyncedProductRecord{}).
		Where("id = ?", syncedProductID).
		Updates(map[string]any{
			"shelf_status": "OFF_SHELF",
			"is_active":    false,
			"updated_at":   now,
			"last_sync_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
