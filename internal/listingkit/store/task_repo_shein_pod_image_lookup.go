package store

import (
	"context"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

func (r *taskRepository) LookupSheinPODImages(ctx context.Context, query *listingkit.SheinPODImageLookupQuery) ([]listingkit.SheinPODImageLookupRecord, int64, error) {
	if query == nil || query.StoreID <= 0 {
		return []listingkit.SheinPODImageLookupRecord{}, 0, nil
	}
	limit := normalizeSheinPODImageLookupLimit(query.Limit)
	db := applyTaskAccessScope(r.db.WithContext(ctx).Model(&listingkit.Task{}), ctx)
	db = applySheinPODImageLookupStoreScope(db, query.StoreID)
	db = applySheinPODImageLookupQueryScope(db, query.Query)

	var tasks []listingkit.Task
	if err := db.
		Select("id", "tenant_id", "user_id", "request", "shein_store_resolution_snapshot", "status", "result", "created_at", "updated_at").
		Order("updated_at DESC").
		Limit(5000).
		Find(&tasks).Error; err != nil {
		return nil, 0, err
	}

	items := make([]listingkit.SheinPODImageLookupRecord, 0, len(tasks))
	for i := range tasks {
		record, ok := listingkit.BuildSheinPODImageLookupRecord(&tasks[i])
		if !ok || record.StoreID != query.StoreID {
			continue
		}
		if !listingkit.SheinPODImageLookupRecordMatches(record, query.Query) {
			continue
		}
		items = append(items, record)
	}
	total := int64(len(items))
	if len(items) > limit {
		items = items[:limit]
	}
	return items, total, nil
}

func normalizeSheinPODImageLookupLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func applySheinPODImageLookupStoreScope(db *gorm.DB, storeID int64) *gorm.DB {
	switch db.Dialector.Name() {
	case "postgres":
		return db.Where(`(
			(shein_store_resolution_snapshot::jsonb ->> 'store_id') = ?
			OR (request::jsonb ->> 'shein_store_id') = ?
		)`, strconv.FormatInt(storeID, 10), strconv.FormatInt(storeID, 10))
	case "sqlite":
		return db.Where(`(
			CAST(json_extract(shein_store_resolution_snapshot, '$.store_id') AS INTEGER) = ?
			OR CAST(json_extract(request, '$.shein_store_id') AS INTEGER) = ?
		)`, storeID, storeID)
	default:
		return db
	}
}

func applySheinPODImageLookupQueryScope(db *gorm.DB, rawQuery string) *gorm.DB {
	trimmed := strings.TrimSpace(rawQuery)
	compact := listingkit.NormalizeSheinPODImageLookupQueryToken(trimmed)
	if trimmed == "" && compact == "" {
		return db
	}
	likeRaw := "%" + strings.ToUpper(trimmed) + "%"
	likeCompact := "%" + compact + "%"
	switch db.Dialector.Name() {
	case "postgres":
		return db.Where(`(
			UPPER(COALESCE(id, '')) LIKE ?
			OR UPPER(COALESCE(result, '')) LIKE ?
			OR UPPER(COALESCE(request, '')) LIKE ?
			OR UPPER(REPLACE(REPLACE(COALESCE(result, ''), '-', ''), '_', '')) LIKE ?
			OR UPPER(REPLACE(REPLACE(COALESCE(request, ''), '-', ''), '_', '')) LIKE ?
		)`, likeRaw, likeRaw, likeRaw, likeCompact, likeCompact)
	case "sqlite":
		return db.Where(`(
			UPPER(COALESCE(id, '')) LIKE ?
			OR UPPER(COALESCE(result, '')) LIKE ?
			OR UPPER(COALESCE(request, '')) LIKE ?
			OR UPPER(REPLACE(REPLACE(COALESCE(result, ''), '-', ''), '_', '')) LIKE ?
			OR UPPER(REPLACE(REPLACE(COALESCE(request, ''), '-', ''), '_', '')) LIKE ?
		)`, likeRaw, likeRaw, likeRaw, likeCompact, likeCompact)
	default:
		return db
	}
}
