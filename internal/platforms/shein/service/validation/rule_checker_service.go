package validation

import (
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	domainvalidation "task-processor/internal/domain/validation"
	"task-processor/internal/infra/clients/management/api"
	shein_model "task-processor/internal/platforms/shein/model"

	"github.com/sirupsen/logrus"
)

// FilterRuleChecker 筛选规则检查器（SHEIN平台包装器）
type FilterRuleChecker struct {
	checker *domainvalidation.RuleChecker
}

// NewFilterRuleChecker 创建新的筛选规则检查器
func NewFilterRuleChecker() *FilterRuleChecker {
	return &FilterRuleChecker{
		checker: domainvalidation.NewRuleChecker(),
	}
}

// GetProductPrice 获取产品价格（使用公共函数）
func GetProductPrice(amazonProduct *model.Product, priceType string) float64 {
	return product.GetProductPrice(amazonProduct, priceType)
}

// getInventory 获取库存（使用公共函数）
func (h *FilterRuleChecker) getInventory(amazonProduct *model.Product) int {
	return product.GetInventory(amazonProduct)
}

// getDeliveryTime 获取发货时效（根据JSON数据实现）
func (h *FilterRuleChecker) getDeliveryTime(amazonProduct *model.Product) int {
	// 从delivery信息中提取发货时效
	// 简化处理：默认返回24小时
	return 24
}

// CheckPriceRange 校验价格范围
func (c *FilterRuleChecker) CheckPriceRange(filterRule *api.FilterRuleRespDTO, priceValue float64) error {
	if err := c.checker.CheckPriceRange(filterRule, priceValue); err != nil {
		return shein_model.NewFilteredError(err.Error())
	}
	return nil
}

// CheckInventory 校验库存
func (c *FilterRuleChecker) CheckInventory(filterRule *api.FilterRuleRespDTO, inventory int) error {
	if err := c.checker.CheckInventory(filterRule, inventory); err != nil {
		return shein_model.NewFilteredError(err.Error())
	}
	return nil
}

// CheckDeliveryTime 校验发货时效
func (c *FilterRuleChecker) CheckDeliveryTime(filterRule *api.FilterRuleRespDTO, deliveryTime int) error {
	if err := c.checker.CheckDeliveryTime(filterRule, deliveryTime); err != nil {
		return shein_model.NewFilteredError(err.Error())
	}
	return nil
}

// CheckRating 校验评论星级
func (c *FilterRuleChecker) CheckRating(filterRule *api.FilterRuleRespDTO, rating float64) error {
	if err := c.checker.CheckRating(filterRule, rating); err != nil {
		return shein_model.NewFilteredError(err.Error())
	}
	return nil
}

// CheckReviewCount 校验评论数量
func (c *FilterRuleChecker) CheckReviewCount(filterRule *api.FilterRuleRespDTO, reviewCount int) error {
	if err := c.checker.CheckReviewCount(filterRule, reviewCount); err != nil {
		return shein_model.NewFilteredError(err.Error())
	}
	return nil
}

// CheckFulfillmentType 校验配送方式
func (c *FilterRuleChecker) CheckFulfillmentType(filterRule *api.FilterRuleRespDTO, amazonProduct *model.Product) error {
	// 判断产品的配送方式（用于日志）
	isFBA := domainvalidation.IsFBAFulfillment(amazonProduct.ShipsFrom)
	isAMZ := domainvalidation.IsAMZSeller(amazonProduct.SellerName)

	logrus.WithFields(logrus.Fields{
		"asin":          amazonProduct.Asin,
		"ships_from":    amazonProduct.ShipsFrom,
		"seller_name":   amazonProduct.SellerName,
		"is_fba":        isFBA,
		"is_amz":        isAMZ,
		"required_type": filterRule.FulfillmentType,
	}).Infof("🔍 校验配送方式")

	// 使用公共验证器检查
	if err := c.checker.CheckFulfillmentType(filterRule, amazonProduct); err != nil {
		return shein_model.NewFilteredError(err.Error())
	}

	return nil
}
