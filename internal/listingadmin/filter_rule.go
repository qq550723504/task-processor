package listingadmin

import (
	"context"
	"errors"
	"time"
)

var ErrFilterRuleNotFound = errors.New("filter rule not found")

type FilterRule struct {
	ID              int64      `json:"id"`
	TenantID        int64      `json:"tenantId"`
	Name            string     `json:"name"`
	RuleCode        string     `json:"ruleCode"`
	Description     string     `json:"description,omitempty"`
	StoreID         *int64     `json:"storeId,omitempty"`
	CategoryID      *int64     `json:"categoryId,omitempty"`
	PriceType       string     `json:"priceType,omitempty"`
	PriceMin        float64    `json:"priceMin"`
	PriceMax        float64    `json:"priceMax"`
	StockMin        int        `json:"stockMin"`
	RatingMin       float64    `json:"ratingMin"`
	ReviewCountMin  int        `json:"reviewCountMin"`
	DeliveryTimeMax *int       `json:"deliveryTimeMax,omitempty"`
	FulfillmentType string     `json:"fulfillmentType,omitempty"`
	Status          int16      `json:"status"`
	Remark          string     `json:"remark,omitempty"`
	CreateTime      *time.Time `json:"createTime,omitempty"`
	UpdateTime      *time.Time `json:"updateTime,omitempty"`
}

type FilterRuleQuery struct {
	TenantID        int64
	OwnerUserID     string
	Page            int
	PageSize        int
	Name            string
	RuleCode        string
	StoreID         *int64
	CategoryID      *int64
	PriceType       string
	FulfillmentType string
	Status          *int16
}

type FilterRulePage struct {
	Items    []FilterRule `json:"items"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

type FilterRuleRepository interface {
	ListFilterRules(ctx context.Context, query FilterRuleQuery) (*FilterRulePage, error)
	GetFilterRule(ctx context.Context, tenantID, id int64) (*FilterRule, error)
	ResolveFilterRules(ctx context.Context, tenantID, storeID, categoryID int64) ([]FilterRule, error)
	CreateFilterRule(ctx context.Context, rule *FilterRule) (*FilterRule, error)
	UpdateFilterRule(ctx context.Context, rule *FilterRule) (*FilterRule, error)
	UpdateFilterRuleStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*FilterRule, error)
	DeleteFilterRule(ctx context.Context, tenantID, id int64) error
}

type listingFilterRule struct {
	ID              int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID        int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID     string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	Name            string     `gorm:"column:name;not null"`
	RuleCode        string     `gorm:"column:rule_code;not null;index"`
	Description     string     `gorm:"column:description"`
	StoreID         int64      `gorm:"column:store_id;index"`
	CategoryID      int64      `gorm:"column:category_id;index"`
	PriceType       string     `gorm:"column:price_type"`
	PriceMin        float64    `gorm:"column:price_min;not null;default:0"`
	PriceMax        float64    `gorm:"column:price_max;not null;default:99999"`
	StockMin        int        `gorm:"column:stock_min;not null;default:10"`
	RatingMin       float64    `gorm:"column:rating_min;not null;default:0"`
	ReviewCountMin  int        `gorm:"column:review_count_min;not null;default:0"`
	DeliveryTimeMax int        `gorm:"column:delivery_time_max"`
	FulfillmentType string     `gorm:"column:fulfillment_type"`
	Status          int16      `gorm:"column:status;not null;default:0;index"`
	Remark          string     `gorm:"column:remark"`
	Creator         string     `gorm:"column:creator"`
	CreatedBy       string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime      *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater         string     `gorm:"column:updater"`
	UpdatedBy       string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime      *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted         int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingFilterRule) TableName() string {
	return "listing_filter_rule"
}
