package listingadmin

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

func findImportTaskRows(ctx context.Context, db *gorm.DB, query ImportTaskQuery) ([]listingProductImportTask, int64, int, int, error) {
	scopeQuery := query
	if strings.TrimSpace(scopeQuery.OwnerUserID) == "" {
		scopeQuery.OwnerUserID = requestUserIDFromContext(ctx)
	}
	var rows []listingProductImportTask
	total, page, pageSize, err := findPagedRows(applyImportTaskQuery(db, scopeQuery), scopeQuery.Page, scopeQuery.PageSize, &rows)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return rows, total, page, pageSize, nil
}

func applyImportTaskQuery(db *gorm.DB, query ImportTaskQuery) *gorm.DB {
	db = applyOwnedTenantQuery(db, query.TenantID, strings.TrimSpace(query.OwnerUserID))
	if query.StoreID != nil {
		db = db.Where("store_id = ?", *query.StoreID)
	}
	if query.Platform != "" {
		db = db.Where("platform = ?", query.Platform)
	}
	if query.Region != "" {
		db = db.Where("region = ?", query.Region)
	}
	if query.CategoryID != nil {
		db = db.Where("category_id = ?", *query.CategoryID)
	}
	if query.ProductID != "" {
		db = db.Where("product_id LIKE ?", "%"+query.ProductID+"%")
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	return db
}
