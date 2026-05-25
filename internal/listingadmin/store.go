package listingadmin

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrStoreNotFound = errors.New("store not found")

type Store struct {
	ID                      int64      `json:"id"`
	TenantID                int64      `json:"tenantId"`
	OwnerUserID             string     `json:"ownerUserId,omitempty"`
	CreatedBy               string     `json:"createdBy,omitempty"`
	UpdatedBy               string     `json:"updatedBy,omitempty"`
	StoreID                 string     `json:"-"`
	Name                    string     `json:"name"`
	Username                string     `json:"username"`
	Password                string     `json:"password,omitempty"`
	LoginURL                string     `json:"loginUrl,omitempty"`
	ShopType                string     `json:"shopType"`
	Region                  string     `json:"region"`
	Platform                string     `json:"platform"`
	DailyLimit              *int       `json:"dailyLimit,omitempty"`
	DailyLimitType          string     `json:"dailyLimitType,omitempty"`
	FixedStockCount         *int       `json:"fixedStockCount,omitempty"`
	SKUGenerateStrategy     string     `json:"skuGenerateStrategy,omitempty"`
	Prefix                  string     `json:"prefix,omitempty"`
	Suffix                  string     `json:"suffix,omitempty"`
	Proxy                   string     `json:"proxy,omitempty"`
	EnableAutoListing       *bool      `json:"enableAutoListing,omitempty"`
	EnableAutoLogin         *bool      `json:"enableAutoLogin,omitempty"`
	EnableDraft             *bool      `json:"enableDraft,omitempty"`
	EnableAutoPrice         *bool      `json:"enableAutoPrice,omitempty"`
	EnableRebargain         *bool      `json:"enableRebargain,omitempty"`
	TemuPriceRejectStrategy string     `json:"temuPriceRejectStrategy,omitempty"`
	PriceType               string     `json:"priceType,omitempty"`
	Remark                  string     `json:"remark,omitempty"`
	Status                  int16      `json:"status"`
	ValidFrom               *time.Time `json:"validFrom,omitempty"`
	ValidUntil              *time.Time `json:"validUntil,omitempty"`
	Expired                 bool       `json:"expired"`
	DedicatedQueueEnabled   *bool      `json:"dedicatedQueueEnabled,omitempty"`
	CreateTime              *time.Time `json:"createTime,omitempty"`
	UpdateTime              *time.Time `json:"updateTime,omitempty"`
}

type StoreQuery struct {
	TenantID          int64
	OwnerUserID       string
	Page              int
	PageSize          int
	StoreID           string
	Name              string
	Username          string
	ShopType          string
	Region            string
	Platform          string
	SKUGenerate       string
	EnableAutoListing *bool
	EnableAutoLogin   *bool
	EnableDraft       *bool
	EnableAutoPrice   *bool
	EnableRebargain   *bool
	PriceType         string
	Status            *int16
	Expired           *bool
	Deleted           *int16
}

type StorePage struct {
	Items    []Store `json:"items"`
	Total    int64   `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
}

type StoreRepository interface {
	ListStores(ctx context.Context, query StoreQuery) (*StorePage, error)
	GetStore(ctx context.Context, tenantID, id int64) (*Store, error)
	CreateStore(ctx context.Context, store *Store) (*Store, error)
	UpdateStore(ctx context.Context, store *Store) (*Store, error)
	UpdateStoreStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*Store, error)
	DeleteStore(ctx context.Context, tenantID, id int64) error
	ListDeletedStores(ctx context.Context, tenantID int64) ([]Store, error)
	RestoreStore(ctx context.Context, tenantID, id int64) (*Store, error)
	PermanentlyDeleteStore(ctx context.Context, tenantID, id int64) error
	ExtendStoreValidity(ctx context.Context, tenantID, id int64, days int) (*Store, error)
}

type listingStore struct {
	ID                      int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID                int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID             string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	StoreID                 string     `gorm:"column:store_id"`
	Name                    string     `gorm:"column:name;not null"`
	Username                string     `gorm:"column:username;not null"`
	Password                string     `gorm:"column:password;not null"`
	LoginURL                string     `gorm:"column:login_url"`
	ShopType                string     `gorm:"column:shop_type;not null"`
	Region                  string     `gorm:"column:region"`
	Platform                string     `gorm:"column:platform;not null"`
	DailyLimit              *int       `gorm:"column:daily_limit"`
	DailyLimitType          string     `gorm:"column:daily_limit_type"`
	FixedStockCount         *int       `gorm:"column:fixed_stock_count"`
	SKUGenerateStrategy     string     `gorm:"column:sku_generate_strategy"`
	Prefix                  string     `gorm:"column:prefix"`
	Suffix                  string     `gorm:"column:suffix"`
	Proxy                   string     `gorm:"column:proxy"`
	EnableAutoListing       *bool      `gorm:"column:enable_auto_listing"`
	EnableAutoLogin         *bool      `gorm:"column:enable_auto_login"`
	EnableDraft             *bool      `gorm:"column:enable_draft"`
	EnableAutoPrice         *bool      `gorm:"column:enable_auto_price"`
	EnableRebargain         *bool      `gorm:"column:enable_rebargain"`
	TemuPriceRejectStrategy string     `gorm:"column:temu_price_reject_strategy"`
	PriceType               string     `gorm:"column:price_type"`
	Remark                  string     `gorm:"column:remark"`
	Status                  int16      `gorm:"column:status"`
	ValidFrom               *time.Time `gorm:"column:valid_from"`
	ValidUntil              *time.Time `gorm:"column:valid_until"`
	Expired                 bool       `gorm:"column:expired"`
	DedicatedQueueEnabled   *bool      `gorm:"column:dedicated_queue_enabled"`
	Creator                 string     `gorm:"column:creator"`
	CreatedBy               string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime              *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater                 string     `gorm:"column:updater"`
	UpdatedBy               string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime              *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted                 int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingStore) TableName() string {
	return "listing_store"
}

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
	var rows []listingStore
	db := r.db.WithContext(ctx).Table("listing_store").Where("deleted = 1")
	if tenantID > 0 {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if ownerScopeEnabled() {
		if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
			db = db.Where("owner_user_id = ?", ownerUserID)
		}
	}
	if err := db.Order("id desc").Find(&rows).Error; err != nil {
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
