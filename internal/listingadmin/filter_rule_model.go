package listingadmin

import "strings"

func (r listingFilterRule) toFilterRule() FilterRule {
	return FilterRule{
		ID:              r.ID,
		TenantID:        r.TenantID,
		Name:            r.Name,
		RuleCode:        r.RuleCode,
		Description:     r.Description,
		StoreID:         int64PtrIfPositive(r.StoreID),
		CategoryID:      int64PtrIfPositive(r.CategoryID),
		PriceType:       r.PriceType,
		PriceMin:        r.PriceMin,
		PriceMax:        r.PriceMax,
		StockMin:        r.StockMin,
		RatingMin:       r.RatingMin,
		ReviewCountMin:  r.ReviewCountMin,
		DeliveryTimeMax: intPtrIfPositive(r.DeliveryTimeMax),
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
	return listingFilterRule{
		ID:              rule.ID,
		TenantID:        rule.TenantID,
		Name:            strings.TrimSpace(rule.Name),
		RuleCode:        strings.TrimSpace(rule.RuleCode),
		Description:     strings.TrimSpace(rule.Description),
		StoreID:         int64Value(rule.StoreID),
		CategoryID:      int64Value(rule.CategoryID),
		PriceType:       strings.TrimSpace(rule.PriceType),
		PriceMin:        rule.PriceMin,
		PriceMax:        rule.PriceMax,
		StockMin:        rule.StockMin,
		RatingMin:       rule.RatingMin,
		ReviewCountMin:  rule.ReviewCountMin,
		DeliveryTimeMax: intValue(rule.DeliveryTimeMax),
		FulfillmentType: strings.TrimSpace(rule.FulfillmentType),
		Status:          rule.Status,
		Remark:          strings.TrimSpace(rule.Remark),
	}
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

func applyFilterRuleAuditFields(row *listingFilterRule, userID string, includeCreate bool) {
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return
	}
	row.OwnerUserID = trimmedUserID
	row.Updater = trimmedUserID
	row.UpdatedBy = trimmedUserID
	if includeCreate {
		row.Creator = trimmedUserID
		row.CreatedBy = trimmedUserID
	}
}
