// Package handlerbase 提供TEMU平台的配送方式检查功能
package handlerbase

import (
	"fmt"

	"task-processor/internal/model"
	api "task-processor/internal/ports/managementapi"
	productpkg "task-processor/internal/product"

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
func (c *FulfillmentChecker) CheckFulfillmentTypeRule(p *model.Product, rule *api.FilterRuleRespDTO) bool {
	return c.CheckFulfillmentTypeRuleDetailed(p, rule).Passed
}

// CheckFulfillmentTypeRuleDetailed 详细检查配送方式规则
func (c *FulfillmentChecker) CheckFulfillmentTypeRuleDetailed(p *model.Product, rule *api.FilterRuleRespDTO) *FilterCheckResult {
	if rule.FulfillmentType == "" || rule.FulfillmentType == "ALL" {
		return &FilterCheckResult{Passed: true}
	}

	isFBA := productpkg.IsFBAFulfillment(p.ShipsFrom)
	isAMZ := productpkg.IsAMZSeller(p.SellerName)

	c.logger.WithFields(logrus.Fields{
		"asin":          p.Asin,
		"ships_from":    p.ShipsFrom,
		"seller_name":   p.SellerName,
		"is_fba":        isFBA,
		"is_amz":        isAMZ,
		"required_type": rule.FulfillmentType,
	}).Debug("检查配送方式规则")

	checker := productpkg.NewRuleChecker()
	if err := checker.CheckFulfillmentType(rule.ToFilterRule(), p); err != nil {
		return &FilterCheckResult{
			Passed:        false,
			FailureReason: err.Error(),
			ProductValue:  buildFulfillmentProductValue(isFBA, isAMZ, p.SellerName),
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
