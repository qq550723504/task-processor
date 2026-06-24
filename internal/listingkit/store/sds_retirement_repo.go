package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
	"task-processor/internal/shared/tenantctx"
)

func (r *taskRepository) CreateSDSRetirementRun(ctx context.Context, run *listingkit.SDSRetirementRunRecord, items []listingkit.SDSRetirementItemRecord) error {
	if run == nil {
		return fmt.Errorf("SDS retirement run is required")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if run.TenantID == "" {
			run.TenantID = tenantctx.TenantIDFromContext(ctx)
		}
		if err := tx.Create(run).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].RunID = run.ID
			if items[i].TenantID == "" {
				items[i].TenantID = run.TenantID
			}
			if err := tx.Create(&items[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *taskRepository) GetSDSRetirementRun(ctx context.Context, runID string) (*listingkit.SDSRetirementRunRecord, []listingkit.SDSRetirementItemRecord, error) {
	var run listingkit.SDSRetirementRunRecord
	scopedDB := applySDSRetirementRunScope(r.db.WithContext(ctx), ctx)
	if err := scopedDB.Where("id = ?", runID).First(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, listingkit.ErrTaskNotFound
		}
		return nil, nil, err
	}
	var items []listingkit.SDSRetirementItemRecord
	if err := applySDSRetirementRunScope(r.db.WithContext(ctx), ctx).
		Where("run_id = ? AND tenant_id = ?", runID, run.TenantID).
		Order("created_at ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, nil, err
	}
	return &run, items, nil
}

func (r *taskRepository) UpdateSDSRetirementItems(ctx context.Context, runID string, updates []listingkit.SDSRetirementItemSelectionUpdate) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run listingkit.SDSRetirementRunRecord
		if err := applySDSRetirementRunScope(tx, ctx).Where("id = ?", runID).First(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return listingkit.ErrTaskNotFound
			}
			return err
		}
		for _, update := range updates {
			status := listingkit.SDSRetirementItemStatusPending
			if update.Selected {
				status = listingkit.SDSRetirementItemStatusSelected
			}
			result := tx.Model(&listingkit.SDSRetirementItemRecord{}).
				Where("run_id = ? AND tenant_id = ? AND id = ?", runID, run.TenantID, update.ItemID).
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
				return listingkit.ErrTaskNotFound
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
		result := tx.Model(&listingkit.SDSRetirementRunRecord{}).
			Where("id = ? AND tenant_id = ?", run.ID, run.TenantID).
			Select("*").
			Omit("created_at").
			Updates(run)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return listingkit.ErrTaskNotFound
		}
		for i := range items {
			if items[i].RunID != run.ID || items[i].TenantID != "" && items[i].TenantID != run.TenantID {
				return listingkit.ErrTaskNotFound
			}
			result := tx.Model(&listingkit.SDSRetirementItemRecord{}).
				Where("run_id = ? AND tenant_id = ? AND id = ?", run.ID, run.TenantID, items[i].ID).
				Select("*").
				Omit("created_at").
				Updates(&items[i])
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return listingkit.ErrTaskNotFound
			}
		}
		return nil
	})
}

func (r *taskRepository) MarkSyncedProductOffShelf(ctx context.Context, tenantID, storeID, syncedProductID int64, now time.Time) error {
	if tenantID <= 0 {
		return fmt.Errorf("tenant id must be positive")
	}
	if storeID <= 0 {
		return fmt.Errorf("store id must be positive")
	}
	if syncedProductID <= 0 {
		return fmt.Errorf("synced product id must be positive")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	result := r.db.WithContext(ctx).
		Model(&listingkit.SheinSyncedProductRecord{}).
		Where("id = ? AND tenant_id = ? AND store_id = ?", syncedProductID, tenantID, storeID).
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

func applySDSRetirementRunScope(db *gorm.DB, ctx context.Context) *gorm.DB {
	tenantID, ok := tenantctx.TenantScopeFromContext(ctx)
	if !ok {
		return db.Where("1 = 0")
	}
	if tenantID == tenantctx.DefaultTenantID {
		return db.Where("(tenant_id = ? OR tenant_id = '' OR tenant_id IS NULL)", tenantID)
	}
	return db.Where("tenant_id = ?", tenantID)
}
