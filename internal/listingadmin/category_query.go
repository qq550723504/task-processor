package listingadmin

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

func findCategoryRows(ctx context.Context, db *gorm.DB, query CategoryQuery) ([]listingCategory, error) {
	scopeQuery := query
	if strings.TrimSpace(scopeQuery.OwnerUserID) == "" {
		scopeQuery.OwnerUserID = requestUserIDFromContext(ctx)
	}
	var rows []listingCategory
	if err := applyCategoryQuery(db, scopeQuery).Order("parent_id asc, sort asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func applyCategoryQuery(db *gorm.DB, query CategoryQuery) *gorm.DB {
	db = applyOwnedTenantQuery(db, query.TenantID, strings.TrimSpace(query.OwnerUserID))
	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Code != "" {
		db = db.Where("code = ?", query.Code)
	}
	if query.ParentID != nil {
		db = db.Where("parent_id = ?", *query.ParentID)
	}
	if query.Level != nil {
		db = db.Where("level = ?", *query.Level)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	return db
}
