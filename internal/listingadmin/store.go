package listingadmin

import (
	"context"
	"errors"
	"time"
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
