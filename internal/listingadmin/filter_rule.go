package listingadmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
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

func (r listingFilterRule) toFilterRule() FilterRule {
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
	var deliveryTimeMax *int
	if r.DeliveryTimeMax > 0 {
		value := r.DeliveryTimeMax
		deliveryTimeMax = &value
	}
	return FilterRule{
		ID:              r.ID,
		TenantID:        r.TenantID,
		Name:            r.Name,
		RuleCode:        r.RuleCode,
		Description:     r.Description,
		StoreID:         storeID,
		CategoryID:      categoryID,
		PriceType:       r.PriceType,
		PriceMin:        r.PriceMin,
		PriceMax:        r.PriceMax,
		StockMin:        r.StockMin,
		RatingMin:       r.RatingMin,
		ReviewCountMin:  r.ReviewCountMin,
		DeliveryTimeMax: deliveryTimeMax,
		FulfillmentType: r.FulfillmentType,
		Status:          r.Status,
		Remark:          r.Remark,
		CreateTime:      r.CreateTime,
		UpdateTime:      r.UpdateTime,
	}
}

func listingFilterRuleFromFilterRule(rule *FilterRule) listingFilterRule {
	if rule == nil {
		return listingFilterRule{}
	}
	var storeID int64
	if rule.StoreID != nil {
		storeID = *rule.StoreID
	}
	var categoryID int64
	if rule.CategoryID != nil {
		categoryID = *rule.CategoryID
	}
	var deliveryTimeMax int
	if rule.DeliveryTimeMax != nil {
		deliveryTimeMax = *rule.DeliveryTimeMax
	}
	return listingFilterRule{
		ID:              rule.ID,
		TenantID:        rule.TenantID,
		Name:            strings.TrimSpace(rule.Name),
		RuleCode:        strings.TrimSpace(rule.RuleCode),
		Description:     strings.TrimSpace(rule.Description),
		StoreID:         storeID,
		CategoryID:      categoryID,
		PriceType:       strings.TrimSpace(rule.PriceType),
		PriceMin:        rule.PriceMin,
		PriceMax:        rule.PriceMax,
		StockMin:        rule.StockMin,
		RatingMin:       rule.RatingMin,
		ReviewCountMin:  rule.ReviewCountMin,
		DeliveryTimeMax: deliveryTimeMax,
		FulfillmentType: strings.TrimSpace(rule.FulfillmentType),
		Status:          rule.Status,
		Remark:          strings.TrimSpace(rule.Remark),
	}
}

type GormFilterRuleRepository struct {
	db *gorm.DB
}

func NewGormFilterRuleRepository(db *gorm.DB) *GormFilterRuleRepository {
	return &GormFilterRuleRepository{db: db}
}

func AutoMigrateFilterRuleRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	return ensureOwnerAuditColumns(db, (listingFilterRule{}).TableName())
}

func (r *GormFilterRuleRepository) ListFilterRules(ctx context.Context, query FilterRuleQuery) (*FilterRulePage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("filter rule repository database is not configured")
	}
	db := applyFilterRuleQuery(r.db.WithContext(ctx).Table("listing_filter_rule"), query)
	var rows []listingFilterRule
	total, page, pageSize, err := findPagedRows(db, query.Page, query.PageSize, &rows)
	if err != nil {
		return nil, err
	}
	items := make([]FilterRule, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toFilterRule())
	}
	return &FilterRulePage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *GormFilterRuleRepository) GetFilterRule(ctx context.Context, tenantID, id int64) (*FilterRule, error) {
	var row listingFilterRule
	err := takeOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_filter_rule"), tenantID, id, "owner_user_id", &row, ErrFilterRuleNotFound)
	if err != nil {
		return nil, err
	}
	rule := row.toFilterRule()
	return &rule, nil
}

func (r *GormFilterRuleRepository) CreateFilterRule(ctx context.Context, rule *FilterRule) (*FilterRule, error) {
	row := listingFilterRuleFromFilterRule(rule)
	applyFilterRuleDefaults(&row)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		row.OwnerUserID = ownerUserID
		row.Creator = ownerUserID
		row.CreatedBy = ownerUserID
		row.Updater = ownerUserID
		row.UpdatedBy = ownerUserID
	}
	if err := r.db.WithContext(ctx).Table("listing_filter_rule").Create(&row).Error; err != nil {
		return nil, err
	}
	created := row.toFilterRule()
	return &created, nil
}

func (r *GormFilterRuleRepository) UpdateFilterRule(ctx context.Context, rule *FilterRule) (*FilterRule, error) {
	row := listingFilterRuleFromFilterRule(rule)
	applyFilterRuleDefaults(&row)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		row.OwnerUserID = ownerUserID
		row.Updater = ownerUserID
		row.UpdatedBy = ownerUserID
	}
	updates := map[string]any{
		"owner_user_id":     row.OwnerUserID,
		"name":              row.Name,
		"rule_code":         row.RuleCode,
		"description":       row.Description,
		"store_id":          row.StoreID,
		"category_id":       row.CategoryID,
		"price_type":        row.PriceType,
		"price_min":         row.PriceMin,
		"price_max":         row.PriceMax,
		"stock_min":         row.StockMin,
		"rating_min":        row.RatingMin,
		"review_count_min":  row.ReviewCountMin,
		"delivery_time_max": row.DeliveryTimeMax,
		"fulfillment_type":  row.FulfillmentType,
		"status":            row.Status,
		"remark":            row.Remark,
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_filter_rule"), row.TenantID, row.ID, "owner_user_id", updates, ErrFilterRuleNotFound); err != nil {
		return nil, err
	}
	return r.GetFilterRule(ctx, row.TenantID, row.ID)
}

func (r *GormFilterRuleRepository) UpdateFilterRuleStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*FilterRule, error) {
	updates := map[string]any{"status": status}
	if strings.TrimSpace(remark) != "" {
		updates["remark"] = strings.TrimSpace(remark)
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_filter_rule"), tenantID, id, "owner_user_id", updates, ErrFilterRuleNotFound); err != nil {
		return nil, err
	}
	return r.GetFilterRule(ctx, tenantID, id)
}

func (r *GormFilterRuleRepository) DeleteFilterRule(ctx context.Context, tenantID, id int64) error {
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	return updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_filter_rule"), tenantID, id, "owner_user_id", updates, ErrFilterRuleNotFound)
}

func applyFilterRuleDefaults(row *listingFilterRule) {
	if row.PriceMax <= 0 {
		row.PriceMax = 99999
	}
	if row.StockMin <= 0 {
		row.StockMin = 10
	}
	if row.FulfillmentType == "" {
		row.FulfillmentType = "ALL"
	}
}

func applyFilterRuleQuery(db *gorm.DB, query FilterRuleQuery) *gorm.DB {
	db = applyOwnedTenantQuery(db, query.TenantID, strings.TrimSpace(query.OwnerUserID))
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
	if query.PriceType != "" {
		db = db.Where("price_type = ?", query.PriceType)
	}
	if query.FulfillmentType != "" {
		db = db.Where("fulfillment_type = ?", query.FulfillmentType)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	return db
}
