package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

func (r *GormSheinSyncRepository) UpsertSyncedProducts(ctx context.Context, records []*listingkit.SheinSyncedProductRecord) error {
	for _, record := range records {
		if record == nil {
			continue
		}

		row := *record
		listingkit.ApplyEffectiveCostPrice(&row)
		now := time.Now().UTC()
		row.UpdatedAt = now
		if row.CreatedAt.IsZero() {
			row.CreatedAt = now
		}
		if row.LastSyncAt == nil {
			row.LastSyncAt = &now
		}

		updates := sheinSyncedProductAssignments(row)
		var existing listingkit.SheinSyncedProductRecord
		err := r.db.WithContext(ctx).
			Where("tenant_id = ? AND store_id = ? AND skc_name = ?", row.TenantID, row.StoreID, row.SKCName).
			First(&existing).Error
		switch {
		case err == nil:
			if record.CreatedAt.IsZero() {
				row.CreatedAt = existing.CreatedAt
			}
			updates = sheinSyncedProductAssignments(row)
			if updateErr := r.db.WithContext(ctx).
				Model(&listingkit.SheinSyncedProductRecord{}).
				Where("id = ?", existing.ID).
				Updates(updates).Error; updateErr != nil {
				return updateErr
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			if createErr := r.db.WithContext(ctx).
				Model(&listingkit.SheinSyncedProductRecord{}).
				Create(updates).Error; createErr != nil {
				return createErr
			}
		default:
			return err
		}
	}
	return nil
}

func (r *GormSheinSyncRepository) ListSyncedProducts(ctx context.Context, query *listingkit.SheinSyncedProductQuery) ([]listingkit.SheinSyncedProductRecord, int64, error) {
	page, pageSize := sheinSyncQueryPage(query)
	db := r.db.WithContext(ctx).Model(&listingkit.SheinSyncedProductRecord{})
	db = applySheinSyncedProductFilters(db, query)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []listingkit.SheinSyncedProductRecord
	if err := db.
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *GormSheinSyncRepository) UpdateManualCostPrice(ctx context.Context, productID int64, manualCostPrice *float64) error {
	var row listingkit.SheinSyncedProductRecord
	if err := r.db.WithContext(ctx).Where("id = ?", productID).First(&row).Error; err != nil {
		return err
	}

	row.ManualCostPrice = cloneFloat64Ptr(manualCostPrice)
	listingkit.ApplyEffectiveCostPrice(&row)
	row.UpdatedAt = time.Now().UTC()

	result := r.db.WithContext(ctx).
		Model(&listingkit.SheinSyncedProductRecord{}).
		Where("id = ?", productID).
		Updates(map[string]any{
			"manual_cost_price":    row.ManualCostPrice,
			"effective_cost_price": row.EffectiveCostPrice,
			"cost_price_source":    row.CostPriceSource,
			"updated_at":           row.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *GormSheinSyncRepository) MarkMissingSyncedProductsInactive(ctx context.Context, tenantID, storeID int64, activeSKCNames []string) error {
	db := r.db.WithContext(ctx).
		Model(&listingkit.SheinSyncedProductRecord{}).
		Where("tenant_id = ? AND store_id = ?", tenantID, storeID)
	if len(activeSKCNames) > 0 {
		db = db.Where("skc_name NOT IN ?", activeSKCNames)
	}
	return db.Updates(map[string]any{
		"is_active":  false,
		"updated_at": time.Now().UTC(),
	}).Error
}

func sheinSyncedProductAssignments(row listingkit.SheinSyncedProductRecord) map[string]any {
	return map[string]any{
		"tenant_id":            row.TenantID,
		"store_id":             row.StoreID,
		"spu_name":             row.SPUName,
		"spu_code":             row.SPUCode,
		"skc_name":             row.SKCName,
		"skc_code":             row.SKCCode,
		"supplier_code":        row.SupplierCode,
		"category_id":          row.CategoryID,
		"brand_name":           row.BrandName,
		"product_name_multi":   row.ProductNameMulti,
		"main_image_url":       row.MainImageURL,
		"sale_name":            row.SaleName,
		"business_model":       row.BusinessModel,
		"shelf_status":         row.ShelfStatus,
		"publish_time":         row.PublishTime,
		"first_shelf_time":     row.FirstShelfTime,
		"currency":             row.Currency,
		"price_snapshot":       row.PriceSnapshot,
		"inventory_snapshot":   row.InventorySnapshot,
		"site_snapshot":        row.SiteSnapshot,
		"auto_cost_price":      row.AutoCostPrice,
		"manual_cost_price":    row.ManualCostPrice,
		"effective_cost_price": row.EffectiveCostPrice,
		"cost_price_source":    row.CostPriceSource,
		"sync_version":         row.SyncVersion,
		"last_sync_at":         row.LastSyncAt,
		"is_active":            row.IsActive,
		"created_at":           row.CreatedAt,
		"updated_at":           row.UpdatedAt,
	}
}
