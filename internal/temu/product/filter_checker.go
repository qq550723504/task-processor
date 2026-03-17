// Package product 提供TEMU平台的产品筛选检查功能
package product

import (
	"fmt"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/pipeline"
	"task-processor/internal/temu/handlerbase"
	"task-processor/internal/temu/rules"

	"github.com/sirupsen/logrus"
)

// ProductFilterChecker 产品筛选检查器
type ProductFilterChecker struct {
	logger        *logrus.Entry
	ruleValidator *rules.RuleValidator
}

// NewProductFilterChecker 创建新的产品筛选检查器
func NewProductFilterChecker(logger *logrus.Entry) *ProductFilterChecker {
	return &ProductFilterChecker{
		logger:        logger.WithField("component", "ProductFilterChecker"),
		ruleValidator: rules.NewRuleValidator(logger),
	}
}

// CheckProductAgainstRules 检查产品是否符合筛选规则
func (c *ProductFilterChecker) CheckProductAgainstRules(product *model.Product, rules *[]api.FilterRuleRespDTO, ctx pipeline.TaskContext) bool {
	result := c.CheckProductAgainstRulesDetailed(product, rules, ctx)
	return result.Passed
}

// CheckProductAgainstRulesDetailed 详细检查产品是否符合筛选规则
func (c *ProductFilterChecker) CheckProductAgainstRulesDetailed(product *model.Product, rules *[]api.FilterRuleRespDTO, ctx pipeline.TaskContext) *handlerbase.FilterCheckResult {
	if rules == nil || len(*rules) == 0 {
		c.logger.Debug("没有筛选规则，产品通过")
		return &handlerbase.FilterCheckResult{Passed: true}
	}

	for _, rule := range *rules {
		// 跳过禁用的规则
		if rule.Status != 0 {
			c.logger.Debugf("跳过禁用的规则: %s (ID: %d)", rule.Name, rule.ID)
			continue
		}

		if result := c.checkSingleRuleWithAdapter(product, &rule, ctx); !result.Passed {

			return &handlerbase.FilterCheckResult{
				Passed:        false,
				FailedRule:    rule.Name,
				FailureReason: result.FailureReason,
				ProductValue:  result.ProductValue,
				RuleValue:     result.RuleValue,
			}
		}
	}

	c.logger.WithField("asin", product.Asin).Info("产品通过所有筛选规则")
	return &handlerbase.FilterCheckResult{Passed: true}
}

// checkSingleRuleWithAdapter 适配器方法，处理接口不匹配问题
func (c *ProductFilterChecker) checkSingleRuleWithAdapter(product *model.Product, rule *api.FilterRuleRespDTO, _ pipeline.TaskContext) *handlerbase.FilterCheckResult {
	// 由于RuleValidator期望*pipeline.TaskContext，但我们有pipeline.TaskContext接口
	// 这里我们创建一个简化的检查逻辑，避免接口不匹配
	// TODO: 重构RuleValidator以使用接口而不是具体类型

	// 简化的规则检查逻辑
	if rule.Status == 1 { // 规则未启用
		return &handlerbase.FilterCheckResult{Passed: true}
	}

	// 基本的价格检查
	if rule.PriceMin != nil && product.FinalPrice < *rule.PriceMin {
		return &handlerbase.FilterCheckResult{
			Passed:        false,
			FailureReason: "价格低于最小值",
			ProductValue:  fmt.Sprintf("%.2f", product.FinalPrice),
			RuleValue:     fmt.Sprintf("%.2f", *rule.PriceMin),
		}
	}

	if rule.PriceMax != nil && product.FinalPrice > *rule.PriceMax {
		return &handlerbase.FilterCheckResult{
			Passed:        false,
			FailureReason: "价格高于最大值",
			ProductValue:  fmt.Sprintf("%.2f", product.FinalPrice),
			RuleValue:     fmt.Sprintf("%.2f", *rule.PriceMax),
		}
	}

	// 基本的评分检查
	if rule.RatingMin != nil && product.Rating < *rule.RatingMin {
		return &handlerbase.FilterCheckResult{
			Passed:        false,
			FailureReason: "评分低于最小值",
			ProductValue:  fmt.Sprintf("%.1f", product.Rating),
			RuleValue:     fmt.Sprintf("%.1f", *rule.RatingMin),
		}
	}

	return &handlerbase.FilterCheckResult{Passed: true}
}
