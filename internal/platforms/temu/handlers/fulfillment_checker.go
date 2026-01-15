// Package handlers 提供TEMU平台的配送方式检查功能
package handlers

import (
	"fmt"
	"task-processor/internal/domain/model"
	domainvalidation "task-processor/internal/domain/validation"
	"task-processor/internal/pkg/management/api"

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
	// 如果规则未设置配送方式或设置为ALL，则不进行筛选
	if rule.FulfillmentType == "" || rule.FulfillmentType == "ALL" {
		return &FilterCheckResult{Passed: true}
	}

	// 判断产品的配送方式（使用公共辅助函数）
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

	// 根据规则要求进行校验
	switch rule.FulfillmentType {
	case "FBA":
		if !isFBA {
			return &FilterCheckResult{
				Passed:        false,
				FailureReason: fmt.Sprintf("配送方式不符合：规则要求FBA配送，但产品为FBM配送 (ships_from: %s)", product.ShipsFrom),
				ProductValue:  "FBM",
				RuleValue:     "FBA",
			}
		}
	case "FBM":
		if isFBA {
			return &FilterCheckResult{
				Passed:        false,
				FailureReason: fmt.Sprintf("配送方式不符合：规则要求FBM配送，但产品为FBA配送 (ships_from: %s)", product.ShipsFrom),
				ProductValue:  "FBA",
				RuleValue:     "FBM",
			}
		}
	case "AMZ":
		if !isAMZ {
			return &FilterCheckResult{
				Passed:        false,
				FailureReason: fmt.Sprintf("配送方式不符合：规则要求亚马逊自营，但卖家为 %s", product.SellerName),
				ProductValue:  product.SellerName,
				RuleValue:     "Amazon",
			}
		}
	default:
		c.logger.Warnf("未知的配送方式类型: %s", rule.FulfillmentType)
		return &FilterCheckResult{
			Passed:        false,
			FailureReason: fmt.Sprintf("未知的配送方式类型: %s", rule.FulfillmentType),
			ProductValue:  rule.FulfillmentType,
			RuleValue:     "FBA/FBM/AMZ",
		}
	}

	return &FilterCheckResult{Passed: true}
}
