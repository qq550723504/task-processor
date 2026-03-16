// Package handlerbase 提供TEMU平台的配送方式检查功能
package handlerbase

import (
	"fmt"
	"task-processor/internal/domain/model"
	domainvalidation "task-processor/internal/domain/validation"
	"task-processor/internal/infra/clients/management/api"

	"github.com/sirupsen/logrus"
)

// FulfillmentChecker 配送方式检查器
type FulfillmentChecker struct {
	logger *logrus.Entry
}

// NewFulfillmentChecker 创建新的配送方式检查器
func NewFulfillmentChecker(logger *logrus.Entry) *FulfillmentChecker {
	return &FulfillmentChecker{
		logger: logger.WithField("component", "FulfillmentChecker"),
	}
}

// CheckFulfillmentTypeRule 检查配送方式规则
func (c *FulfillmentChecker) CheckFulfillmentTypeRule(product *model.Product, rule *api.FilterRuleRespDTO) bool {
	result := c.CheckFulfillmentTypeRuleDetailed(product, rule)
	return result.Passed
}

// CheckFulfillmentTypeRuleDetailed 详细检查配送方式规则
func (c *FulfillmentChecker) CheckFulfillmentTypeRuleDetailed(product *model.Product, rule *api.FilterRuleRespDTO) *FilterCheckResult {
	if rule.FulfillmentType == "" || rule.FulfillmentType == "ALL" {
		return &FilterCheckResult{Passed: true}
	}

	isFBA := domainvalidation.IsFBAFulfillment(product.ShipsFrom)
	isAMZ := domainvalidation.IsAMZSeller(product.SellerName)

	c.logger.WithFields(logrus.Fields{
		"asin":          product.Asin,
		"ships_from":    product.ShipsFrom,
		"seller_name":   product.SellerName,
		"is_fba":        isFBA,
		"is_amz":        isAMZ,
		"required_type": rule.FulfillmentType,
	}).Debug("检查配送方式规则")

	// 转换后调用 domain checker
	checker := domainvalidation.NewRuleChecker()
	if err := checker.CheckFulfillmentType(rule.ToFilterRule(), product); err != nil {
		productValue := "FBM"
		if isFBA {
			productValue = "FBA"
		}
		if isAMZ {
			productValue = product.SellerName
		}
		return &FilterCheckResult{
			Passed:        false,
			FailureReason: err.Error(),
			ProductValue:  productValue,
			RuleValue:     rule.FulfillmentType,
		}
	}

	return &FilterCheckResult{Passed: true}
}

// buildFulfillmentProductValue 构建配送方式产品值描述
func buildFulfillmentProductValue(isFBA, isAMZ bool, sellerName string) string {
	if isAMZ {
		return fmt.Sprintf("AMZ(%s)", sellerName)
	}
	if isFBA {
		return "FBA"
	}
	return "FBM"
}
