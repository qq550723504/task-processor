package publishing

import "task-processor/internal/listingadmin"

const defaultProfitRate = 1.5

// CalculateMinAcceptablePrice applies a TEMU minimum-price rule without
// depending on the historical TEMU runtime package.
func CalculateMinAcceptablePrice(originCostPrice float64, rule *listingadmin.PricingRuleRespDTO) float64 {
	if originCostPrice <= 0 || rule == nil || rule.RuleValue == nil {
		return originCostPrice * defaultProfitRate
	}

	switch rule.RuleType {
	case "multiple_fixed":
		return originCostPrice*(*rule.RuleValue) + safeFloat64(rule.FixedValue)
	case "multiple":
		return originCostPrice * (*rule.RuleValue)
	case "fixed":
		return originCostPrice + *rule.RuleValue
	case "fixed_price":
		return *rule.RuleValue
	default:
		return originCostPrice * (*rule.RuleValue)
	}
}

// SelectApplicablePricingRule returns the first rule whose price range
// strictly contains costPrice, matching the historical TEMU behavior.
func SelectApplicablePricingRule(costPrice float64, rules []listingadmin.PricingRuleRespDTO) *listingadmin.PricingRuleRespDTO {
	for _, rule := range rules {
		minPrice := safeFloat64(rule.PriceMin)
		maxPrice := safeFloat64(rule.PriceMax)
		if costPrice > minPrice && costPrice < maxPrice {
			selected := rule
			return &selected
		}
	}

	return nil
}

func safeFloat64(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}
