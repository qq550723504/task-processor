package listingadmin

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

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
	rows, total, page, pageSize, err := findFilterRuleRows(ctx, r.db.WithContext(ctx).Table("listing_filter_rule"), query)
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
		applyFilterRuleAuditFields(&row, ownerUserID, true)
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
		applyFilterRuleAuditFields(&row, ownerUserID, false)
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
