package listingadmin

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

func findSensitiveWordRows(ctx context.Context, db *gorm.DB, query SensitiveWordQuery) ([]listingSensitiveWord, int64, int, int, error) {
	scopeQuery := query
	if strings.TrimSpace(scopeQuery.OwnerUserID) == "" {
		scopeQuery.OwnerUserID = requestUserIDFromContext(ctx)
	}
	var rows []listingSensitiveWord
	total, page, pageSize, err := findPagedRows(applySensitiveWordQuery(db, scopeQuery), scopeQuery.Page, scopeQuery.PageSize, &rows)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return rows, total, page, pageSize, nil
}

func applySensitiveWordQuery(db *gorm.DB, query SensitiveWordQuery) *gorm.DB {
	db = applyOwnedTenantQuery(db, query.TenantID, strings.TrimSpace(query.OwnerUserID))
	if query.Word != "" {
		db = db.Where("word LIKE ?", "%"+query.Word+"%")
	}
	if query.Language != "" {
		db = db.Where("language = ?", query.Language)
	}
	if query.Tags != "" {
		db = db.Where("tags LIKE ?", "%"+query.Tags+"%")
	}
	if query.Level != nil {
		db = db.Where("level = ?", *query.Level)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	if query.Remark != "" {
		db = db.Where("remark LIKE ?", "%"+query.Remark+"%")
	}
	return db
}
