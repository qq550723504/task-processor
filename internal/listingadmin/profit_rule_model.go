package listingadmin

import "strings"

func (r listingProfitRule) toProfitRule() ProfitRule {
	return ProfitRule{
		ID:                      r.ID,
		TenantID:                r.TenantID,
		Name:                    r.Name,
		RuleCode:                r.RuleCode,
		Description:             r.Description,
		StoreID:                 int64PtrIfPositive(r.StoreID),
		CategoryID:              int64PtrIfPositive(r.CategoryID),
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
	return listingProfitRule{
		ID:                      rule.ID,
		TenantID:                rule.TenantID,
		Name:                    strings.TrimSpace(rule.Name),
		RuleCode:                strings.TrimSpace(rule.RuleCode),
		Description:             strings.TrimSpace(rule.Description),
		StoreID:                 int64Value(rule.StoreID),
		CategoryID:              int64Value(rule.CategoryID),
		SalePriceMultiplier:     rule.SalePriceMultiplier,
		DiscountPriceMultiplier: rule.DiscountPriceMultiplier,
		Status:                  rule.Status,
		Remark:                  strings.TrimSpace(rule.Remark),
	}
}

func applyProfitRuleDefaults(row *listingProfitRule) {
	if row.SalePriceMultiplier <= 0 {
		row.SalePriceMultiplier = 1
	}
	if row.DiscountPriceMultiplier <= 0 {
		row.DiscountPriceMultiplier = 1
	}
}

func applyProfitRuleAuditFields(row *listingProfitRule, userID string, includeCreate bool) {
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
