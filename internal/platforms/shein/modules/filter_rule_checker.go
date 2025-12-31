package modules

import (
	"fmt"
	"regexp"
	"strconv"
	"task-processor/internal/common/management/api"
	"task-processor/internal/domain/model"

	"github.com/sirupsen/logrus"
)

// FilterRuleChecker 筛选规则检查器
type FilterRuleChecker struct{}

// NewFilterRuleChecker 创建新的筛选规则检查器
func NewFilterRuleChecker() *FilterRuleChecker {
	return &FilterRuleChecker{}
}

// getProductPrice 获取产品价格
func GetProductPrice(amazonProduct *model.Product, priceType string) float64 {
	// 空指针检查
	if amazonProduct == nil {
		logrus.Warn("⚠️ GetProductPrice 接收到 nil 产品指针，返回价格 0")
		return 0
	}

	// 根据价格类型获取价格//TODO:还要处理运费
	freight := getFreight(amazonProduct)
	var price float64
	// 根据项目规范："special"表示特价，"original"表示原价
	switch priceType {
	case "special":
		// 特价，从buyboxprices.finalprice获取
		price = amazonProduct.FinalPrice
	case "original":
		// 原价，从 prices_breakdown.list_price 获取
		if amazonProduct.PricesBreakdown.ListPrice != nil {
			price = *amazonProduct.PricesBreakdown.ListPrice
		} else {
			// 如果没有 list_price，使用 initial_price 作为备选
			price = amazonProduct.InitialPrice
		}
	default:
		// 默认使用buyboxprices.finalprice
		price = amazonProduct.FinalPrice
	}

	return freight + price
}

// getFreight 获取运费//TODO:还要处理运费
func getFreight(amazonProduct *model.Product) float64 {
	// 从delivery信息中提取运费

	// 假设运费为0
	return float64(0)
}

// getInventory 获取库存（根据JSON数据实现，支持多语言）
func (h *FilterRuleChecker) getInventory(amazonProduct *model.Product) int {
	logrus.WithFields(logrus.Fields{
		"asin":         amazonProduct.Asin,
		"availability": amazonProduct.Availability,
		"is_available": amazonProduct.IsAvailable,
	}).Debug("🔍 开始获取库存信息")

	// 优先：如果有明确的最大可用数量
	if amazonProduct.MaxQuantityAvailable > 0 {
		logrus.WithFields(logrus.Fields{
			"asin":  amazonProduct.Asin,
			"stock": amazonProduct.MaxQuantityAvailable,
		}).Debug("✅ 从 MaxQuantityAvailable 获取库存")
		return amazonProduct.MaxQuantityAvailable
	}

	// 其次：使用正则从 Availability 文本中提取库存数量，支持多语言格式
	patterns := []string{
		// 英语
		`(?i)only\s+(\d+)\s+left`,                 // "Only 13 left in stock"
		`(?i)(\d+)\s+left`,                        // "13 left in stock"
		`(?i)(\d+)\s+in\s+stock`,                  // "13 in stock"
		`(?i)(\d+)\s+available`,                   // "13 available"
		`(?i)(\d+)\s+remaining`,                   // "13 remaining"
		`(?i)stock:\s*(\d+)`,                      // "Stock: 13"
		`(?i)quantity:\s*(\d+)`,                   // "Quantity: 13"
		`(?i)(\d+)\s+units?\s+(?:left|available)`, // "13 units left"
		// 西班牙语
		`(?i)quedan\s+(\d+)`,       // "quedan 13"
		`(?i)(\d+)\s+disponibles?`, // "13 disponibles"
		`(?i)solo\s+(\d+)`,         // "solo 13"
		// 日语
		`(?i)残り\s*(\d+)`, // "残り13"
		`(?i)(\d+)\s*個`,  // "13個"
		`(?i)(\d+)\s*点`,  // "13点"
		// 德语
		`(?i)noch\s+(\d+)`,       // "noch 13"
		`(?i)(\d+)\s+verfügbar`,  // "13 verfügbar"
		`(?i)nur\s+noch\s+(\d+)`, // "nur noch 13"
		// 法语
		`(?i)reste\s+(\d+)`,        // "reste 13"
		`(?i)(\d+)\s+disponibles?`, // "13 disponible(s)"
		`(?i)seulement\s+(\d+)`,    // "seulement 13"
		// 意大利语
		`(?i)rimangono\s+(\d+)`,    // "rimangono 13"
		`(?i)(\d+)\s+disponibili?`, // "13 disponibili"
		`(?i)solo\s+(\d+)`,         // "solo 13"
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(amazonProduct.Availability); len(matches) > 1 {
			if stock, err := strconv.Atoi(matches[1]); err == nil {
				logrus.WithFields(logrus.Fields{
					"asin":    amazonProduct.Asin,
					"pattern": pattern,
					"stock":   stock,
				}).Debug("✅ 从 Availability 文本中提取到库存数量")
				return stock
			}
		}
	}

	// 再次：检查 IsAvailable 字段
	if !amazonProduct.IsAvailable {
		logrus.WithFields(logrus.Fields{
			"asin":         amazonProduct.Asin,
			"availability": amazonProduct.Availability,
		}).Warn("⚠️ 产品标记为不可用且无法从文本提取库存，返回库存 0")
		return 0
	}

	// 最后：如果产品可用但没有具体数量，返回一个较大的值（表示充足库存）
	// 根据用户说明，In Stock代表库存大于30
	logrus.WithFields(logrus.Fields{
		"asin":         amazonProduct.Asin,
		"availability": amazonProduct.Availability,
		"stock":        31,
	}).Debug("✅ 产品可用但无具体数量，返回默认库存 31")
	return 31
}

