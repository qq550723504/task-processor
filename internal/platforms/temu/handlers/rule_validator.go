// Package handlers 提供TEMU平台的规则验证功能
package handlers

import (
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	domainvalidation "task-processor/internal/domain/validation"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/management/api"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// RuleValidator 规则验证器（TEMU平台包装器）
type RuleValidator struct {
	logger             *logrus.Entry
	checker            *domainvalidation.RuleChecker
	fulfillmentChecker *FulfillmentChecker
}

// NewRuleValidator 创建新的规则验证器
func NewRuleValidator(logger *logrus.Entry) *RuleValidator {
	return &RuleValidator{
		logger:             logger.WithField("component", "RuleValidator"),
		checker:            domainvalidation.NewRuleChecker(),
		fulfillmentChecker: NewFulfillmentChecker(logger),
	}
}

// CheckSingleRule 检查单个规则（兼容旧接口）
func (v *RuleValidator) CheckSingleRule(product *model.Product, rule *api.FilterRuleRespDTO, ctx pipeline.TaskContext) bool {
	// 尝试转换为强类型上下文
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		result := v.CheckSingleRuleDetailedTemu(product, rule, temuCtx)
		return result.Passed
	}

	// 兼容旧的接口
	result := v.CheckSingleRuleDetailed(product, rule, ctx)
	return result.Passed
}

// CheckSingleRuleDetailed 详细检查单个规则（兼容旧接口）
func (v *RuleValidator) CheckSingleRuleDetailed(product *model.Product, rule *api.FilterRuleRespDTO, ctx pipeline.TaskContext) *FilterCheckResult {
	// 尝试转换为强类型上下文
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return v.CheckSingleRuleDetailedTemu(product, rule, temuCtx)
	}

	// 兼容旧的接口，使用基本检查
	return v.checkBasicRules(product, rule)
}

// CheckSingleRuleDetailedTemu 详细检查单个规则（强类型上下文）
func (v *RuleValidator) CheckSingleRuleDetailedTemu(amazonProduct *model.Product, rule *api.FilterRuleRespDTO, temuCtx *temucontext.TemuTaskContext) *FilterCheckResult {
	// 价格检查
	if result := v.checkPriceRuleDetailedTemu(amazonProduct, rule, temuCtx); !result.Passed {
		return result
	}

	// 评分检查
	if result := v.checkRatingRuleDetailed(amazonProduct, rule); !result.Passed {
		return result
	}

	// 评论数量检查
	if result := v.checkReviewCountRuleDetailed(amazonProduct, rule); !result.Passed {
		return result
	}

	// 库存检查
	if result := v.checkStockRuleDetailed(amazonProduct, rule); !result.Passed {
		return result
	}

	// 配送方式检查
	if result := v.fulfillmentChecker.CheckFulfillmentTypeRuleDetailed(amazonProduct, rule); !result.Passed {
		return result
	}

	return &FilterCheckResult{Passed: true}
}

// checkBasicRules 基本规则检查（不依赖上下文）
func (v *RuleValidator) checkBasicRules(amazonProduct *model.Product, rule *api.FilterRuleRespDTO) *FilterCheckResult {
	// 价格检查（使用默认价格类型）
	if result := v.checkPriceRuleBasic(amazonProduct, rule); !result.Passed {
		return result
	}

	// 评分检查
	if result := v.checkRatingRuleDetailed(amazonProduct, rule); !result.Passed {
		return result
	}

	// 评论数量检查
	if result := v.checkReviewCountRuleDetailed(amazonProduct, rule); !result.Passed {
		return result
	}

	// 库存检查
	if result := v.checkStockRuleDetailed(amazonProduct, rule); !result.Passed {
		return result
	}

	// 配送方式检查
	if result := v.fulfillmentChecker.CheckFulfillmentTypeRuleDetailed(amazonProduct, rule); !result.Passed {
		return result
	}

	return &FilterCheckResult{Passed: true}
}

// checkPriceRuleDetailedTemu 详细检查价格规则（强类型上下文）
func (v *RuleValidator) checkPriceRuleDetailedTemu(amazonProduct *model.Product, rule *api.FilterRuleRespDTO, temuCtx *temucontext.TemuTaskContext) *FilterCheckResult {
	// 获取店铺配置的价格类型
	priceType := "final" // 默认价格类型
	if temuCtx.StoreInfo != nil && temuCtx.StoreInfo.PriceType != "" {
		priceType = temuCtx.StoreInfo.PriceType
	}

	// 根据价格类型获取价格 - 使用公共函数
	price := product.GetProductPrice(amazonProduct, priceType)

	v.logger.WithFields(logrus.Fields{
		"asin":       amazonProduct.Asin,
		"price_type": priceType,
		"price":      price,
		"price_min":  rule.PriceMin,
		"price_max":  rule.PriceMax,
	}).Debug("检查价格规则")

	// 使用公共验证器检查价格范围
	if err := v.checker.CheckPriceRange(rule, price); err != nil {
		ruleValue := 0.0
		if rule.PriceMin != nil && price < *rule.PriceMin {
			ruleValue = *rule.PriceMin
		} else if rule.PriceMax != nil {
			ruleValue = *rule.PriceMax
		}
		return &FilterCheckResult{
			Passed:        false,
			FailureReason: err.Error(),
			ProductValue:  price,
			RuleValue:     ruleValue,
		}
	}

	return &FilterCheckResult{Passed: true}
}

