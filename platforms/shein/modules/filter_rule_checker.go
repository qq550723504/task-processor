package modules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"task-processor/common/amazon"
	"task-processor/common/management/api"
)

// FilterRuleChecker 筛选规则检查器
type FilterRuleChecker struct{}

// NewFilterRuleChecker 创建新的筛选规则检查器
func NewFilterRuleChecker() *FilterRuleChecker {
	return &FilterRuleChecker{}
}

// getProductPrice 获取产品价格
func GetProductPrice(amazonProduct *amazon.Product, priceType string) float64 {
	// 根据价格类型获取价格//TODO:还要处理运费
	freight := getFreight(amazonProduct)
	var price float64
	// 根据项目规范："special"表示特价，"original"表示原价
	switch priceType {
	case "special":
		// 特价，从buyboxprices.finalprice获取
		price = amazonProduct.FinalPrice
	case "original":
		// 原价，从initialprice获取
		price = amazonProduct.InitialPrice
	default:
		// 默认使用buyboxprices.finalprice
		price = amazonProduct.FinalPrice
	}

	return freight + price
}

// getFreight 获取运费//TODO:还要处理运费
func getFreight(amazonProduct *amazon.Product) float64 {
	// 从delivery信息中提取运费

	// 假设运费为0
	return float64(0)
}

// getInventory 获取库存（根据JSON数据实现）
func (h *FilterRuleChecker) getInventory(amazonProduct *amazon.Product) int {
	if strings.EqualFold(amazonProduct.Availability, "In Stock") {
		// 根据用户说明，In Stock代表库存大于30
		return 31 // 返回一个大于30的值
	} else if amazonProduct.MaxQuantityAvailable != 0 {
		return amazonProduct.MaxQuantityAvailable
	} else {
		// 使用正则提取库存数量，支持多种格式
		patterns := []string{
			`(?i)only\s+(\d+)\s+left\s+in\s+stock`,    // "Only 13 left in stock - order soon."
			`(?i)(\d+)\s+left\s+in\s+stock`,           // "13 left in stock"
			`(?i)(\d+)\s+in\s+stock`,                  // "13 in stock"
			`(?i)(\d+)\s+available`,                   // "13 available"
			`(?i)(\d+)\s+remaining`,                   // "13 remaining"
			`(?i)stock:\s*(\d+)`,                      // "Stock: 13"
			`(?i)quantity:\s*(\d+)`,                   // "Quantity: 13"
			`(?i)(\d+)\s+units?\s+(?:left|available)`, // "13 units left" or "13 unit available"
		}

		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			if matches := re.FindStringSubmatch(amazonProduct.Availability); len(matches) > 1 {
				if stock, err := strconv.Atoi(matches[1]); err == nil {
					return stock
				}
			}
		}
		return 0
	}
}

// getDeliveryTime 获取发货时效（根据JSON数据实现）
func (h *FilterRuleChecker) getDeliveryTime(amazonProduct *amazon.Product) int {
	// 从delivery信息中提取发货时效
	// 简化处理：默认返回24小时
	// 实际项目中可能需要解析delivery数组中的文本信息来获取更准确的发货时效
	// 根据JSON数据中的示例："FREE delivery Sunday, August 3 on orders shipped by Amazon over $35"
	// 可以提取发货时效信息，但这里简化处理
	return 24
}

// CheckPriceRange 校验价格范围
func (c *FilterRuleChecker) CheckPriceRange(filterRule *api.FilterRuleRespDTO, priceValue float64) error {
	// 检查最低价格
	if filterRule.PriceMin != nil {
		if priceValue < *filterRule.PriceMin {
			return NewFilteredError(fmt.Sprintf("产品价格(%.2f)低于筛选规则最低价格(%.2f)", priceValue, *filterRule.PriceMin))
		}
	}

	// 检查最高价格
	if filterRule.PriceMax != nil {
		if priceValue > *filterRule.PriceMax {
			return NewFilteredError(fmt.Sprintf("产品价格(%.2f)高于筛选规则最高价格(%.2f)", priceValue, *filterRule.PriceMax))
		}
	}

	return nil
}

// CheckInventory 校验库存
func (c *FilterRuleChecker) CheckInventory(filterRule *api.FilterRuleRespDTO, inventory int) error {
	if filterRule.StockMin == nil {
		return nil
	}

	if *filterRule.StockMin > 30 {
		thirty := 30
		filterRule.StockMin = &thirty
	}

	if inventory < *filterRule.StockMin {
		return NewFilteredError(fmt.Sprintf("产品库存(%d)低于筛选规则最低库存(%d)", inventory, *filterRule.StockMin))
	}
	return nil
}

// CheckDeliveryTime 校验发货时效
func (c *FilterRuleChecker) CheckDeliveryTime(filterRule *api.FilterRuleRespDTO, deliveryTime int) error {
	if filterRule.DeliveryTimeMax == nil {
		return nil
	}

	if *filterRule.DeliveryTimeMax == 0 {
		fortyEight := 48
		filterRule.DeliveryTimeMax = &fortyEight
	}

	if deliveryTime > *filterRule.DeliveryTimeMax*24 {
		return NewFilteredError(fmt.Sprintf("产品发货时效(%d小时)超过筛选规则最大时效(%d小时)", deliveryTime, *filterRule.DeliveryTimeMax))
	}
	return nil
}

// CheckRating 校验评论星级
func (c *FilterRuleChecker) CheckRating(filterRule *api.FilterRuleRespDTO, rating float64) error {
	if filterRule.RatingMin == nil {
		return nil
	}

	if rating < *filterRule.RatingMin {
		return NewFilteredError(fmt.Sprintf("产品评论星级(%.1f)低于筛选规则最低星级(%.1f)", rating, *filterRule.RatingMin))
	}
	return nil
}

// CheckReviewCount 校验评论数量
func (c *FilterRuleChecker) CheckReviewCount(filterRule *api.FilterRuleRespDTO, reviewCount int) error {
	if filterRule.ReviewCountMin == nil {
		return nil
	}

	if reviewCount < *filterRule.ReviewCountMin {
		return NewFilteredError(fmt.Sprintf("产品评论数量(%d)低于筛选规则最低评论数量(%d)", reviewCount, *filterRule.ReviewCountMin))
	}
	return nil
}
