package listingadmin

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

func findFilterRuleRows(ctx context.Context, db *gorm.DB, query FilterRuleQuery) ([]listingFilterRule, int64, int, int, error) {
	scopeQuery := query
	if strings.TrimSpace(scopeQuery.OwnerUserID) == "" {
		scopeQuery.OwnerUserID = requestUserIDFromContext(ctx)
	}
	var rows []listingFilterRule
	total, page, pageSize, err := findPagedRows(applyFilterRuleQuery(db, scopeQuery), scopeQuery.Page, scopeQuery.PageSize, &rows)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return rows, total, page, pageSize, nil
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