// checkPriceRuleBasic 基本价格规则检查（不依赖上下文）
func (v *RuleValidator) checkPriceRuleBasic(amazonProduct *model.Product, rule *api.FilterRuleRespDTO) *FilterCheckResult {
	// 使用默认价格类型 - 使用公共函数
	price := product.GetProductPrice(amazonProduct, "final")

	v.logger.WithFields(logrus.Fields{
		"asin":       amazonProduct.Asin,
		"price_type": "final",
		"price":      price,
		"price_min":  rule.PriceMin,
		"price_max":  rule.PriceMax,
	}).Debug("检查价格规则（基本模式）")

	// 使用公共验证器检查价格范围
	if err := v.checker.CheckPriceRange(rule, price); err != nil {
		ruleValue := 0.0
		if rule.PriceMin != nil && price < *rule.PriceMin {
			ruleValue = *rule.PriceMin
		} else if rule.PriceMax != nil {
			ruleValue = *rule.PriceMax
		}
		return &FilterCheckResult{
			Passed:        false,
			FailureReason: err.Error(),
			ProductValue:  price,
			RuleValue:     ruleValue,
		}
	}

	return &FilterCheckResult{Passed: true}
}

// checkRatingRuleDetailed 详细检查评分规则
func (v *RuleValidator) checkRatingRuleDetailed(amazonProduct *model.Product, rule *api.FilterRuleRespDTO) *FilterCheckResult {
	if rule.RatingMin == nil {
		return &FilterCheckResult{Passed: true}
	}

	v.logger.WithFields(logrus.Fields{
		"asin":       amazonProduct.Asin,
		"rating":     amazonProduct.Rating,
		"rating_min": *rule.RatingMin,
	}).Debug("检查评分规则")

	if err := v.checker.CheckRating(rule, amazonProduct.Rating); err != nil {
		return &FilterCheckResult{
			Passed:        false,
			FailureReason: err.Error(),
			ProductValue:  amazonProduct.Rating,
			RuleValue:     *rule.RatingMin,
		}
	}

	return &FilterCheckResult{Passed: true}
}

// checkReviewCountRuleDetailed 详细检查评论数量规则
func (v *RuleValidator) checkReviewCountRuleDetailed(amazonProduct *model.Product, rule *api.FilterRuleRespDTO) *FilterCheckResult {
	if rule.ReviewCountMin == nil {
		return &FilterCheckResult{Passed: true}
	}

	v.logger.WithFields(logrus.Fields{
		"asin":             amazonProduct.Asin,
		"review_count":     amazonProduct.ReviewsCount,
		"review_count_min": *rule.ReviewCountMin,
	}).Debug("检查评论数量规则")

	if err := v.checker.CheckReviewCount(rule, amazonProduct.ReviewsCount); err != nil {
		return &FilterCheckResult{
			Passed:        false,
			FailureReason: err.Error(),
			ProductValue:  amazonProduct.ReviewsCount,
			RuleValue:     *rule.ReviewCountMin,
		}
	}

	return &FilterCheckResult{Passed: true}
}

// checkStockRuleDetailed 详细检查库存规则
func (v *RuleValidator) checkStockRuleDetailed(amazonProduct *model.Product, rule *api.FilterRuleRespDTO) *FilterCheckResult {
	if rule.StockMin == nil {
		return &FilterCheckResult{Passed: true}
	}

	// 获取库存数量（使用公共函数）
	stock := product.GetInventory(amazonProduct)

	v.logger.WithFields(logrus.Fields{
		"asin":      amazonProduct.Asin,
		"stock":     stock,
		"stock_min": *rule.StockMin,
	}).Debug("检查库存规则")

	if err := v.checker.CheckInventory(rule, stock); err != nil {
		return &FilterCheckResult{
			Passed:        false,
			FailureReason: err.Error(),
			ProductValue:  stock,
			RuleValue:     *rule.StockMin,
		}
	}

	return &FilterCheckResult{Passed: true}
}
