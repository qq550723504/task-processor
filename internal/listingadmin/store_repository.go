package listingadmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type GormStoreRepository struct {
	db *gorm.DB
}

func NewGormStoreRepository(db *gorm.DB) *GormStoreRepository {
	return &GormStoreRepository{db: db}
}

func AutoMigrateStoreRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	return ensureOwnerAuditColumns(db, (listingStore{}).TableName())
}

func (r *GormStoreRepository) ListStores(ctx context.Context, query StoreQuery) (*StorePage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("store repository database is not configured")
	}
	db := applyStoreQuery(r.db.WithContext(ctx).Table("listing_store"), query)
	var rows []listingStore
	total, page, pageSize, err := findPagedRows(db, query.Page, query.PageSize, &rows)
	if err != nil {
		return nil, err
	}
	items := make([]Store, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toStore())
	}
	return &StorePage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *GormStoreRepository) GetStore(ctx context.Context, tenantID, id int64) (*Store, error) {
	var row listingStore
	err := takeStoreAccessRow(ctx, r.db.WithContext(ctx).Table("listing_store"), tenantID, id, 0, &row)
	if err != nil {
		return nil, err
	}
	store := row.toStore()
	return &store, nil
}

func (r *GormStoreRepository) CreateStore(ctx context.Context, store *Store) (*Store, error) {
	row := listingStoreFromStore(store)
	applyStoreCreateDefaults(&row)
	if row.OwnerUserID == "" {
		row.OwnerUserID = strings.TrimSpace(store.OwnerUserID)
	}
	if row.CreatedBy == "" {
		row.CreatedBy = strings.TrimSpace(store.CreatedBy)
	}
	if row.UpdatedBy == "" {
		row.UpdatedBy = strings.TrimSpace(store.UpdatedBy)
	}
	if err := r.db.WithContext(ctx).Table("listing_store").Create(&row).Error; err != nil {
		return nil, err
	}
	created := row.toStore()
	return &created, nil
}

func (r *GormStoreRepository) UpdateStore(ctx context.Context, store *Store) (*Store, error) {
	row := listingStoreFromStore(store)
	updates := map[string]any{
		"store_id":                   row.StoreID,
		"name":                       row.Name,
		"username":                   row.Username,
		"login_url":                  row.LoginURL,
		"shop_type":                  row.ShopType,
		"region":                     row.Region,
		"platform":                   row.Platform,
		"daily_limit":                row.DailyLimit,
		"daily_limit_type":           row.DailyLimitType,
		"fixed_stock_count":          row.FixedStockCount,
		"sku_generate_strategy":      row.SKUGenerateStrategy,
		"prefix":                     row.Prefix,
		"suffix":                     row.Suffix,
		"proxy":                      row.Proxy,
		"enable_auto_listing":        row.EnableAutoListing,
		"enable_auto_login":          row.EnableAutoLogin,
		"enable_draft":               row.EnableDraft,
		"enable_auto_price":          row.EnableAutoPrice,
		"enable_rebargain":           row.EnableRebargain,
		"enable_brand_authorization": row.EnableBrandAuthorization,
		"authorized_brand_code":      row.AuthorizedBrandCode,
		"authorized_brand_name":      row.AuthorizedBrandName,
		"temu_price_reject_strategy": row.TemuPriceRejectStrategy,
		"price_type":                 row.PriceType,
		"remark":                     row.Remark,
		"status":                     row.Status,
		"valid_from":                 row.ValidFrom,
		"valid_until":                row.ValidUntil,
		"expired":                    row.Expired,
		"dedicated_queue_enabled":    row.DedicatedQueueEnabled,
		"owner_user_id":              row.OwnerUserID,
		"updater":                    firstNonEmptyStoreValue(row.UpdatedBy, row.Updater),
		"updated_by":                 firstNonEmptyStoreValue(row.UpdatedBy, row.Updater),
	}
	if strings.TrimSpace(row.Password) != "" {
		updates["password"] = row.Password
	}
	if err := updateStoreAccessRow(ctx, r.db.WithContext(ctx).Table("listing_store"), row.TenantID, row.ID, 0, updates); err != nil {
		return nil, err
	}
	return r.GetStore(ctx, row.TenantID, row.ID)
}

