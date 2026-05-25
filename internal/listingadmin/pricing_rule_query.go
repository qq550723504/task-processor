package listingadmin

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

func findPricingRuleRows(ctx context.Context, db *gorm.DB, query PricingRuleQuery) ([]listingPricingRule, int64, int, int, error) {
	scopeQuery := query
	if strings.TrimSpace(scopeQuery.OwnerUserID) == "" {
		scopeQuery.OwnerUserID = requestUserIDFromContext(ctx)
	}
	var rows []listingPricingRule
	total, page, pageSize, err := findPagedRows(applyPricingRuleQuery(db, scopeQuery), scopeQuery.Page, scopeQuery.PageSize, &rows)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return rows, total, page, pageSize, nil
}

func applyPricingRuleQuery(db *gorm.DB, query PricingRuleQuery) *gorm.DB {
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
	if query.RuleType != "" {
		db = db.Where("rule_type = ?", query.RuleType)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	return db
}
