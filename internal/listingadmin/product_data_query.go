package listingadmin

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

func findProductDataRows(ctx context.Context, db *gorm.DB, query ProductDataQuery) ([]listingProductData, int64, int, int, error) {
	scopeQuery := query
	if strings.TrimSpace(scopeQuery.OwnerUserID) == "" {
		scopeQuery.OwnerUserID = requestUserIDFromContext(ctx)
	}
	var rows []listingProductData
	total, page, pageSize, err := findPagedRows(applyProductDataQuery(db, scopeQuery), scopeQuery.Page, scopeQuery.PageSize, &rows)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return rows, total, page, pageSize, nil
}

func applyProductDataQuery(db *gorm.DB, query ProductDataQuery) *gorm.DB {
	db = applyOwnedTenantQuery(db, query.TenantID, strings.TrimSpace(query.OwnerUserID))
	if query.StoreID != nil {
		db = db.Where("store_id = ?", *query.StoreID)
	}
	if query.CategoryID != nil {
		db = db.Where("category_id = ?", *query.CategoryID)
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
	if query.Title != "" {
		db = db.Where("title LIKE ?", "%"+query.Title+"%")
	}
	if query.Brand != "" {
		db = db.Where("brand = ?", query.Brand)
	}
	if query.Category != "" {
		db = db.Where("category LIKE ?", "%"+query.Category+"%")
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	if query.PlatformProductID != "" {
		db = db.Where("platform_product_id = ?", query.PlatformProductID)
	}
	if query.ShelfStatus != nil {
		db = db.Where("shelf_status = ?", *query.ShelfStatus)
	}
	return db
}
