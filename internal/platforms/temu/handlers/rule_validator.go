// Package handlers 提供TEMU平台的规则验证功能
package handlers

import (
	"fmt"
	"regexp"
	"strconv"
	"task-processor/internal/common/management/api"
	"task-processor/internal/model"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// RuleValidator 规则验证器
type RuleValidator struct {
	logger             *logrus.Entry
	inventoryChecker   *InventoryChecker
	fulfillmentChecker *FulfillmentChecker
}

// NewRuleValidator 创建新的规则验证器
func NewRuleValidator(logger *logrus.Entry) *RuleValidator {
	return &RuleValidator{
		logger:             logger.WithField("component", "RuleValidator"),
		inventoryChecker:   NewInventoryChecker(logger),
		fulfillmentChecker: NewFulfillmentChecker(logger),
	}
}

// CheckSingleRule 检查单个规则（兼容旧接口）
func (v *RuleValidator) CheckSingleRule(product *model.Product, rule *api.FilterRuleRespDTO, ctx pipeline.TaskContext) bool {
	// 尝试转换为强类型上下文
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		result := v.CheckSingleRuleDetailedTemu(product, rule, temuCtx)
		return result.Passed
	}

	// 兼容旧的接口
	result := v.CheckSingleRuleDetailed(product, rule, ctx)
	return result.Passed
}

// CheckSingleRuleDetailed 详细检查单个规则（兼容旧接口）
func (v *RuleValidator) CheckSingleRuleDetailed(product *model.Product, rule *api.FilterRuleRespDTO, ctx pipeline.TaskContext) *FilterCheckResult {
	// 尝试转换为强类型上下文
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return v.CheckSingleRuleDetailedTemu(product, rule, temuCtx)
	}

	// 兼容旧的接口，使用基本检查
	return v.checkBasicRules(product, rule)
}

// CheckSingleRuleDetailedTemu 详细检查单个规则（强类型上下文）
func (v *RuleValidator) CheckSingleRuleDetailedTemu(product *model.Product, rule *api.FilterRuleRespDTO, temuCtx *temucontext.TemuTaskContext) *FilterCheckResult {
	// 价格检查
	if result := v.checkPriceRuleDetailedTemu(product, rule, temuCtx); !result.Passed {
		return result
	}

	// 评分检查
	if result := v.checkRatingRuleDetailed(product, rule); !result.Passed {
		return result
	}

	// 评论数量检查
	if result := v.checkReviewCountRuleDetailed(product, rule); !result.Passed {
		return result
	}

	// 库存检查
	if result := v.checkStockRuleDetailed(product, rule); !result.Passed {
		return result
	}

	// 配送方式检查
	if result := v.fulfillmentChecker.CheckFulfillmentTypeRuleDetailed(product, rule); !result.Passed {
		return result
	}

	return &FilterCheckResult{Passed: true}
}

// checkBasicRules 基本规则检查（不依赖上下文）
func (v *RuleValidator) checkBasicRules(product *model.Product, rule *api.FilterRuleRespDTO) *FilterCheckResult {
	// 价格检查（使用默认价格类型）
	if result := v.checkPriceRuleBasic(product, rule); !result.Passed {
		return result
	}

	// 评分检查
	if result := v.checkRatingRuleDetailed(product, rule); !result.Passed {
		return result
	}

	// 评论数量检查
	if result := v.checkReviewCountRuleDetailed(product, rule); !result.Passed {
		return result
	}

	// 库存检查
	if result := v.checkStockRuleDetailed(product, rule); !result.Passed {
		return result
	}

	// 配送方式检查
	if result := v.fulfillmentChecker.CheckFulfillmentTypeRuleDetailed(product, rule); !result.Passed {
		return result
	}

	return &FilterCheckResult{Passed: true}
}

