// Package handlers 提供TEMU平台的配送方式检查功能
package handlers

import (
	"fmt"
	"regexp"
	"task-processor/internal/domain/model"
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

	// 判断产品的配送方式
	isFBA := c.isFBAFulfillment(product.ShipsFrom)
	isAMZ := c.isAMZSeller(product.SellerName)

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

// isFBAFulfillment 判断是否为FBA配送
func (c *FulfillmentChecker) isFBAFulfillment(shipsFrom string) bool {
	if shipsFrom == "" {
		return false
	}

	// 支持多语言站点的 Amazon 关键词匹配
	amazonKeywords := []string{
		"Amazon",
		"amazon",
		"AMAZON",
	}

	for _, keyword := range amazonKeywords {
		if regexp.MustCompile(keyword).MatchString(shipsFrom) {
			return true
		}
	}

	return false
}

// isAMZSeller 判断是否为亚马逊自营
func (c *FulfillmentChecker) isAMZSeller(sellerName string) bool {
	if sellerName == "" {
		return false
	}

	// 支持多语言站点的 Amazon 卖家名称匹配
	amazonSellerKeywords := []string{
		"Amazon",
		"amazon",
		"AMAZON",
		"Amazon.com",
		"Amazon.co.jp",
		"Amazon.de",
		"Amazon.fr",
		"Amazon.co.uk",
		"Amazon.es",
		"Amazon.it",
		"Amazon.com.mx",
	}

	for _, keyword := range amazonSellerKeywords {
		if regexp.MustCompile(keyword).MatchString(sellerName) {
			return true
		}
	}

	return false
}
