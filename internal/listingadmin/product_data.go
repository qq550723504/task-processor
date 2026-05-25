package listingadmin

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var ErrProductDataNotFound = errors.New("product data not found")

type ProductData struct {
	ID                int64           `json:"id"`
	TenantID          int64           `json:"tenantId"`
	Source            string          `json:"source,omitempty"`
	ImportTaskID      *int64          `json:"importTaskId,omitempty"`
	RawJSONDataID     *int64          `json:"rawJsonDataId,omitempty"`
	StoreID           *int64          `json:"storeId,omitempty"`
	CategoryID        *int64          `json:"categoryId,omitempty"`
	Platform          string          `json:"platform,omitempty"`
	Region            string          `json:"region,omitempty"`
	ParentProductID   string          `json:"parentProductId,omitempty"`
	ProductID         string          `json:"productId"`
	Title             string          `json:"title,omitempty"`
	Description       string          `json:"description,omitempty"`
	OriginalPrice     float64         `json:"originalPrice,omitempty"`
	SpecialPrice      float64         `json:"specialPrice,omitempty"`
	PriceCurrency     string          `json:"priceCurrency,omitempty"`
	Stock             string          `json:"stock,omitempty"`
	Brand             string          `json:"brand,omitempty"`
	Category          string          `json:"category,omitempty"`
	MainImageURL      string          `json:"mainImageUrl,omitempty"`
	ImageURLs         json.RawMessage `json:"imageUrls,omitempty"`
	Attributes        json.RawMessage `json:"attributes,omitempty"`
	SourceURL         string          `json:"sourceUrl,omitempty"`
	Status            int16           `json:"status"`
	PlatformProductID string          `json:"platformProductId,omitempty"`
	PlatformStatus    string          `json:"platformStatus,omitempty"`
	ShelfStatus       *int            `json:"shelfStatus,omitempty"`
	PublishTime       *time.Time      `json:"publishTime,omitempty"`
	ShelfTime         *time.Time      `json:"shelfTime,omitempty"`
	LastSyncTime      *time.Time      `json:"lastSyncTime,omitempty"`
	PlatformData      json.RawMessage `json:"platformData,omitempty"`
	CreateTime        *time.Time      `json:"createTime,omitempty"`
	UpdateTime        *time.Time      `json:"updateTime,omitempty"`
}

type ProductDataQuery struct {
	TenantID          int64
	OwnerUserID       string
	Page              int
	PageSize          int
	StoreID           *int64
	CategoryID        *int64
	Platform          string
	Region            string
	ProductID         string
	ParentProductID   string
	Title             string
	Brand             string
	Status            *int16
	PlatformProductID string
	ShelfStatus       *int
}

type ProductDataPage struct {
	Items    []ProductData `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

type ProductDataRepository interface {
	ListProductData(ctx context.Context, query ProductDataQuery) (*ProductDataPage, error)
	GetProductData(ctx context.Context, tenantID, id int64) (*ProductData, error)
	CreateProductData(ctx context.Context, product *ProductData) (*ProductData, error)
	UpdateProductData(ctx context.Context, product *ProductData) (*ProductData, error)
	UpdateProductDataStatus(ctx context.Context, tenantID, id int64, status int16) (*ProductData, error)
	DeleteProductData(ctx context.Context, tenantID, id int64) error
}

type listingProductData struct {
	ID                int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID          int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID       string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	Source            string     `gorm:"column:source"`
	ImportTaskID      int64      `gorm:"column:import_task_id;index"`
	RawJSONDataID     int64      `gorm:"column:raw_json_data_id;index"`
	StoreID           int64      `gorm:"column:store_id;index"`
	CategoryID        int64      `gorm:"column:category_id;index"`
	Platform          string     `gorm:"column:platform;index"`
	Region            string     `gorm:"column:region;index"`
	ParentProductID   string     `gorm:"column:parent_product_id;index"`
	ProductID         string     `gorm:"column:product_id;index"`
	Title             string     `gorm:"column:title"`
	Description       string     `gorm:"column:description"`
	OriginalPrice     float64    `gorm:"column:original_price"`
	SpecialPrice      float64    `gorm:"column:special_price"`
	PriceCurrency     string     `gorm:"column:price_currency"`
	Stock             string     `gorm:"column:stock"`
	Brand             string     `gorm:"column:brand"`
	Category          string     `gorm:"column:category"`
	MainImageURL      string     `gorm:"column:main_image_url"`
	ImageURLs         string     `gorm:"column:image_urls"`
	Attributes        string     `gorm:"column:attributes"`
	SourceURL         string     `gorm:"column:source_url"`
	Status            int16      `gorm:"column:status;not null;default:0;index"`
	PlatformProductID string     `gorm:"column:platform_product_id;index"`
	PlatformStatus    string     `gorm:"column:platform_status"`
	ShelfStatus       int        `gorm:"column:shelf_status;index"`
	PublishTime       *time.Time `gorm:"column:publish_time"`
	ShelfTime         *time.Time `gorm:"column:shelf_time"`
	LastSyncTime      *time.Time `gorm:"column:last_sync_time"`
	PlatformData      string     `gorm:"column:platform_data"`
	Creator           string     `gorm:"column:creator"`
	CreatedBy         string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime        *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater           string     `gorm:"column:updater"`
	UpdatedBy         string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime        *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted           int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingProductData) TableName() string {
	return "listing_product_data"
}
