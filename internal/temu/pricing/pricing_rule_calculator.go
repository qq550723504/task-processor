// package pricing 提供TEMU平台的核价规则适配。
package pricing

import (
	api "task-processor/internal/listingadmin"
	temupublishing "task-processor/internal/marketplace/temu/publishing"

	"github.com/sirupsen/logrus"
)

// PricingRuleCalculator keeps the historical pricing interface while the
// formula and rule-selection policies live in the marketplace package.
type PricingRuleCalculator struct {
	logger *logrus.Entry
}

// NewPricingRuleCalculator creates a pricing rule calculator.
func NewPricingRuleCalculator(logger *logrus.Entry) PriceCalculator {
	return &PricingRuleCalculator{logger: logger}
}

// CalculateMinAcceptablePrice delegates the pure pricing formula policy.
func (c *PricingRuleCalculator) CalculateMinAcceptablePrice(
	originCostPrice float64,
	pricingRule *api.PricingRuleRespDTO,
) float64 {
	if originCostPrice <= 0 {
		c.logger.Warn("原始成本价无效，使用默认利润率")
	} else if pricingRule == nil || pricingRule.RuleValue == nil {
		c.logger.Warnf("核价规则为空，使用默认利润率")
	} else if pricingRule.RuleType != "multiple_fixed" &&
		pricingRule.RuleType != "multiple" &&
		pricingRule.RuleType != "fixed" &&
		pricingRule.RuleType != "fixed_price" {
		c.logger.Warnf("未知的规则类型: %s，使用默认倍率计算", pricingRule.RuleType)
	}

	result := temupublishing.CalculateMinAcceptablePrice(originCostPrice, pricingRule)
	c.logCalculation(originCostPrice, pricingRule, result)
	return result
}

// GetDefaultPricingRules delegates the pure price-range selection policy.
func (c *PricingRuleCalculator) GetDefaultPricingRules(costPrice float64, rules *[]api.PricingRuleRespDTO) *api.PricingRuleRespDTO {
	if rules == nil || len(*rules) == 0 {
		c.logger.Debug("没有可用的核价规则")
		return nil
	}

	selected := temupublishing.SelectApplicablePricingRule(costPrice, *rules)
	if selected == nil {
		c.logger.Debugf("没有找到适用于价格 %.2f 的规则", costPrice)
		return nil
	}

	c.logger.Debugf("找到适用规则: %s (价格范围: %.2f - %.2f)",
		selected.Name, float64OrZero(selected.PriceMin), float64OrZero(selected.PriceMax))
	return selected
}

func float64OrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func (c *PricingRuleCalculator) logCalculation(originCostPrice float64, rule *api.PricingRuleRespDTO, result float64) {
	if originCostPrice <= 0 || rule == nil || rule.RuleValue == nil {
		c.logger.Infof("使用默认利润率 %.2f: %.2f * %.2f = %.2f", 1.5, originCostPrice, 1.5, result)
		return
	}

	switch rule.RuleType {
	case "multiple_fixed":
		fixedValue := float64OrZero(rule.FixedValue)
		c.logger.Infof("使用核价规则 %s (倍率加固定值): %.2f * %.2f + %.2f = %.2f",
			rule.Name, originCostPrice, *rule.RuleValue, fixedValue, result)
	case "multiple":
		c.logger.Infof("使用核价规则 %s (倍率): %.2f * %.2f = %.2f",
			rule.Name, originCostPrice, *rule.RuleValue, result)
	case "fixed":
		c.logger.Infof("使用核价规则 %s (固定加价): %.2f + %.2f = %.2f",
			rule.Name, originCostPrice, *rule.RuleValue, result)
	case "fixed_price":
		c.logger.Infof("使用核价规则 %s (固定价格): %.2f", rule.Name, result)
	default:
		c.logger.Infof("使用核价规则 %s (倍率): %.2f * %.2f = %.2f",
			rule.Name, originCostPrice, *rule.RuleValue, result)
	}
}
