package listingadmin

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

func findProductImportMappingRows(ctx context.Context, db *gorm.DB, query ProductImportMappingQuery) ([]listingProductImportMapping, int64, int, int, error) {
	scopeQuery := query
	if strings.TrimSpace(scopeQuery.OwnerUserID) == "" {
		scopeQuery.OwnerUserID = requestUserIDFromContext(ctx)
	}
	var rows []listingProductImportMapping
	total, page, pageSize, err := findPagedRows(applyProductImportMappingQuery(db, scopeQuery), scopeQuery.Page, scopeQuery.PageSize, &rows)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return rows, total, page, pageSize, nil
}

func applyProductImportMappingQuery(db *gorm.DB, query ProductImportMappingQuery) *gorm.DB {
	db = applyOwnedTenantQuery(db, query.TenantID, strings.TrimSpace(query.OwnerUserID))
	if query.ImportTaskID != nil {
		db = db.Where("import_task_id = ?", *query.ImportTaskID)
	}
	if query.StoreID != nil {
		db = db.Where("store_id = ?", *query.StoreID)
	}
	if query.Platform != "" {
		db = db.Where("platform = ?", query.Platform)
	}
	if query.Region != "" {
		db = db.Where("region = ?", query.Region)
	}
	if query.ProductID != "" {
		db = db.Where("product_id = ?", query.ProductID)
	}
	if query.ParentProductID != "" {
		db = db.Where("parent_product_id = ?", query.ParentProductID)
	}
	if query.SKU != "" {
		db = db.Where("sku = ?", query.SKU)
	}
	if query.PlatformProductID != "" {
		db = db.Where("platform_product_id = ?", query.PlatformProductID)
	}
	if query.PlatformParentProductID != "" {
		db = db.Where("platform_parent_product_id = ?", query.PlatformParentProductID)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	return db
}
