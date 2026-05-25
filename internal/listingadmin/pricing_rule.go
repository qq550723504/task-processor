package listingadmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
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

func (r listingPricingRule) toPricingRule() PricingRule {
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
	var fixedValue *float64
	if r.FixedValue != 0 {
		value := r.FixedValue
		fixedValue = &value
	}
	return PricingRule{
		ID:              r.ID,
		TenantID:        r.TenantID,
		Name:            r.Name,
		RuleCode:        r.RuleCode,
		Description:     r.Description,
		Remark:          r.Remark,
		StoreID:         storeID,
		CategoryID:      categoryID,
		PriceMin:        r.PriceMin,
		PriceMax:        r.PriceMax,
		RuleType:        r.RuleType,
		RuleValue:       r.RuleValue,
		FixedValue:      fixedValue,
		AcceptCondition: r.AcceptCondition,
		RejectCondition: r.RejectCondition,
		Status:          r.Status,
		CreateTime:      r.CreateTime,
		UpdateTime:      r.UpdateTime,
	}
}

func listingPricingRuleFromPricingRule(rule *PricingRule) listingPricingRule {
	if rule == nil {
		return listingPricingRule{}
	}
	var storeID int64
	if rule.StoreID != nil {
		storeID = *rule.StoreID
	}
	var categoryID int64
	if rule.CategoryID != nil {
		categoryID = *rule.CategoryID
	}
	var fixedValue float64
	if rule.FixedValue != nil {
		fixedValue = *rule.FixedValue
	}
	return listingPricingRule{
		ID:              rule.ID,
		TenantID:        rule.TenantID,
		Name:            strings.TrimSpace(rule.Name),
		RuleCode:        strings.TrimSpace(rule.RuleCode),
		Description:     strings.TrimSpace(rule.Description),
		Remark:          strings.TrimSpace(rule.Remark),
		StoreID:         storeID,
		CategoryID:      categoryID,
		PriceMin:        rule.PriceMin,
		PriceMax:        rule.PriceMax,
		RuleType:        strings.TrimSpace(rule.RuleType),
		RuleValue:       rule.RuleValue,
		FixedValue:      fixedValue,
		AcceptCondition: strings.TrimSpace(rule.AcceptCondition),
		RejectCondition: strings.TrimSpace(rule.RejectCondition),
		Status:          rule.Status,
	}
}

type GormPricingRuleRepository struct{ db *gorm.DB }

func NewGormPricingRuleRepository(db *gorm.DB) *GormPricingRuleRepository {
	return &GormPricingRuleRepository{db: db}
}

func AutoMigratePricingRuleRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	return ensureOwnerAuditColumns(db, (listingPricingRule{}).TableName())
}

func (r *GormPricingRuleRepository) ListPricingRules(ctx context.Context, query PricingRuleQuery) (*PricingRulePage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("pricing rule repository database is not configured")
	}
	db := applyPricingRuleQuery(r.db.WithContext(ctx).Table("listing_pricing_rule"), query)
	var rows []listingPricingRule
	total, page, pageSize, err := findPagedRows(db, query.Page, query.PageSize, &rows)
	if err != nil {
		return nil, err
	}
	items := make([]PricingRule, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toPricingRule())
	}
	return &PricingRulePage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *GormPricingRuleRepository) GetPricingRule(ctx context.Context, tenantID, id int64) (*PricingRule, error) {
	var row listingPricingRule
	err := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_pricing_rule").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPricingRuleNotFound
	}
	if err != nil {
		return nil, err
	}
	rule := row.toPricingRule()
	return &rule, nil
}

func (r *GormPricingRuleRepository) CreatePricingRule(ctx context.Context, rule *PricingRule) (*PricingRule, error) {
	row := listingPricingRuleFromPricingRule(rule)
	applyPricingRuleDefaults(&row)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		row.OwnerUserID = ownerUserID
		row.Creator = ownerUserID
		row.CreatedBy = ownerUserID
		row.Updater = ownerUserID
		row.UpdatedBy = ownerUserID
	}
	if err := r.db.WithContext(ctx).Table("listing_pricing_rule").Create(&row).Error; err != nil {
		return nil, err
	}
	created := row.toPricingRule()
	return &created, nil
}

func (r *GormPricingRuleRepository) UpdatePricingRule(ctx context.Context, rule *PricingRule) (*PricingRule, error) {
	row := listingPricingRuleFromPricingRule(rule)
	applyPricingRuleDefaults(&row)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		row.OwnerUserID = ownerUserID
		row.Updater = ownerUserID
		row.UpdatedBy = ownerUserID
	}
	updates := map[string]any{
		"owner_user_id":    row.OwnerUserID,
		"name":             row.Name,
		"rule_code":        row.RuleCode,
		"description":      row.Description,
		"remark":           row.Remark,
		"store_id":         row.StoreID,
		"category_id":      row.CategoryID,
		"price_min":        row.PriceMin,
		"price_max":        row.PriceMax,
		"rule_type":        row.RuleType,
		"rule_value":       row.RuleValue,
		"fixed_value":      row.FixedValue,
		"accept_condition": row.AcceptCondition,
		"reject_condition": row.RejectCondition,
		"status":           row.Status,
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_pricing_rule").Where("tenant_id = ? AND id = ? AND deleted = 0", row.TenantID, row.ID),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrPricingRuleNotFound
	}
	return r.GetPricingRule(ctx, row.TenantID, row.ID)
}

func (r *GormPricingRuleRepository) UpdatePricingRuleStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*PricingRule, error) {
	updates := map[string]any{"status": status}
	if strings.TrimSpace(remark) != "" {
		updates["remark"] = strings.TrimSpace(remark)
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_pricing_rule").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrPricingRuleNotFound
	}
	return r.GetPricingRule(ctx, tenantID, id)
}

func (r *GormPricingRuleRepository) DeletePricingRule(ctx context.Context, tenantID, id int64) error {
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_pricing_rule").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPricingRuleNotFound
	}
	return nil
}

func applyPricingRuleDefaults(row *listingPricingRule) {
	if row.PriceMax <= 0 {
		row.PriceMax = 99999
	}
}

func applyPricingRuleQuery(db *gorm.DB, query PricingRuleQuery) *gorm.DB {
	db = db.Where("deleted = 0")
	if query.TenantID > 0 {
		db = db.Where("tenant_id = ?", query.TenantID)
	}
	if ownerScopeEnabled() && strings.TrimSpace(query.OwnerUserID) != "" {
		db = db.Where("owner_user_id = ?", strings.TrimSpace(query.OwnerUserID))
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
	if query.RuleType != "" {
		db = db.Where("rule_type = ?", query.RuleType)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	return db
}
