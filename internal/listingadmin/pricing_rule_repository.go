package listingadmin

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

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
	rows, total, page, pageSize, err := findPricingRuleRows(ctx, r.db.WithContext(ctx).Table("listing_pricing_rule"), query)
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
	err := takeOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_pricing_rule"), tenantID, id, "owner_user_id", &row, ErrPricingRuleNotFound)
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
		applyPricingRuleAuditFields(&row, ownerUserID, true)
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
		applyPricingRuleAuditFields(&row, ownerUserID, false)
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
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_pricing_rule"), row.TenantID, row.ID, "owner_user_id", updates, ErrPricingRuleNotFound); err != nil {
		return nil, err
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
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_pricing_rule"), tenantID, id, "owner_user_id", updates, ErrPricingRuleNotFound); err != nil {
		return nil, err
	}
	return r.GetPricingRule(ctx, tenantID, id)
}

func (r *GormPricingRuleRepository) DeletePricingRule(ctx context.Context, tenantID, id int64) error {
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	return updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_pricing_rule"), tenantID, id, "owner_user_id", updates, ErrPricingRuleNotFound)
}

func (r *GormPricingRuleRepository) ListByStoreID(ctx context.Context, storeID int64) ([]PricingRule, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("pricing rule repository database is not configured")
	}
	var rows []listingPricingRule
	err := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_pricing_rule").Where("store_id = ? AND deleted = 0", storeID),
		ctx,
		"owner_user_id",
	).Order("id desc").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	items := make([]PricingRule, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toPricingRule())
	}
	return items, nil
}
