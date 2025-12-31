// Package service 提供价格计算器的实现。
package service

import (
	"context"
	"fmt"
	"task-processor/internal/pkg/pricing/model"

	"github.com/sirupsen/logrus"
)

// DefaultPricingCalculator 默认价格计算器实现
type DefaultPricingCalculator struct {
	logger *logrus.Entry
}

// NewDefaultPricingCalculator 创建默认价格计算器
func NewDefaultPricingCalculator() *DefaultPricingCalculator {
	return &DefaultPricingCalculator{
		logger: logrus.WithField("component", "DefaultPricingCalculator"),
	}
}

// CalculatePrice 根据规则计算价格
func (c *DefaultPricingCalculator) CalculatePrice(ctx context.Context, originPrice float64, rules []model.PricingRule) (float64, *model.PricingRule, error) {
	if originPrice <= 0 {
		return 0, nil, model.ErrZeroCostPrice
	}

	if len(rules) == 0 {
		c.logger.Warn("未找到核价规则，使用原价")
		return originPrice, nil, model.ErrNoPricingRule
	}

	// 查找适用的规则
	applicableRule := c.FindApplicableRule(originPrice, rules)
	if applicableRule == nil {
		c.logger.Warnf("未找到适用于价格 %.2f 的规则，使用原价", originPrice)
		return originPrice, nil, model.ErrNoPricingRule
	}

	// 应用规则计算价格
	calculatedPrice, err := applicableRule.ApplyRule(originPrice)
	if err != nil {
		c.logger.Errorf("应用规则 %s 计算价格失败: %v", applicableRule.Name, err)
		return originPrice, applicableRule, fmt.Errorf("应用规则失败: %w", err)
	}

	if calculatedPrice < 0 {
		c.logger.Errorf("计算得出的价格为负数: %.2f", calculatedPrice)
		return originPrice, applicableRule, model.ErrNegativePrice
	}

	c.logger.Infof("使用规则 %s 计算价格: %.2f -> %.2f",
		applicableRule.Name, originPrice, calculatedPrice)

	return calculatedPrice, applicableRule, nil
}

// FindApplicableRule 查找适用的规则
func (c *DefaultPricingCalculator) FindApplicableRule(price float64, rules []model.PricingRule) *model.PricingRule {
	for i := range rules {
		rule := &rules[i]
		if rule.IsApplicable(price) {
			c.logger.Debugf("找到适用规则: %s (类型: %s)", rule.Name, rule.RuleType)
			return rule
		}
	}

	c.logger.Debugf("未找到适用于价格 %.2f 的规则", price)
	return nil
}

// ValidateRules 验证规则配置
func (c *DefaultPricingCalculator) ValidateRules(rules []model.PricingRule) error {
	for i, rule := range rules {
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("规则 %d 验证失败: %w", i, err)
		}
	}
	return nil
}
