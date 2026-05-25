package listingadmin

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

func storeAccessQuery(ctx context.Context, db *gorm.DB, tenantID int64) *gorm.DB {
	return applyStoreAccessScope(db, StoreQuery{
		TenantID:    tenantID,
		OwnerUserID: requestUserIDFromContext(ctx),
	})
}

func takeStoreAccessRow(ctx context.Context, db *gorm.DB, tenantID, id int64, deleted int16, row *listingStore) error {
	err := storeAccessQuery(ctx, db, tenantID).Where("id = ? AND deleted = ?", id, deleted).Take(row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrStoreNotFound
	}
	return err
}

func updateStoreAccessRow(ctx context.Context, db *gorm.DB, tenantID, id int64, deleted int16, updates map[string]any) error {
	res := storeAccessQuery(ctx, db, tenantID).Where("id = ? AND deleted = ?", id, deleted).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrStoreNotFound
	}
	return nil
}

func deleteStoreAccessRow(ctx context.Context, db *gorm.DB, tenantID, id int64, deleted int16) error {
	res := storeAccessQuery(ctx, db, tenantID).Where("id = ? AND deleted = ?", id, deleted).Delete(&listingStore{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrStoreNotFound
	}
	return nil
}
