package store

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/sheinpodimage"
)

func (r *taskRepository) LookupSheinPODImages(ctx context.Context, query *sheinpodimage.SheinPODImageLookupQuery) ([]sheinpodimage.SheinPODImageLookupRecord, int64, error) {
	if query == nil || query.StoreID <= 0 {
		return []sheinpodimage.SheinPODImageLookupRecord{}, 0, nil
	}
	limit := normalizeSheinPODImageLookupLimit(query.Limit)
	db := applySheinPODImageLookupAccessScope(
		r.db.WithContext(ctx).Model(&listingkit.SheinPODImageLookupIndex{}),
		ctx,
	)
	db = applySheinPODImageLookupStoreScope(db, query.StoreID)
	db = applySheinPODImageLookupQueryScope(db, query.Query)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var indexes []listingkit.SheinPODImageLookupIndex
	if err := db.Order("updated_at DESC").Limit(limit).Find(&indexes).Error; err != nil {
		return nil, 0, err
	}
	items := make([]sheinpodimage.SheinPODImageLookupRecord, 0, len(indexes))
	for i := range indexes {
		items = append(items, sheinPODImageLookupRecordFromIndex(&indexes[i]))
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
	return db.Where("store_id = ?", storeID)
}

func applySheinPODImageLookupQueryScope(db *gorm.DB, rawQuery string) *gorm.DB {
	normalized := sheinpodimage.NormalizeSheinPODImageLookupQueryToken(rawQuery)
	if normalized == "" {
		return db
	}
	query := sheinPODImageLookupKey(rawQuery)
	return db.Where(`(
		(task_id_lookup_key = ? AND normalized_task_id = ?)
		OR (product_name_lookup_key = ? AND normalized_product_name = ?)
		OR (supplier_code_lookup_key = ? AND normalized_supplier_code = ?)
		OR (seller_sku_lookup_key = ? AND normalized_seller_sku = ?)
		OR (shein_spu_name_lookup_key = ? AND normalized_shein_spu_name = ?)
		OR (shein_version_lookup_key = ? AND normalized_shein_version = ?)
		OR (ai_original_image_url_lookup_key = ? AND normalized_ai_original_image_url = ?)
		OR (ai_original_image_key_lookup_key = ? AND normalized_ai_original_image_key = ?)
		OR (sds_main_image_url_lookup_key = ? AND normalized_sds_main_image_url = ?)
	)`,
		query, normalized,
		query, normalized,
		query, normalized,
		query, normalized,
		query, normalized,
		query, normalized,
		query, normalized,
		query, normalized,
		query, normalized,
	)
}

func applySheinPODImageLookupAccessScope(db *gorm.DB, ctx context.Context) *gorm.DB {
	db = applyTenantScope(db, ctx, "tenant_id")
	if !listingkit.OwnerScopeEnabled() || listingkit.RequestHasPlatformAdminAccess(ctx) {
		return db
	}
	userID := strings.TrimSpace(listingkit.RequestUserIDFromContext(ctx))
	if userID == "" {
		return db
	}
	return db.Where("user_id = ?", userID)
}

func sheinPODImageLookupRecordFromIndex(index *listingkit.SheinPODImageLookupIndex) sheinpodimage.SheinPODImageLookupRecord {
	if index == nil {
		return sheinpodimage.SheinPODImageLookupRecord{}
	}
	return sheinpodimage.SheinPODImageLookupRecord{
		TaskID:              index.TaskID,
		StoreID:             index.StoreID,
		Status:              index.Status,
		Prompt:              index.Prompt,
		ProductName:         index.ProductName,
		SupplierCode:        index.SupplierCode,
		SellerSKU:           index.SellerSKU,
		SheinSPUName:        index.SheinSPUName,
		SheinVersion:        index.SheinVersion,
		AIOriginalImageURL:  index.AIOriginalImageURL,
		AIOriginalImageKey:  index.AIOriginalImageKey,
		SDSMainImageURL:     index.SDSMainImageURL,
		SDSGalleryImageURLs: append([]string(nil), index.SDSGalleryImageURLs...),
		CreatedAt:           index.CreatedAt,
		UpdatedAt:           index.UpdatedAt,
	}
}
