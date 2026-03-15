// Package validation 提供TEMU平台的规则验证功能
package rules

import (
	"fmt"
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	domainvalidation "task-processor/internal/domain/validation"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/handlers/common"

	"github.com/sirupsen/logrus"
)

// RuleValidator 规则验证器（TEMU平台包装器）
type RuleValidator struct {
	logger             *logrus.Entry
	checker            *domainvalidation.RuleChecker
	fulfillmentChecker *common.FulfillmentChecker
}

// NewRuleValidator 创建新的规则验证器
func NewRuleValidator(logger *logrus.Entry) *RuleValidator {
	return &RuleValidator{
		logger:             logger.WithField("component", "RuleValidator"),
		checker:            domainvalidation.NewRuleChecker(),
		fulfillmentChecker: common.NewFulfillmentChecker(logger),
	}
}

// CheckSingleRule 检查单个规则（兼容旧接口）
func (v *RuleValidator) CheckSingleRule(product *model.Product, rule *api.FilterRuleRespDTO, ctx pipeline.TaskContext) bool {
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		result := v.CheckSingleRuleDetailedTemu(product, rule, temuCtx)
		return result.Passed
	}
	result := v.CheckSingleRuleDetailed(product, rule, ctx)
	return result.Passed
}

// CheckSingleRuleDetailed 详细检查单个规则（兼容旧接口）
func (v *RuleValidator) CheckSingleRuleDetailed(product *model.Product, rule *api.FilterRuleRespDTO, ctx pipeline.TaskContext) *common.FilterCheckResult {
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return v.CheckSingleRuleDetailedTemu(product, rule, temuCtx)
	}
	return v.checkBasicRules(product, rule)
}

// CheckSingleRuleDetailedTemu 详细检查单个规则（强类型上下文）
func (v *RuleValidator) CheckSingleRuleDetailedTemu(amazonProduct *model.Product, rule *api.FilterRuleRespDTO, temuCtx *temucontext.TemuTaskContext) *common.FilterCheckResult {
	if result := v.checkImageCountRuleDetailed(amazonProduct); !result.Passed {
		return result
	}
	if result := v.checkPriceRuleDetailedTemu(amazonProduct, rule, temuCtx); !result.Passed {
		return result
	}
	if result := v.checkRatingRuleDetailed(amazonProduct, rule); !result.Passed {
		return result
	}
	if result := v.checkReviewCountRuleDetailed(amazonProduct, rule); !result.Passed {
		return result
	}
	if result := v.checkStockRuleDetailed(amazonProduct, rule); !result.Passed {
		return result
	}
	if result := v.fulfillmentChecker.CheckFulfillmentTypeRuleDetailed(amazonProduct, rule); !result.Passed {
		return result
	}
	return &common.FilterCheckResult{Passed: true}
}

// checkBasicRules 基本规则检查（不依赖上下文）
func (v *RuleValidator) checkBasicRules(amazonProduct *model.Product, rule *api.FilterRuleRespDTO) *common.FilterCheckResult {
	if result := v.checkImageCountRuleDetailed(amazonProduct); !result.Passed {
		return result
	}
	if result := v.checkPriceRuleBasic(amazonProduct, rule); !result.Passed {
		return result
	}
	if result := v.checkRatingRuleDetailed(amazonProduct, rule); !result.Passed {
		return result
	}
	if result := v.checkReviewCountRuleDetailed(amazonProduct, rule); !result.Passed {
		return result
	}
	if result := v.checkStockRuleDetailed(amazonProduct, rule); !result.Passed {
		return result
	}
	if result := v.fulfillmentChecker.CheckFulfillmentTypeRuleDetailed(amazonProduct, rule); !result.Passed {
		return result
	}
	return &common.FilterCheckResult{Passed: true}
}

func (v *RuleValidator) checkPriceRuleDetailedTemu(amazonProduct *model.Product, rule *api.FilterRuleRespDTO, temuCtx *temucontext.TemuTaskContext) *common.FilterCheckResult {
	priceType := "final"
	if temuCtx.StoreInfo != nil && temuCtx.StoreInfo.PriceType != "" {
		priceType = temuCtx.StoreInfo.PriceType
	}
	price := product.GetProductPrice(amazonProduct, priceType)

	v.logger.WithFields(logrus.Fields{
		"asin":       amazonProduct.Asin,
		"price_type": priceType,
		"price":      price,
		"price_min":  rule.PriceMin,
		"price_max":  rule.PriceMax,
	}).Debug("检查价格规则")

	if err := v.checker.CheckPriceRange(rule.ToFilterRule(), price); err != nil {
		ruleValue := 0.0
		if rule.PriceMin != nil && price < *rule.PriceMin {
			ruleValue = *rule.PriceMin
		} else if rule.PriceMax != nil {
			ruleValue = *rule.PriceMax
		}
		return &common.FilterCheckResult{
			Passed:        false,
			FailureReason: err.Error(),
			ProductValue:  price,
			RuleValue:     ruleValue,
		}
	}
	return &common.FilterCheckResult{Passed: true}
}

