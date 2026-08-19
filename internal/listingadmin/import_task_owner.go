package listingadmin

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// ResolveStoreOwnerUserID returns the canonical owner of a tenant-scoped store
// for internal child writes such as import tasks.
func ResolveStoreOwnerUserID(ctx context.Context, db *gorm.DB, tenantID, storeID int64) (string, error) {
	if db == nil {
		return "", errors.New("database is not configured")
	}
	var row listingStore
	err := db.WithContext(ctx).
		Table("listing_store").
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
