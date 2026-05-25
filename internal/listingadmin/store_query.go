package listingadmin

import (
	"context"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

func findStoreRows(ctx context.Context, db *gorm.DB, query StoreQuery) ([]listingStore, error) {
	scopeQuery := query
	if strings.TrimSpace(scopeQuery.OwnerUserID) == "" {
		scopeQuery.OwnerUserID = requestUserIDFromContext(ctx)
	}
	var rows []listingStore
	if err := applyStoreQuery(db, scopeQuery).Order("id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func applyStoreQuery(db *gorm.DB, query StoreQuery) *gorm.DB {
	if query.Deleted != nil {
		db = db.Where("deleted = ?", *query.Deleted)
	} else {
		db = db.Where("deleted = 0")
	}
	if query.Deleted == nil {
		db = applyOwnedTenantQuery(db, query.TenantID, strings.TrimSpace(query.OwnerUserID))
	} else {
		if query.TenantID > 0 {
			db = db.Where("tenant_id = ?", query.TenantID)
		}
		if ownerScopeEnabled() && strings.TrimSpace(query.OwnerUserID) != "" {
			db = db.Where("owner_user_id = ?", strings.TrimSpace(query.OwnerUserID))
		}
	}
	if query.StoreID != "" {
		db = db.Where("store_id LIKE ?", "%"+query.StoreID+"%")
	}
	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Username != "" {
		db = db.Where("username LIKE ?", "%"+query.Username+"%")
	}
	if query.ShopType != "" {
		db = db.Where("shop_type = ?", query.ShopType)
	}
	if query.Region != "" {
		db = db.Where("region = ?", query.Region)
	}
	if query.Platform != "" {
		db = db.Where("platform = ?", query.Platform)
	}
	if query.SKUGenerate != "" {
		db = db.Where("sku_generate_strategy = ?", query.SKUGenerate)
	}
	if query.EnableAutoListing != nil {
		db = db.Where("enable_auto_listing = ?", *query.EnableAutoListing)
	}
	if query.EnableAutoLogin != nil {
		db = db.Where("enable_auto_login = ?", *query.EnableAutoLogin)
	}
	if query.EnableDraft != nil {
		db = db.Where("enable_draft = ?", *query.EnableDraft)
	}
	if query.EnableAutoPrice != nil {
		db = db.Where("enable_auto_price = ?", *query.EnableAutoPrice)
	}
	if query.EnableRebargain != nil {
		db = db.Where("enable_rebargain = ?", *query.EnableRebargain)
	}
	if query.PriceType != "" {
		db = db.Where("price_type = ?", query.PriceType)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	if query.Expired != nil {
		db = db.Where("expired = ?", *query.Expired)
	}
	return db
}

func parseTenantID(value string) int64 {
	id, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return id
}

func applyStoreAccessScope(db *gorm.DB, query StoreQuery) *gorm.DB {
	if query.TenantID > 0 {
		db = db.Where("tenant_id = ?", query.TenantID)
	}
	if ownerScopeEnabled() && strings.TrimSpace(query.OwnerUserID) != "" {
		db = db.Where("owner_user_id = ?", strings.TrimSpace(query.OwnerUserID))
	}
	return db
}