func (r *GormStoreRepository) UpdateStoreID(ctx context.Context, id int64, storeID string) (*Store, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("store repository database is not configured")
	}
	store, err := r.FindStoreByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := updateStoreAccessRow(ctx, r.db.WithContext(ctx).Table("listing_store"), store.TenantID, id, 0, map[string]any{
		"store_id": storeID,
	}); err != nil {
		return nil, err
	}
	return r.FindStoreByID(ctx, id)
}

func (r *GormStoreRepository) UpdateStoreStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*Store, error) {
	updates := map[string]any{
		"status": status,
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	if strings.TrimSpace(remark) != "" {
		updates["remark"] = strings.TrimSpace(remark)
	}
	if err := updateStoreAccessRow(ctx, r.db.WithContext(ctx).Table("listing_store"), tenantID, id, 0, updates); err != nil {
		return nil, err
	}
	return r.GetStore(ctx, tenantID, id)
}

func (r *GormStoreRepository) DeleteStore(ctx context.Context, tenantID, id int64) error {
	return updateStoreAccessRow(ctx, r.db.WithContext(ctx).Table("listing_store"), tenantID, id, 0, map[string]any{"deleted": 1})
}

func (r *GormStoreRepository) ListDeletedStores(ctx context.Context, tenantID int64) ([]Store, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("store repository database is not configured")
	}
	deleted := int16(1)
	rows, err := findStoreRows(ctx, r.db.WithContext(ctx).Table("listing_store"), StoreQuery{TenantID: tenantID, Deleted: &deleted})
	if err != nil {
		return nil, err
	}
	items := make([]Store, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toStore())
	}
	return items, nil
}

func (r *GormStoreRepository) RestoreStore(ctx context.Context, tenantID, id int64) (*Store, error) {
	if err := updateStoreAccessRow(ctx, r.db.WithContext(ctx).Table("listing_store"), tenantID, id, 1, map[string]any{"deleted": 0}); err != nil {
		return nil, err
	}
	return r.GetStore(ctx, tenantID, id)
}

func (r *GormStoreRepository) PermanentlyDeleteStore(ctx context.Context, tenantID, id int64) error {
	return deleteStoreAccessRow(ctx, r.db.WithContext(ctx).Table("listing_store"), tenantID, id, 1)
}

func (r *GormStoreRepository) ExtendStoreValidity(ctx context.Context, tenantID, id int64, days int) (*Store, error) {
	if days <= 0 {
		days = 30
	}
	var row listingStore
	err := takeStoreAccessRow(ctx, r.db.WithContext(ctx).Table("listing_store"), tenantID, id, 0, &row)
	if err != nil {
		return nil, err
	}
	base := time.Now()
	if row.ValidUntil != nil {
		base = *row.ValidUntil
	}
	nextValidUntil := base.AddDate(0, 0, days)
	if err := updateStoreAccessRow(ctx, r.db.WithContext(ctx).Table("listing_store"), tenantID, id, 0, map[string]any{
		"valid_until": nextValidUntil,
		"expired":     false,
		"updater":     requestUserIDFromContext(ctx),
		"updated_by":  requestUserIDFromContext(ctx),
	}); err != nil {
		return nil, err
	}
	return r.GetStore(ctx, tenantID, id)
}

func (r *GormStoreRepository) FindStoreByID(ctx context.Context, id int64) (*Store, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("store repository database is not configured")
	}
	var row listingStore
	err := applyStoreAccessScope(r.db.WithContext(ctx).Table("listing_store"), StoreQuery{}).
		Where("id = ? AND deleted = 0", id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrStoreNotFound
	}
	if err != nil {
		return nil, err
	}
	store := row.toStore()
	return &store, nil
}