// checkPriceRuleDetailedTemu 详细检查价格规则（强类型上下文）
func (v *RuleValidator) checkPriceRuleDetailedTemu(product *model.Product, rule *api.FilterRuleRespDTO, temuCtx *temucontext.TemuTaskContext) *FilterCheckResult {
	// 获取店铺配置的价格类型
	priceType := "final" // 默认价格类型
	if temuCtx.StoreInfo != nil && temuCtx.StoreInfo.PriceType != "" {
		priceType = temuCtx.StoreInfo.PriceType
	}

	// 根据价格类型获取价格
	price := v.getProductPrice(product, priceType)

	v.logger.WithFields(logrus.Fields{
		"asin":       product.Asin,
		"price_type": priceType,
		"price":      price,
		"price_min":  rule.PriceMin,
		"price_max":  rule.PriceMax,
	}).Debug("检查价格规则")

	// 最低价格检查
	if rule.PriceMin != nil && price < *rule.PriceMin {
		return &FilterCheckResult{
			Passed:        false,
			FailureReason: fmt.Sprintf("价格 %.2f 低于最低价格 %.2f", price, *rule.PriceMin),
			ProductValue:  price,
			RuleValue:     *rule.PriceMin,
		}
	}

	// 最高价格检查
	if rule.PriceMax != nil && price > *rule.PriceMax {
		return &FilterCheckResult{
			Passed:        false,
			FailureReason: fmt.Sprintf("价格 %.2f 高于最高价格 %.2f", price, *rule.PriceMax),
			ProductValue:  price,
			RuleValue:     *rule.PriceMax,
		}
	}

	return &FilterCheckResult{Passed: true}
}

// checkPriceRuleBasic 基本价格规则检查（不依赖上下文）
func (v *RuleValidator) checkPriceRuleBasic(product *model.Product, rule *api.FilterRuleRespDTO) *FilterCheckResult {
	// 使用默认价格类型
	price := v.getProductPrice(product, "final")

	v.logger.WithFields(logrus.Fields{
		"asin":       product.Asin,
		"price_type": "final",
		"price":      price,
		"price_min":  rule.PriceMin,
		"price_max":  rule.PriceMax,
	}).Debug("检查价格规则（基本模式）")

	// 最低价格检查
	if rule.PriceMin != nil && price < *rule.PriceMin {
		return &FilterCheckResult{
			Passed:        false,
			FailureReason: fmt.Sprintf("价格 %.2f 低于最低价格 %.2f", price, *rule.PriceMin),
			ProductValue:  price,
			RuleValue:     *rule.PriceMin,
		}
	}

	// 最高价格检查
	if rule.PriceMax != nil && price > *rule.PriceMax {
		return &FilterCheckResult{
			Passed:        false,
			FailureReason: fmt.Sprintf("价格 %.2f 高于最高价格 %.2f", price, *rule.PriceMax),
			ProductValue:  price,
			RuleValue:     *rule.PriceMax,
		}
	}

	return &FilterCheckResult{Passed: true}
}

// checkRatingRuleDetailed 详细检查评分规则
func (v *RuleValidator) checkRatingRuleDetailed(product *model.Product, rule *api.FilterRuleRespDTO) *FilterCheckResult {
	if rule.RatingMin == nil {
		return &FilterCheckResult{Passed: true}
	}

	rating := product.Rating

	v.logger.WithFields(logrus.Fields{
		"asin":       product.Asin,
		"rating":     rating,
		"rating_min": *rule.RatingMin,
	}).Debug("检查评分规则")

	if rating < *rule.RatingMin {
		return &FilterCheckResult{
			Passed:        false,
			FailureReason: fmt.Sprintf("评分 %.1f 低于最低评分 %.1f", rating, *rule.RatingMin),
			ProductValue:  rating,
			RuleValue:     *rule.RatingMin,
		}
	}

	return &FilterCheckResult{Passed: true}
}

// checkReviewCountRuleDetailed 详细检查评论数量规则
func (v *RuleValidator) checkReviewCountRuleDetailed(product *model.Product, rule *api.FilterRuleRespDTO) *FilterCheckResult {
	if rule.ReviewCountMin == nil {
		return &FilterCheckResult{Passed: true}
	}

	reviewCount := product.ReviewsCount

	v.logger.WithFields(logrus.Fields{
		"asin":             product.Asin,
		"review_count":     reviewCount,
		"review_count_min": *rule.ReviewCountMin,
	}).Debug("检查评论数量规则")

	if reviewCount < *rule.ReviewCountMin {
		return &FilterCheckResult{
			Passed:        false,
			FailureReason: fmt.Sprintf("评论数量 %d 低于最低评论数量 %d", reviewCount, *rule.ReviewCountMin),
			ProductValue:  reviewCount,
			RuleValue:     *rule.ReviewCountMin,
		}
	}

	return &FilterCheckResult{Passed: true}
}

