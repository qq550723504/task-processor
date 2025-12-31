// Package modules 提供SHEIN平台的自动核价计算功能
package modules

import (
	managementapi "task-processor/internal/pkg/management/api"
)

// AutoPricingCalculator 自动核价计算器，负责根据规则计算价格
type AutoPricingCalculator struct{}

// NewAutoPricingCalculator 创建新的自动核价计算器
func NewAutoPricingCalculator() *AutoPricingCalculator {
	return &AutoPricingCalculator{}
}

// GetAutoPrice 获取自动核价
func (c *AutoPricingCalculator) GetAutoPrice(originPrice float64, rules []managementapi.PricingRuleRespDTO) float64 {
	for _, rule := range rules {
		// 判断是否在规则范围内
		if rule.PriceMin != nil && rule.PriceMax != nil &&
			originPrice >= *rule.PriceMin && originPrice < *rule.PriceMax {
			return c.ApplyRule(originPrice, rule)
		}
	}
	return originPrice // 没有匹配规则则原价
}

// ApplyRule 应用自动核价规则
func (c *AutoPricingCalculator) ApplyRule(originPrice float64, rule managementapi.PricingRuleRespDTO) float64 {
	if rule.RuleValue == nil {
		return originPrice
	}

	switch rule.RuleType {
	case "fixed":
		// 固定加价
		return originPrice + *rule.RuleValue
	case "percent":
		// 加价百分比
		return originPrice * (1 + *rule.RuleValue)
	case "multiple":
		// 倍数
		return *rule.RuleValue * originPrice
	case "discount":
		// 折扣率
		return originPrice * (1 - *rule.RuleValue)
	case "fixed_price":
		// 固定价格
		return *rule.RuleValue
	}
	return originPrice
}
