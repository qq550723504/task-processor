package listingadmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
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
	TenantID   int64
	Page       int
	PageSize   int
	Name       string
	RuleCode   string
	StoreID    *int64
	CategoryID *int64
	Status     *int16
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
	CreateTime              *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater                 string     `gorm:"column:updater"`
	UpdateTime              *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted                 int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingProfitRule) TableName() string {
	return "listing_profit_rule"
}

func (r listingProfitRule) toProfitRule() ProfitRule {
	var storeID *int64
	if r.StoreID > 0 {
		value := r.StoreID
		storeID = &value
	}
	var categoryID *int64
	if r.CategoryID > 0 {
		value := r.CategoryID
		categoryID = &value
	}
	return ProfitRule{
		ID:                      r.ID,
		TenantID:                r.TenantID,
		Name:                    r.Name,
		RuleCode:                r.RuleCode,
		Description:             r.Description,
		StoreID:                 storeID,
		CategoryID:              categoryID,
		SalePriceMultiplier:     r.SalePriceMultiplier,
		DiscountPriceMultiplier: r.DiscountPriceMultiplier,
		Status:                  r.Status,
		Remark:                  r.Remark,
		CreateTime:              r.CreateTime,
		UpdateTime:              r.UpdateTime,
	}
}

func listingProfitRuleFromProfitRule(rule *ProfitRule) listingProfitRule {
	if rule == nil {
		return listingProfitRule{}
	}
	var storeID int64
	if rule.StoreID != nil {
		storeID = *rule.StoreID
	}
	var categoryID int64
	if rule.CategoryID != nil {
		categoryID = *rule.CategoryID
	}
	return listingProfitRule{
		ID:                      rule.ID,
		TenantID:                rule.TenantID,
		Name:                    strings.TrimSpace(rule.Name),
		RuleCode:                strings.TrimSpace(rule.RuleCode),
		Description:             strings.TrimSpace(rule.Description),
		StoreID:                 storeID,
		CategoryID:              categoryID,
		SalePriceMultiplier:     rule.SalePriceMultiplier,
		DiscountPriceMultiplier: rule.DiscountPriceMultiplier,
		Status:                  rule.Status,
		Remark:                  strings.TrimSpace(rule.Remark),
	}
}

type GormProfitRuleRepository struct {
	db *gorm.DB
}

func NewGormProfitRuleRepository(db *gorm.DB) *GormProfitRuleRepository {
	return &GormProfitRuleRepository{db: db}
}

func AutoMigrateProfitRuleRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	return db.AutoMigrate(&listingProfitRule{})
}

func (r *GormProfitRuleRepository) ListProfitRules(ctx context.Context, query ProfitRuleQuery) (*ProfitRulePage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("profit rule repository database is not configured")
	}
	page, pageSize := normalizePage(query.Page, query.PageSize)
	db := applyProfitRuleQuery(r.db.WithContext(ctx).Table("listing_profit_rule"), query)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []listingProfitRule
	if err := db.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]ProfitRule, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toProfitRule())
	}
	return &ProfitRulePage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *GormProfitRuleRepository) GetProfitRule(ctx context.Context, tenantID, id int64) (*ProfitRule, error) {
	var row listingProfitRule
	err := r.db.WithContext(ctx).Table("listing_profit_rule").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProfitRuleNotFound
	}
	if err != nil {
		return nil, err
	}
	rule := row.toProfitRule()
	return &rule, nil
}

func (r *GormProfitRuleRepository) CreateProfitRule(ctx context.Context, rule *ProfitRule) (*ProfitRule, error) {
	row := listingProfitRuleFromProfitRule(rule)
	applyProfitRuleDefaults(&row)
	if err := r.db.WithContext(ctx).Table("listing_profit_rule").Create(&row).Error; err != nil {
		return nil, err
	}
	created := row.toProfitRule()
	return &created, nil
}

func (r *GormProfitRuleRepository) UpdateProfitRule(ctx context.Context, rule *ProfitRule) (*ProfitRule, error) {
	row := listingProfitRuleFromProfitRule(rule)
	applyProfitRuleDefaults(&row)
	updates := map[string]any{
		"name":                      row.Name,
		"rule_code":                 row.RuleCode,
		"description":               row.Description,
		"store_id":                  row.StoreID,
		"category_id":               row.CategoryID,
		"sale_price_multiplier":     row.SalePriceMultiplier,
		"discount_price_multiplier": row.DiscountPriceMultiplier,
		"status":                    row.Status,
		"remark":                    row.Remark,
	}
	res := r.db.WithContext(ctx).Table("listing_profit_rule").Where("tenant_id = ? AND id = ? AND deleted = 0", row.TenantID, row.ID).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrProfitRuleNotFound
	}
	return r.GetProfitRule(ctx, row.TenantID, row.ID)
}

func (r *GormProfitRuleRepository) UpdateProfitRuleStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*ProfitRule, error) {
	updates := map[string]any{"status": status}
	if strings.TrimSpace(remark) != "" {
		updates["remark"] = strings.TrimSpace(remark)
	}
	res := r.db.WithContext(ctx).Table("listing_profit_rule").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrProfitRuleNotFound
	}
	return r.GetProfitRule(ctx, tenantID, id)
}

func (r *GormProfitRuleRepository) DeleteProfitRule(ctx context.Context, tenantID, id int64) error {
	res := r.db.WithContext(ctx).Table("listing_profit_rule").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id).Update("deleted", 1)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrProfitRuleNotFound
	}
	return nil
}

func applyProfitRuleDefaults(row *listingProfitRule) {
	if row.SalePriceMultiplier <= 0 {
		row.SalePriceMultiplier = 1
	}
	if row.DiscountPriceMultiplier <= 0 {
		row.DiscountPriceMultiplier = 1
	}
}

func applyProfitRuleQuery(db *gorm.DB, query ProfitRuleQuery) *gorm.DB {
	db = db.Where("deleted = 0")
	if query.TenantID > 0 {
		db = db.Where("tenant_id = ?", query.TenantID)
	}
	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.RuleCode != "" {
		db = db.Where("rule_code LIKE ?", "%"+query.RuleCode+"%")
	}
	if query.StoreID != nil {
		db = db.Where("store_id = ?", *query.StoreID)
	}
	if query.CategoryID != nil {
		db = db.Where("category_id = ?", *query.CategoryID)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	return db
}
