package listingadmin

import (
	"context"
	"errors"
	"time"
)

var ErrProfitRuleNotFound = errors.New("profit rule not found")

type ProfitRule struct {
	ID                      int64      `json:"id"`
	TenantID                int64      `json:"tenantId"`
	Name                    string     `json:"name"`
	RuleCode                string     `json:"ruleCode"`
	Description             string     `json:"description,omitempty"`
	StoreID                 *int64     `json:"storeId,omitempty"`
	CategoryID              *int64     `json:"categoryId,omitempty"`
	SalePriceMultiplier     float64    `json:"salePriceMultiplier"`
	DiscountPriceMultiplier float64    `json:"discountPriceMultiplier"`
	Status                  int16      `json:"status"`
	Remark                  string     `json:"remark,omitempty"`
	CreateTime              *time.Time `json:"createTime,omitempty"`
	UpdateTime              *time.Time `json:"updateTime,omitempty"`
}

type ProfitRuleQuery struct {
	TenantID    int64
	OwnerUserID string
	Page        int
	PageSize    int
	Name        string
	RuleCode    string
	StoreID     *int64
	CategoryID  *int64
	Status      *int16
}

type ProfitRulePage struct {
	Items    []ProfitRule `json:"items"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

type ProfitRuleRepository interface {
	ListProfitRules(ctx context.Context, query ProfitRuleQuery) (*ProfitRulePage, error)
	GetProfitRule(ctx context.Context, tenantID, id int64) (*ProfitRule, error)
	CreateProfitRule(ctx context.Context, rule *ProfitRule) (*ProfitRule, error)
	UpdateProfitRule(ctx context.Context, rule *ProfitRule) (*ProfitRule, error)
	UpdateProfitRuleStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*ProfitRule, error)
	DeleteProfitRule(ctx context.Context, tenantID, id int64) error
}

type listingProfitRule struct {
	ID                      int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID                int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID             string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	Name                    string     `gorm:"column:name;not null"`
	RuleCode                string     `gorm:"column:rule_code;not null;index"`
	Description             string     `gorm:"column:description"`
	StoreID                 int64      `gorm:"column:store_id;index"`
	CategoryID              int64      `gorm:"column:category_id;index"`
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

func (listingProfitRule) TableName() string {
	return "listing_profit_rule"
}