// checkStockRuleDetailed 详细检查库存规则
func (v *RuleValidator) checkStockRuleDetailed(product *model.Product, rule *api.FilterRuleRespDTO) *FilterCheckResult {
	if rule.StockMin == nil {
		return &FilterCheckResult{Passed: true}
	}

	// 获取库存数量
	stock := v.inventoryChecker.GetInventory(product)

	v.logger.WithFields(logrus.Fields{
		"asin":      product.Asin,
		"stock":     stock,
		"stock_min": *rule.StockMin,
	}).Debug("检查库存规则")

	if stock < *rule.StockMin {
		return &FilterCheckResult{
			Passed:        false,
			FailureReason: fmt.Sprintf("库存 %d 低于最低库存 %d", stock, *rule.StockMin),
			ProductValue:  stock,
			RuleValue:     *rule.StockMin,
		}
	}

	return &FilterCheckResult{Passed: true}
}

// getProductPrice 获取产品价格的辅助函数
func (v *RuleValidator) getProductPrice(product *model.Product, priceType string) float64 {
	// 空指针检查
	if product == nil {
		v.logger.Warn("⚠️ getProductPrice 接收到 nil 产品指针，返回价格 0")
		return 0
	}

	// 获取运费
	freight := v.getFreight(product)
	var price float64

	// 根据价格类型获取价格
	switch priceType {
	case "special":
		// 特价，从 FinalPrice 获取
		price = product.FinalPrice
	case "original":
		// 原价，优先从 PricesBreakdown.ListPrice 获取
		if product.PricesBreakdown.ListPrice != nil {
			price = *product.PricesBreakdown.ListPrice
		} else {
			// 如果没有 ListPrice，使用 InitialPrice 作为备选
			price = product.InitialPrice
		}
	default:
		// 默认使用 FinalPrice
		price = product.FinalPrice
	}

	v.logger.WithFields(logrus.Fields{
		"asin":       product.Asin,
		"price_type": priceType,
		"base_price": price,
		"freight":    freight,
		"total":      price + freight,
	}).Debug("获取产品价格")

	return price + freight
}

// getFreight 获取运费
func (v *RuleValidator) getFreight(product *model.Product) float64 {
	// TODO: 从 delivery 信息中提取运费
	// 目前暂时返回 0，后续可以根据实际需求实现运费计算逻辑
	return 0
}

// InventoryChecker 库存检查器
type InventoryChecker struct {
	logger *logrus.Entry
}

// NewInventoryChecker 创建新的库存检查器
func NewInventoryChecker(logger *logrus.Entry) *InventoryChecker {
	return &InventoryChecker{
		logger: logger.WithField("component", "InventoryChecker"),
	}
}

// GetInventory 获取库存数量（支持多语言）
func (c *InventoryChecker) GetInventory(product *model.Product) int {
	// 优先：如果有明确的最大可用数量
	if product.MaxQuantityAvailable > 0 {
		c.logger.WithFields(logrus.Fields{
			"asin":  product.Asin,
			"stock": product.MaxQuantityAvailable,
		}).Debug("✅ 从 MaxQuantityAvailable 获取库存")
		return product.MaxQuantityAvailable
	}

	// 其次：尝试从 Availability 文本中提取具体数量
	availability := product.Availability

	// 支持多语言的库存数量提取
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
				c.logger.WithFields(logrus.Fields{
					"asin":    product.Asin,
					"pattern": pattern,
					"stock":   stock,
				}).Debug("✅ 从 Availability 文本中提取到库存数量")
				return stock
			}
		}
	}

	// 再次：检查 IsAvailable 字段
	if !product.IsAvailable {
		c.logger.WithFields(logrus.Fields{
			"asin":         product.Asin,
			"availability": product.Availability,
		}).Warn("⚠️ 产品标记为不可用且无法从文本提取库存，返回库存 0")
		return 0
	}

	// 最后：如果产品可用但没有具体数量，返回一个较大的值（表示充足库存）
	// 根据用户说明，In Stock代表库存大于30
	c.logger.WithFields(logrus.Fields{
		"asin":         product.Asin,
		"availability": product.Availability,
		"stock":        31,
	}).Debug("✅ 产品可用但无具体数量，返回默认库存 31")
	return 31
}
