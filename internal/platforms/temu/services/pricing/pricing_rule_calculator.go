// Package pricing 提供TEMU平台核价规则计算功能
package pricing

import (
	"task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// PricingRuleCalculator 核价规则计算器
type PricingRuleCalculator struct {
	logger *logrus.Entry
}

// NewPricingRuleCalculator 创建核价规则计算器
func NewPricingRuleCalculator(logger *logrus.Entry) PriceCalculator {
	return &PricingRuleCalculator{
		logger: logger,
	}
}

// CalculateMinAcceptablePrice 根据核价规则计算最低可接受价格
func (c *PricingRuleCalculator) CalculateMinAcceptablePrice(
	originCostPrice float64,
	pricingRule *api.PricingRuleRespDTO,
) float64 {
	// 参数验证
	if originCostPrice <= 0 {
		c.logger.Warn("原始成本价无效，使用默认利润率")
		return c.calculateWithDefaultRate(originCostPrice)
	}

	if pricingRule == nil || pricingRule.RuleValue == nil {
		c.logger.Warnf("核价规则为空，使用默认利润率")
		return c.calculateWithDefaultRate(originCostPrice)
	}

	// 根据规则类型计算最低可接受价格
	switch pricingRule.RuleType {
	case "multiple_fixed":
		return c.calculateMultipleFixed(originCostPrice, pricingRule)
	case "multiple":
		return c.calculateMultiple(originCostPrice, pricingRule)
	case "fixed":
		return c.calculateFixed(originCostPrice, pricingRule)
	case "fixed_price":
		return c.calculateFixedPrice(pricingRule)
	default:
		c.logger.Warnf("未知的规则类型: %s，使用默认倍率计算", pricingRule.RuleType)
		return c.calculateMultiple(originCostPrice, pricingRule)
	}
}

// GetDefaultPricingRules 获取默认核价规则
func (c *PricingRuleCalculator) GetDefaultPricingRules(costPrice float64, rules *[]api.PricingRuleRespDTO) *api.PricingRuleRespDTO {
	if rules == nil || len(*rules) == 0 {
		c.logger.Debug("没有可用的核价规则")
		return nil
	}

	// 查找适用的规则
	for _, rule := range *rules {
		if c.isRuleApplicable(costPrice, &rule) {
			c.logger.Debugf("找到适用规则: %s (价格范围: %.2f - %.2f)",
				rule.Name, c.safeFloat64(rule.PriceMin), c.safeFloat64(rule.PriceMax))
			return &rule
		}
	}

	c.logger.Debugf("没有找到适用于价格 %.2f 的规则", costPrice)
	return nil
}

// calculateWithDefaultRate 使用默认利润率计算
func (c *PricingRuleCalculator) calculateWithDefaultRate(originCostPrice float64) float64 {
	const defaultProfitRate = 1.5
	result := originCostPrice * defaultProfitRate
	c.logger.Infof("使用默认利润率 %.2f: %.2f * %.2f = %.2f",
		defaultProfitRate, originCostPrice, defaultProfitRate, result)
	return result
}

// calculateMultipleFixed 倍率加固定值计算
func (c *PricingRuleCalculator) calculateMultipleFixed(originCostPrice float64, rule *api.PricingRuleRespDTO) float64 {
	fixedValue := c.safeFloat64(rule.FixedValue)
	result := originCostPrice*(*rule.RuleValue) + fixedValue
	c.logger.Infof("使用核价规则 %s (倍率加固定值): %.2f * %.2f + %.2f = %.2f",
		rule.Name, originCostPrice, *rule.RuleValue, fixedValue, result)
	return result
}

// calculateMultiple 倍率计算
func (c *PricingRuleCalculator) calculateMultiple(originCostPrice float64, rule *api.PricingRuleRespDTO) float64 {
	result := originCostPrice * (*rule.RuleValue)
	c.logger.Infof("使用核价规则 %s (倍率): %.2f * %.2f = %.2f",
		rule.Name, originCostPrice, *rule.RuleValue, result)
	return result
}

// calculateFixed 固定加价计算
func (c *PricingRuleCalculator) calculateFixed(originCostPrice float64, rule *api.PricingRuleRespDTO) float64 {
	result := originCostPrice + *rule.RuleValue
	c.logger.Infof("使用核价规则 %s (固定加价): %.2f + %.2f = %.2f",
		rule.Name, originCostPrice, *rule.RuleValue, result)
	return result
}

// calculateFixedPrice 固定价格计算
func (c *PricingRuleCalculator) calculateFixedPrice(rule *api.PricingRuleRespDTO) float64 {
	result := *rule.RuleValue
	c.logger.Infof("使用核价规则 %s (固定价格): %.2f",
		rule.Name, result)
	return result
}

// isRuleApplicable 检查规则是否适用于给定价格
func (c *PricingRuleCalculator) isRuleApplicable(costPrice float64, rule *api.PricingRuleRespDTO) bool {
	minPrice := c.safeFloat64(rule.PriceMin)
	maxPrice := c.safeFloat64(rule.PriceMax)
	return costPrice > minPrice && costPrice < maxPrice
}

// safeFloat64 安全获取float64指针的值
func (c *PricingRuleCalculator) safeFloat64(ptr *float64) float64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}
