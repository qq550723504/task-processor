package listingadmin

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

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
	return ensureOwnerAuditColumns(db, (listingProfitRule{}).TableName())
}

func (r *GormProfitRuleRepository) ListProfitRules(ctx context.Context, query ProfitRuleQuery) (*ProfitRulePage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("profit rule repository database is not configured")
	}
	rows, total, page, pageSize, err := findProfitRuleRows(ctx, r.db.WithContext(ctx).Table("listing_profit_rule"), query)
	if err != nil {
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
	err := takeOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_profit_rule"), tenantID, id, "owner_user_id", &row, ErrProfitRuleNotFound)
	if err != nil {
		return nil, err
	}
	rule := row.toProfitRule()
	return &rule, nil
}

func (r *GormProfitRuleRepository) CreateProfitRule(ctx context.Context, rule *ProfitRule) (*ProfitRule, error) {
	row := listingProfitRuleFromProfitRule(rule)
	applyProfitRuleDefaults(&row)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		applyProfitRuleAuditFields(&row, ownerUserID, true)
	}
	if err := r.db.WithContext(ctx).Table("listing_profit_rule").Create(&row).Error; err != nil {
		return nil, err
	}
	created := row.toProfitRule()
	return &created, nil
}

func (r *GormProfitRuleRepository) UpdateProfitRule(ctx context.Context, rule *ProfitRule) (*ProfitRule, error) {
	row := listingProfitRuleFromProfitRule(rule)
	applyProfitRuleDefaults(&row)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		applyProfitRuleAuditFields(&row, ownerUserID, false)
	}
	updates := map[string]any{
		"owner_user_id":             row.OwnerUserID,
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
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_profit_rule"), row.TenantID, row.ID, "owner_user_id", updates, ErrProfitRuleNotFound); err != nil {
		return nil, err
	}
	return r.GetProfitRule(ctx, row.TenantID, row.ID)
}

func (r *GormProfitRuleRepository) UpdateProfitRuleStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*ProfitRule, error) {
	updates := map[string]any{"status": status}
	if strings.TrimSpace(remark) != "" {
		updates["remark"] = strings.TrimSpace(remark)
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_profit_rule"), tenantID, id, "owner_user_id", updates, ErrProfitRuleNotFound); err != nil {
		return nil, err
	}
	return r.GetProfitRule(ctx, tenantID, id)
}

func (r *GormProfitRuleRepository) DeleteProfitRule(ctx context.Context, tenantID, id int64) error {
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	return updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_profit_rule"), tenantID, id, "owner_user_id", updates, ErrProfitRuleNotFound)
}

func (r *GormProfitRuleRepository) ResolveProfitRule(ctx context.Context, tenantID, storeID int64) (*ProfitRule, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("profit rule repository database is not configured")
	}
	load := func(db *gorm.DB) (*ProfitRule, error) {
		var row listingProfitRule
		err := db.Order("id desc").Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		rule := row.toProfitRule()
		return &rule, nil
	}

	base := func() *gorm.DB {
		return applyOwnerScope(
			r.db.WithContext(ctx).Table("listing_profit_rule").Where("tenant_id = ? AND deleted = 0", tenantID),
			ctx,
			"owner_user_id",
		)
	}
	rule, err := load(base().Where("store_id = ?", storeID))
	if err != nil || rule != nil {
		return rule, err
	}
	return load(base().Where("store_id IS NULL OR store_id = 0"))
}
