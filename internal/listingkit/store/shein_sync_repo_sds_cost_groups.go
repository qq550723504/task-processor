package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

func (r *GormSheinSyncRepository) UpsertSDSCostGroup(ctx context.Context, record *listingkit.SheinSDSCostGroupRecord) error {
	if record == nil {
		return nil
	}

	now := time.Now().UTC()
	row := *record
	row.GroupKey = strings.TrimSpace(row.GroupKey)
	row.GroupLabel = strings.TrimSpace(row.GroupLabel)
	row.UpdatedAt = now
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}

	var existing listingkit.SheinSDSCostGroupRecord
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND store_id = ? AND group_key = ?", row.TenantID, row.StoreID, row.GroupKey).
		First(&existing).Error
	switch {
	case err == nil:
		if row.GroupLabel == "" {
			row.GroupLabel = existing.GroupLabel
		}
		return r.db.WithContext(ctx).
			Model(&listingkit.SheinSDSCostGroupRecord{}).
			Where("id = ?", existing.ID).
			Updates(map[string]any{
				"group_label":       row.GroupLabel,
				"manual_cost_price": row.ManualCostPrice,
				"updated_at":        row.UpdatedAt,
			}).Error
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
		return err
	default:
		return r.db.WithContext(ctx).Create(&row).Error
	}
}

func (r *GormSheinSyncRepository) ListSDSCostGroups(ctx context.Context, query *listingkit.SheinSDSCostGroupQuery) ([]listingkit.SheinSDSCostGroupRecord, int64, error) {
	page, pageSize := normalizeSheinSyncPage(0, 0)
	db := r.db.WithContext(ctx).Model(&listingkit.SheinSDSCostGroupRecord{})
	if query != nil {
		page, pageSize = normalizeSheinSyncPage(query.Page, query.PageSize)
		if query.TenantID > 0 {
			db = db.Where("tenant_id = ?", query.TenantID)
		}
		if query.StoreID > 0 {
			db = db.Where("store_id = ?", query.StoreID)
		}
		if len(query.GroupKeys) > 0 {
			db = db.Where("group_key IN ?", normalizedSheinSDSGroupKeys(query.GroupKeys))
		}
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []listingkit.SheinSDSCostGroupRecord
	if err := db.Order("group_key ASC, id ASC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