func (v *RuleValidator) checkPriceRuleBasic(amazonProduct *model.Product, rule *api.FilterRuleRespDTO) *common.FilterCheckResult {
	price := product.GetProductPrice(amazonProduct, "final")

	v.logger.WithFields(logrus.Fields{
		"asin":       amazonProduct.Asin,
		"price_type": "final",
		"price":      price,
		"price_min":  rule.PriceMin,
		"price_max":  rule.PriceMax,
	}).Debug("检查价格规则（基本模式）")

	if err := v.checker.CheckPriceRange(rule.ToFilterRule(), price); err != nil {
		ruleValue := 0.0
		if rule.PriceMin != nil && price < *rule.PriceMin {
			ruleValue = *rule.PriceMin
		} else if rule.PriceMax != nil {
			ruleValue = *rule.PriceMax
		}
		return &common.FilterCheckResult{
			Passed:        false,
			FailureReason: err.Error(),
			ProductValue:  price,
			RuleValue:     ruleValue,
		}
	}
	return &common.FilterCheckResult{Passed: true}
}

func (v *RuleValidator) checkRatingRuleDetailed(amazonProduct *model.Product, rule *api.FilterRuleRespDTO) *common.FilterCheckResult {
	if rule.RatingMin == nil {
		return &common.FilterCheckResult{Passed: true}
	}
	v.logger.WithFields(logrus.Fields{
		"asin":       amazonProduct.Asin,
		"rating":     amazonProduct.Rating,
		"rating_min": *rule.RatingMin,
	}).Debug("检查评分规则")

	if err := v.checker.CheckRating(rule.ToFilterRule(), amazonProduct.Rating); err != nil {
		return &common.FilterCheckResult{
			Passed:        false,
			FailureReason: err.Error(),
			ProductValue:  amazonProduct.Rating,
			RuleValue:     *rule.RatingMin,
		}
	}
	return &common.FilterCheckResult{Passed: true}
}

func (v *RuleValidator) checkReviewCountRuleDetailed(amazonProduct *model.Product, rule *api.FilterRuleRespDTO) *common.FilterCheckResult {
	if rule.ReviewCountMin == nil {
		return &common.FilterCheckResult{Passed: true}
	}
	v.logger.WithFields(logrus.Fields{
		"asin":             amazonProduct.Asin,
		"review_count":     amazonProduct.ReviewsCount,
		"review_count_min": *rule.ReviewCountMin,
	}).Debug("检查评论数量规则")

	if err := v.checker.CheckReviewCount(rule.ToFilterRule(), amazonProduct.ReviewsCount); err != nil {
		return &common.FilterCheckResult{
			Passed:        false,
			FailureReason: err.Error(),
			ProductValue:  amazonProduct.ReviewsCount,
			RuleValue:     *rule.ReviewCountMin,
		}
	}
	return &common.FilterCheckResult{Passed: true}
}

func (v *RuleValidator) checkStockRuleDetailed(amazonProduct *model.Product, rule *api.FilterRuleRespDTO) *common.FilterCheckResult {
	if rule.StockMin == nil {
		return &common.FilterCheckResult{Passed: true}
	}
	stock := product.GetInventory(amazonProduct)

	v.logger.WithFields(logrus.Fields{
		"asin":      amazonProduct.Asin,
		"stock":     stock,
		"stock_min": *rule.StockMin,
	}).Debug("检查库存规则")

	if err := v.checker.CheckInventory(rule.ToFilterRule(), stock); err != nil {
		return &common.FilterCheckResult{
			Passed:        false,
			FailureReason: err.Error(),
			ProductValue:  stock,
			RuleValue:     *rule.StockMin,
		}
	}
	return &common.FilterCheckResult{Passed: true}
}

func (v *RuleValidator) checkImageCountRuleDetailed(amazonProduct *model.Product) *common.FilterCheckResult {
	const minImageCount = 3

	imageCount := len(amazonProduct.Images)
	v.logger.WithFields(logrus.Fields{
		"asin":         amazonProduct.Asin,
		"image_count":  imageCount,
		"min_required": minImageCount,
	}).Debug("检查图片数量规则")

	if imageCount < minImageCount {
		return &common.FilterCheckResult{
			Passed:        false,
			FailureReason: fmt.Sprintf("Amazon原始数据图片不足，当前%d张，至少需要%d张", imageCount, minImageCount),
			ProductValue:  float64(imageCount),
			RuleValue:     float64(minImageCount),
		}
	}
	return &common.FilterCheckResult{Passed: true}
}
