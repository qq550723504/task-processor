// Package validation 提供通用的规则验证功能
package validation

import (
	"fmt"

	"task-processor/internal/domain/model"
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/pkg/product"
)

// RuleChecker 通用规则检查器
type RuleChecker struct{}

// NewRuleChecker 创建新的规则检查器
func NewRuleChecker() *RuleChecker {
	return &RuleChecker{}
}

// CheckPriceRange 校验价格范围
func (c *RuleChecker) CheckPriceRange(rule *api.FilterRuleRespDTO, price float64) error {
	// 检查最低价格
	if rule.PriceMin != nil && price < *rule.PriceMin {
		return fmt.Errorf("产品价格(%.2f)低于筛选规则最低价格(%.2f)", price, *rule.PriceMin)
	}

	// 检查最高价格
	if rule.PriceMax != nil && price > *rule.PriceMax {
		return fmt.Errorf("产品价格(%.2f)高于筛选规则最高价格(%.2f)", price, *rule.PriceMax)
	}

	return nil
}

// CheckInventory 校验库存
func (c *RuleChecker) CheckInventory(rule *api.FilterRuleRespDTO, inventory int) error {
	if rule.StockMin == nil {
		return nil
	}

	// 限制最大库存要求为30
	stockMin := *rule.StockMin
	if stockMin > 30 {
		stockMin = 30
	}

	if inventory < stockMin {
		return fmt.Errorf("产品库存(%d)低于筛选规则最低库存(%d)", inventory, stockMin)
	}

	return nil
}

// CheckRating 校验评论星级
func (c *RuleChecker) CheckRating(rule *api.FilterRuleRespDTO, rating float64) error {
	if rule.RatingMin == nil {
		return nil
	}

	if rating < *rule.RatingMin {
		return fmt.Errorf("产品评论星级(%.1f)低于筛选规则最低星级(%.1f)", rating, *rule.RatingMin)
	}

	return nil
}

// CheckReviewCount 校验评论数量
func (c *RuleChecker) CheckReviewCount(rule *api.FilterRuleRespDTO, reviewCount int) error {
	if rule.ReviewCountMin == nil {
		return nil
	}

	if reviewCount < *rule.ReviewCountMin {
		return fmt.Errorf("产品评论数量(%d)低于筛选规则最低评论数量(%d)", reviewCount, *rule.ReviewCountMin)
	}

	return nil
}

// CheckDeliveryTime 校验发货时效
func (c *RuleChecker) CheckDeliveryTime(rule *api.FilterRuleRespDTO, deliveryTimeHours int) error {
	if rule.DeliveryTimeMax == nil {
		return nil
	}

	// 限制最大发货时效为48小时
	maxHours := *rule.DeliveryTimeMax * 24
	if maxHours == 0 {
		maxHours = 48
	}

	if deliveryTimeHours > maxHours {
		return fmt.Errorf("产品发货时效(%d小时)超过筛选规则最大时效(%d小时)", deliveryTimeHours, maxHours)
	}

	return nil
}

// CheckFulfillmentType 校验配送方式
func (c *RuleChecker) CheckFulfillmentType(rule *api.FilterRuleRespDTO, amazonProduct *model.Product) error {
	// 如果规则未设置配送方式或设置为ALL，则不进行筛选
	if rule.FulfillmentType == "" || rule.FulfillmentType == "ALL" {
		return nil
	}

	// 空指针检查
	if amazonProduct == nil {
		return fmt.Errorf("产品信息为空，无法校验配送方式")
	}

	// 判断产品的配送方式
	isFBA := IsFBAFulfillment(amazonProduct.ShipsFrom)
	isAMZ := IsAMZSeller(amazonProduct.SellerName)

	// 根据规则要求进行校验
	switch rule.FulfillmentType {
	case "FBA":
		if !isFBA {
			return fmt.Errorf("产品配送方式不符合要求：规则要求FBA配送，但产品为FBM配送 (ships_from: %s)", amazonProduct.ShipsFrom)
		}
	case "FBM":
		if isFBA {
			return fmt.Errorf("产品配送方式不符合要求：规则要求FBM配送，但产品为FBA配送 (ships_from: %s)", amazonProduct.ShipsFrom)
		}
	case "AMZ":
		if !isAMZ {
			return fmt.Errorf("产品配送方式不符合要求：规则要求亚马逊自营，但卖家为 %s", amazonProduct.SellerName)
		}
	}

	return nil
}

// CheckAllRules 检查所有规则（便捷方法）
func (c *RuleChecker) CheckAllRules(rule *api.FilterRuleRespDTO, amazonProduct *model.Product, priceType string) error {
	// 获取价格
	price := product.GetProductPrice(amazonProduct, priceType)
	if err := c.CheckPriceRange(rule, price); err != nil {
		return err
	}

	// 获取库存
	inventory := product.GetInventory(amazonProduct)
	if err := c.CheckInventory(rule, inventory); err != nil {
		return err
	}

	// 检查评分
	if err := c.CheckRating(rule, amazonProduct.Rating); err != nil {
		return err
	}

	// 检查评论数量
	if err := c.CheckReviewCount(rule, amazonProduct.ReviewsCount); err != nil {
		return err
	}

	// 检查发货时效（默认24小时）
	if err := c.CheckDeliveryTime(rule, 24); err != nil {
		return err
	}

	// 检查配送方式
	if err := c.CheckFulfillmentType(rule, amazonProduct); err != nil {
		return err
	}

	return nil
}
