package listingadmin

import (
	"context"
	"errors"
	"time"
)

var ErrPricingRuleNotFound = errors.New("pricing rule not found")

type PricingRule struct {
	ID              int64      `json:"id"`
	TenantID        int64      `json:"tenantId"`
	Name            string     `json:"name"`
	RuleCode        string     `json:"ruleCode"`
	Description     string     `json:"description,omitempty"`
	Remark          string     `json:"remark,omitempty"`
	StoreID         *int64     `json:"storeId,omitempty"`
	CategoryID      *int64     `json:"categoryId,omitempty"`
	PriceMin        float64    `json:"priceMin"`
	PriceMax        float64    `json:"priceMax"`
	RuleType        string     `json:"ruleType"`
	RuleValue       float64    `json:"ruleValue"`
	FixedValue      *float64   `json:"fixedValue,omitempty"`
	AcceptCondition string     `json:"acceptCondition,omitempty"`
	RejectCondition string     `json:"rejectCondition,omitempty"`
	Status          int16      `json:"status"`
	CreateTime      *time.Time `json:"createTime,omitempty"`
	UpdateTime      *time.Time `json:"updateTime,omitempty"`
}

type PricingRuleQuery struct {
	TenantID    int64
	OwnerUserID string
	Page        int
	PageSize    int
	Name        string
	RuleCode    string
	StoreID     *int64
	CategoryID  *int64
	RuleType    string
	Status      *int16
}

type PricingRulePage struct {
	Items    []PricingRule `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

type PricingRuleRepository interface {
	ListPricingRules(ctx context.Context, query PricingRuleQuery) (*PricingRulePage, error)
	GetPricingRule(ctx context.Context, tenantID, id int64) (*PricingRule, error)
	CreatePricingRule(ctx context.Context, rule *PricingRule) (*PricingRule, error)
	UpdatePricingRule(ctx context.Context, rule *PricingRule) (*PricingRule, error)
	UpdatePricingRuleStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*PricingRule, error)
	DeletePricingRule(ctx context.Context, tenantID, id int64) error
}

type listingPricingRule struct {
	ID              int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID        int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID     string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	Name            string     `gorm:"column:name;not null"`
	RuleCode        string     `gorm:"column:rule_code;not null;index"`
	Description     string     `gorm:"column:description"`
	Remark          string     `gorm:"column:remark"`
	StoreID         int64      `gorm:"column:store_id;index"`
	CategoryID      int64      `gorm:"column:category_id;index"`
	PriceMin        float64    `gorm:"column:price_min;not null;default:0"`
	PriceMax        float64    `gorm:"column:price_max;not null;default:99999"`
	RuleType        string     `gorm:"column:rule_type;not null;index"`
	RuleValue       float64    `gorm:"column:rule_value;not null"`
	FixedValue      float64    `gorm:"column:fixed_value"`
	AcceptCondition string     `gorm:"column:accept_condition"`
	RejectCondition string     `gorm:"column:reject_condition"`
	Status          int16      `gorm:"column:status;not null;default:0;index"`
	Creator         string     `gorm:"column:creator"`
	CreatedBy       string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime      *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater         string     `gorm:"column:updater"`
	UpdatedBy       string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime      *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted         int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingPricingRule) TableName() string {
	return "listing_pricing_rule"
}
