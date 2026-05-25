package listingadmin

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

func applyOwnedTenantQuery(db *gorm.DB, tenantID int64, ownerUserID string) *gorm.DB {
	db = db.Where("deleted = 0")
	if tenantID > 0 {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if ownerScopeEnabled() && ownerUserID != "" {
		db = db.Where("owner_user_id = ?", ownerUserID)
	}
	return db
}

func normalizePage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func findPagedRows[T any](db *gorm.DB, page, pageSize int, rows *[]T) (total int64, normalizedPage int, normalizedPageSize int, err error) {
	normalizedPage, normalizedPageSize = normalizePage(page, pageSize)
	if err = db.Count(&total).Error; err != nil {
		return 0, normalizedPage, normalizedPageSize, err
	}
	if err = db.Order("id desc").Offset((normalizedPage - 1) * normalizedPageSize).Limit(normalizedPageSize).Find(rows).Error; err != nil {
		return 0, normalizedPage, normalizedPageSize, err
	}
	return total, normalizedPage, normalizedPageSize, nil
}

func takeOwnedTenantRow[T any](ctx context.Context, db *gorm.DB, tenantID, id int64, ownerColumn string, row *T, notFound error) error {
	err := applyOwnerScope(
		db.Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		ownerColumn,
	).Take(row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notFound
	}
	return err
}

func updateOwnedTenantRow(ctx context.Context, db *gorm.DB, tenantID, id int64, ownerColumn string, updates map[string]any, notFound error) error {
	res := applyOwnerScope(
		db.Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		ownerColumn,
	).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return notFound
	}
	return nil
}
