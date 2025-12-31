// Package temu 提供TEMU平台核价规则计算功能
package temu

import (
	"task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// PricingRuleCalculator 核价规则计算器
type PricingRuleCalculator struct {
	logger *logrus.Entry
}

// NewPricingRuleCalculator 创建核价规则计算器
func NewPricingRuleCalculator(logger *logrus.Entry) *PricingRuleCalculator {
	return &PricingRuleCalculator{
		logger: logger,
	}
}

// CalculateMinAcceptablePrice 根据核价规则计算最低可接受价格
func (c *PricingRuleCalculator) CalculateMinAcceptablePrice(
	originCostPrice float64,
	pricingRule *api.PricingRuleRespDTO,
) float64 {
	// 默认利润率
	defaultProfitRate := 1.5

	if pricingRule == nil || pricingRule.RuleValue == nil {
		c.logger.Warnf("核价规则为空，使用默认利润率%.2f", defaultProfitRate)
		return originCostPrice * defaultProfitRate
	}

	var minAcceptablePrice float64

	// 根据规则类型计算最低可接受价格
	switch pricingRule.RuleType {
	case "multiple_fixed": // 倍率加固定值
		fixedValue := c.getFixedValueOrDefault(pricingRule.FixedValue, 0)
		minAcceptablePrice = originCostPrice*(*pricingRule.RuleValue) + fixedValue
		c.logger.Infof("使用核价规则 %s (倍率加固定值): 倍率=%.2f, 固定值=%.2f",
			pricingRule.Name, *pricingRule.RuleValue, fixedValue)

	case "multiple": // 倍率
		minAcceptablePrice = originCostPrice * (*pricingRule.RuleValue)
		c.logger.Infof("使用核价规则 %s (倍率): 倍率=%.2f",
			pricingRule.Name, *pricingRule.RuleValue)

	case "fixed": // 固定加价
		minAcceptablePrice = originCostPrice + *pricingRule.RuleValue
		c.logger.Infof("使用核价规则 %s (固定加价): 固定值=%.2f",
			pricingRule.Name, *pricingRule.RuleValue)

	case "fixed_price": // 固定值
		minAcceptablePrice = *pricingRule.RuleValue
		c.logger.Infof("使用核价规则 %s (固定价格): 价格=%.2f",
			pricingRule.Name, *pricingRule.RuleValue)

	default:
		// 默认使用倍率计算
		minAcceptablePrice = originCostPrice * (*pricingRule.RuleValue)
		c.logger.Infof("使用核价规则 %s 的默认计算方式(倍率): %.2f",
			pricingRule.Name, *pricingRule.RuleValue)
	}

	return minAcceptablePrice
}

// getFixedValueOrDefault 获取固定值或默认值
func (c *PricingRuleCalculator) getFixedValueOrDefault(fixedValue *float64, defaultValue float64) float64 {
	if fixedValue != nil {
		return *fixedValue
	}
	return defaultValue
}
