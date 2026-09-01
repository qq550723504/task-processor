package validation

import (
	"task-processor/internal/marketplace/productpolicy"
	"task-processor/internal/model"

	shein "task-processor/internal/shein"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// FilterRuleChecker 筛选规则检查器（SHEIN平台包装器）
type FilterRuleChecker struct {
	checker *productpolicy.RuleChecker
}

// NewFilterRuleChecker 创建新的筛选规则检查器
func NewFilterRuleChecker() *FilterRuleChecker {
	return &FilterRuleChecker{
		checker: productpolicy.NewRuleChecker(),
	}
}

// GetProductPrice 获取产品价格（使用公共函数）
func GetProductPrice(amazonProduct *model.Product, priceType string) float64 {
	return productpolicy.GetProductPrice(amazonProduct, priceType)
}

// getInventory 获取库存（使用公共函数）
func (h *FilterRuleChecker) getInventory(amazonProduct *model.Product) int {
	return productpolicy.GetInventory(amazonProduct)
}

// getDeliveryTime 获取发货时效（固定返回24小时）
func (h *FilterRuleChecker) getDeliveryTime(_ *model.Product) int {
	return 24
}

// CheckPriceRange 校验价格范围
func (c *FilterRuleChecker) CheckPriceRange(filterRule *productpolicy.FilterRule, priceValue float64) error {
	if err := c.checker.CheckPriceRange(filterRule, priceValue); err != nil {
		return shein.NewFilteredError(err.Error())
	}
	return nil
}

// CheckInventory 校验库存
func (c *FilterRuleChecker) CheckInventory(filterRule *productpolicy.FilterRule, inventory int) error {
	if err := c.checker.CheckInventory(filterRule, inventory); err != nil {
		return shein.NewFilteredError(err.Error())
	}
	return nil
}

// CheckDeliveryTime 校验发货时效
func (c *FilterRuleChecker) CheckDeliveryTime(filterRule *productpolicy.FilterRule, deliveryTime int) error {
	if err := c.checker.CheckDeliveryTime(filterRule, deliveryTime); err != nil {
		return shein.NewFilteredError(err.Error())
	}
	return nil
}

// CheckRating 校验评论星级
func (c *FilterRuleChecker) CheckRating(filterRule *productpolicy.FilterRule, rating float64) error {
	if err := c.checker.CheckRating(filterRule, rating); err != nil {
		return shein.NewFilteredError(err.Error())
	}
	return nil
}

// CheckReviewCount 校验评论数量
func (c *FilterRuleChecker) CheckReviewCount(filterRule *productpolicy.FilterRule, reviewCount int) error {
	if err := c.checker.CheckReviewCount(filterRule, reviewCount); err != nil {
		return shein.NewFilteredError(err.Error())
	}
	return nil
}

// CheckFulfillmentType 校验配送方式
func (c *FilterRuleChecker) CheckFulfillmentType(filterRule *productpolicy.FilterRule, amazonProduct *model.Product) error {
	isFBA := productpolicy.IsFBAFulfillment(amazonProduct.ShipsFrom)
	isAMZ := productpolicy.IsAMZSeller(amazonProduct.SellerName)

	logger.GetGlobalLogger("shein/validation").WithFields(logrus.Fields{
		"asin":          amazonProduct.Asin,
		"ships_from":    amazonProduct.ShipsFrom,
		"seller_name":   amazonProduct.SellerName,
		"is_fba":        isFBA,
		"is_amz":        isAMZ,
		"required_type": filterRule.FulfillmentType,
	}).Infof("🔍 校验配送方式")

	if err := c.checker.CheckFulfillmentType(filterRule, amazonProduct); err != nil {
		return shein.NewFilteredError(err.Error())
	}
	return nil
}
