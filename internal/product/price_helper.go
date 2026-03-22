// Package product 提供产品相关的公共工具函数
package product

import (
	"task-processor/internal/core/logger"
	"regexp"
	"strconv"
	"task-processor/internal/model"

)

// GetProductPrice 根据价格类型获取Amazon产品价格（包含运费）
// priceType: "special" 表示特价（FinalPrice），"original" 表示原价（ListPrice或InitialPrice）
func GetProductPrice(amazonProduct *model.Product, priceType string) float64 {
	// 空指针检查
	if amazonProduct == nil {
		logger.GetGlobalLogger("product/price_helper.go").Warn("⚠️ GetProductPrice 接收到 nil 产品指针，返回价格 0")
		return 0
	}

	// 获取运费
	freight := GetFreight(amazonProduct)
	var price float64

	// 根据价格类型获取价格
	switch priceType {
	case "special":
		// 特价，使用最终价格
		price = amazonProduct.FinalPrice
	case "original":
		// 原价，优先使用list_price，否则使用initial_price
		if amazonProduct.PricesBreakdown.ListPrice != nil {
			price = *amazonProduct.PricesBreakdown.ListPrice
		} else {
			price = amazonProduct.InitialPrice
		}
	default:
		// 默认使用最终价格
		price = amazonProduct.FinalPrice
	}

	return freight + price
}

// GetFreight 获取运费
// TODO: 从delivery信息中提取运费，目前返回0
func GetFreight(amazonProduct *model.Product) float64 {
	// 从delivery信息中提取运费
	// 暂时返回0，后续可扩展
	return 0.0
}

// GetInventory 从Amazon产品中提取库存数量（支持多语言）
func GetInventory(amazonProduct *model.Product) int {
	// 空指针检查
	if amazonProduct == nil {
		logger.GetGlobalLogger("product/price_helper.go").Warn("⚠️ GetInventory 接收到 nil 产品指针，返回库存 0")
		return 0
	}

	// 优先：如果有明确的最大可用数量
	if amazonProduct.MaxQuantityAvailable > 0 {
		return amazonProduct.MaxQuantityAvailable
	}

	// 其次：使用正则从 Availability 文本中提取库存数量，支持多语言格式
	stock := extractStockFromAvailability(amazonProduct.Availability)
	if stock > 0 {
		return stock
	}

	// 再次：检查 IsAvailable 字段
	if !amazonProduct.IsAvailable {
		return 0
	}

	// 最后：如果产品可用但没有具体数量，返回一个较大的值（表示充足库存）
	// 根据用户说明，In Stock代表库存大于30
	return 31
}

// extractStockFromAvailability 从 Availability 文本中提取库存数量（支持多语言）
func extractStockFromAvailability(availability string) int {
	if availability == "" {
		return 0
	}

	// 支持多语言的库存数量提取模式
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
		if matches := re.FindStringSubmatch(availability); len(matches) > 1 {
			if stock, err := strconv.Atoi(matches[1]); err == nil {
				return stock
			}
		}
	}

	return 0
}
