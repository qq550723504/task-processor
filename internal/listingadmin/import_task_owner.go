package listingadmin

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ResolveStoreOwnerUserID returns the canonical owner of a tenant-scoped store
// for internal child writes such as import tasks.
func ResolveStoreOwnerUserID(ctx context.Context, db *gorm.DB, tenantID, storeID int64) (string, error) {
	return resolveStoreOwnerUserID(ctx, db, tenantID, storeID, false)
}

func resolveStoreOwnerUserIDForUpdate(ctx context.Context, db *gorm.DB, tenantID, storeID int64) (string, error) {
	return resolveStoreOwnerUserID(ctx, db, tenantID, storeID, true)
}

func resolveStoreOwnerUserID(ctx context.Context, db *gorm.DB, tenantID, storeID int64, lock bool) (string, error) {
	if db == nil {
		return "", errors.New("database is not configured")
	}
	query := db.WithContext(ctx).Table("listing_store")
	if lock && db.Dialector != nil && db.Dialector.Name() == "postgres" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row listingStore
	err := query.
		Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, storeID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ErrStoreNotFound
	}
	if err != nil {
		return "", err
	}
	ownerUserID := strings.TrimSpace(row.OwnerUserID)
	if ownerUserID == "" {
		return "", ErrOwnerUserIDRequired
	}
	return ownerUserID, nil
}
