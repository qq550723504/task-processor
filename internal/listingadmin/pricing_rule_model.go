package listingadmin

import "strings"

func (r listingPricingRule) toPricingRule() PricingRule {
	return PricingRule{
		ID:              r.ID,
		TenantID:        r.TenantID,
		Name:            r.Name,
		RuleCode:        r.RuleCode,
		Description:     r.Description,
		Remark:          r.Remark,
		StoreID:         int64PtrIfPositive(r.StoreID),
		CategoryID:      int64PtrIfPositive(r.CategoryID),
		PriceMin:        r.PriceMin,
		PriceMax:        r.PriceMax,
		RuleType:        r.RuleType,
		RuleValue:       r.RuleValue,
		FixedValue:      floatPtrIfPositive(r.FixedValue),
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
	return listingPricingRule{
		ID:              rule.ID,
		TenantID:        rule.TenantID,
		Name:            strings.TrimSpace(rule.Name),
		RuleCode:        strings.TrimSpace(rule.RuleCode),
		Description:     strings.TrimSpace(rule.Description),
		Remark:          strings.TrimSpace(rule.Remark),
		StoreID:         int64Value(rule.StoreID),
		CategoryID:      int64Value(rule.CategoryID),
		PriceMin:        rule.PriceMin,
		PriceMax:        rule.PriceMax,
		RuleType:        strings.TrimSpace(rule.RuleType),
		RuleValue:       rule.RuleValue,
		FixedValue:      floatValue(rule.FixedValue),
		AcceptCondition: strings.TrimSpace(rule.AcceptCondition),
		RejectCondition: strings.TrimSpace(rule.RejectCondition),
		Status:          rule.Status,
	}
}

func applyPricingRuleDefaults(row *listingPricingRule) {
	if row.PriceMax <= 0 {
		row.PriceMax = 99999
	}
}

func applyPricingRuleAuditFields(row *listingPricingRule, userID string, includeCreate bool) {
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
