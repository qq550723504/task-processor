package listingadmin

import (
	"context"
	"errors"
	"time"
)

var ErrProductImportMappingNotFound = errors.New("product import mapping not found")

type ProductImportMapping struct {
	ID                      int64      `json:"id"`
	TenantID                int64      `json:"tenantId"`
	ImportTaskID            int64      `json:"importTaskId"`
	StoreID                 int64      `json:"storeId"`
	Platform                string     `json:"platform"`
	Region                  string     `json:"region"`
	ProductID               string     `json:"productId"`
	ParentProductID         string     `json:"parentProductId,omitempty"`
	SKU                     string     `json:"sku,omitempty"`
	CostPrice               *float64   `json:"costPrice,omitempty"`
	PlatformProductID       string     `json:"platformProductId,omitempty"`
	PlatformParentProductID string     `json:"platformParentProductId,omitempty"`
	FilterRuleID            *int64     `json:"filterRuleId,omitempty"`
	FilterRuleRange         string     `json:"filterRuleRange,omitempty"`
	ProfitRuleID            *int64     `json:"profitRuleId,omitempty"`
	SalePriceMultiplier     float64    `json:"salePriceMultiplier"`
	DiscountPriceMultiplier float64    `json:"discountPriceMultiplier"`
	Status                  int16      `json:"status"`
	Remark                  string     `json:"remark,omitempty"`
	CreateTime              *time.Time `json:"createTime,omitempty"`
	UpdateTime              *time.Time `json:"updateTime,omitempty"`
}

type ProductImportMappingQuery struct {
	TenantID                int64
	OwnerUserID             string
	Page                    int
	PageSize                int
	ImportTaskID            *int64
	StoreID                 *int64
	Platform                string
	Region                  string
	ProductID               string
	ParentProductID         string
	SKU                     string
	PlatformProductID       string
	PlatformParentProductID string
	Status                  *int16
}

type ProductImportMappingPage struct {
	Items    []ProductImportMapping `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

type ProductImportMappingRepository interface {
	ListProductImportMappings(ctx context.Context, query ProductImportMappingQuery) (*ProductImportMappingPage, error)
	GetProductImportMapping(ctx context.Context, tenantID, id int64) (*ProductImportMapping, error)
	CreateProductImportMapping(ctx context.Context, mapping *ProductImportMapping) (*ProductImportMapping, error)
	UpdateProductImportMapping(ctx context.Context, mapping *ProductImportMapping) (*ProductImportMapping, error)
	UpdateProductImportMappingStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*ProductImportMapping, error)
	DeleteProductImportMapping(ctx context.Context, tenantID, id int64) error
}

type listingProductImportMapping struct {
	ID                      int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID                int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID             string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	ImportTaskID            int64      `gorm:"column:import_task_id;not null;index"`
	StoreID                 int64      `gorm:"column:store_id;not null;index"`
	Platform                string     `gorm:"column:platform;not null;index"`
	Region                  string     `gorm:"column:region;not null;index"`
	ProductID               string     `gorm:"column:product_id;not null;index"`
	ParentProductID         string     `gorm:"column:parent_product_id;index"`
	SKU                     string     `gorm:"column:sku;index"`
	CostPrice               float64    `gorm:"column:cost_price"`
	PlatformProductID       string     `gorm:"column:platform_product_id;index"`
	PlatformParentProductID string     `gorm:"column:platform_parent_product_id;index"`
	FilterRuleID            int64      `gorm:"column:filter_rule_id;index"`
	FilterRuleRange         string     `gorm:"column:filter_rule_range"`
	ProfitRuleID            int64      `gorm:"column:profit_rule_id;index"`
	SalePriceMultiplier     float64    `gorm:"column:sale_price_multiplier;not null;default:1"`
	DiscountPriceMultiplier float64    `gorm:"column:discount_price_multiplier;not null;default:1"`
	Status                  int16      `gorm:"column:status;not null;default:0;index"`
	Remark                  string     `gorm:"column:remark"`
	Creator                 string     `gorm:"column:creator"`
	CreatedBy               string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime              *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater                 string     `gorm:"column:updater"`
	UpdatedBy               string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime              *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted                 int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingProductImportMapping) TableName() string {
	return "listing_product_import_mapping"
}

func int64PtrIfPositive(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	out := value
	return &out
}

func int64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