// getDeliveryTime 获取发货时效（根据JSON数据实现）
func (h *FilterRuleChecker) getDeliveryTime(amazonProduct *model.Product) int {
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

// CheckFulfillmentType 校验配送方式
func (c *FilterRuleChecker) CheckFulfillmentType(filterRule *api.FilterRuleRespDTO, amazonProduct *model.Product) error {
	// 如果规则未设置配送方式或设置为ALL，则不进行筛选
	if filterRule.FulfillmentType == "" || filterRule.FulfillmentType == "ALL" {
		return nil
	}

	// 空指针检查
	if amazonProduct == nil {
		return NewFilteredError("产品信息为空，无法校验配送方式")
	}

	// 判断产品的配送方式
	isFBA := c.isFBAFulfillment(amazonProduct.ShipsFrom)
	isAMZ := c.isAMZSeller(amazonProduct.SellerName)

	logrus.WithFields(logrus.Fields{
		"asin":          amazonProduct.Asin,
		"ships_from":    amazonProduct.ShipsFrom,
		"seller_name":   amazonProduct.SellerName,
		"is_fba":        isFBA,
		"is_amz":        isAMZ,
		"required_type": filterRule.FulfillmentType,
	}).Infof("🔍 校验配送方式")

	// 根据规则要求进行校验
	switch filterRule.FulfillmentType {
	case "FBA":
		if !isFBA {
			return NewFilteredError(fmt.Sprintf("产品配送方式不符合要求：规则要求FBA配送，但产品为FBM配送 (ships_from: %s)", amazonProduct.ShipsFrom))
		}
	case "FBM":
		if isFBA {
			return NewFilteredError(fmt.Sprintf("产品配送方式不符合要求：规则要求FBM配送，但产品为FBA配送 (ships_from: %s)", amazonProduct.ShipsFrom))
		}
	case "AMZ":
		if !isAMZ {
			return NewFilteredError(fmt.Sprintf("产品配送方式不符合要求：规则要求亚马逊自营，但卖家为 %s", amazonProduct.SellerName))
		}
	default:
		logrus.WithField("fulfillmentType", filterRule.FulfillmentType).Warn("⚠️ 未知的配送方式类型")
	}

	return nil
}

// isFBAFulfillment 判断是否为FBA配送
// 通过检查 ships_from 字段是否包含 "Amazon" 关键词来判断
func (c *FilterRuleChecker) isFBAFulfillment(shipsFrom string) bool {
	if shipsFrom == "" {
		return false
	}

	// 支持多语言站点的 Amazon 关键词匹配
	// 包括：Amazon.com, Amazon.co.jp, Amazon.de, Amazon.fr, Amazon.co.uk 等
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
// 通过检查 seller_name 字段是否包含 "Amazon" 关键词来判断
func (c *FilterRuleChecker) isAMZSeller(sellerName string) bool {
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
