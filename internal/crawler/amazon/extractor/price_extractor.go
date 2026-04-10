// Package extractor 提供Amazon价格提取器的核心功能
package extractor

import (
	"strings"
	"task-processor/internal/core/logger"
	"task-processor/internal/model"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

var primaryPriceContainers = []string{
	"#corePriceDisplay_desktop_feature_div",
	"#corePrice_feature_div",
	"#apex_desktop",
}

// PriceExtractor 价格提取器
type PriceExtractor struct {
	// Marketplace 用于区分不同站点 (US, JP, UK, DE, FR, IT, ES, etc.)
	Marketplace string

	// 依赖注入的组件
	validator    *PriceValidator
	parser       *PriceParser
	currencyMgr  *CurrencyManager
	listPriceExt *ListPriceExtractor
}

// NewPriceExtractor 创建价格提取器
func NewPriceExtractor(marketplace string) *PriceExtractor {
	return &PriceExtractor{
		Marketplace:  marketplace,
		validator:    NewPriceValidator(marketplace),
		parser:       NewPriceParser(marketplace),
		currencyMgr:  NewCurrencyManager(marketplace),
		listPriceExt: NewListPriceExtractor(marketplace),
	}
}

// HasValidPrice 快速检查产品是否有有效价格
func (e *PriceExtractor) HasValidPrice(page playwright.Page) bool {
	return e.validator.HasValidPrice(page)
}

// Extract 提取价格信息
func (e *PriceExtractor) Extract(page playwright.Page, product *model.Product) error {

	// 站点默认货币，用于无价格/不可用时的兜底
	defaultCurrency := e.currencyMgr.GetDefaultCurrencyByMarketplace()

	// 检查产品可用性（仅在明确不可用时才跳过价格提取）
	if product.Availability != "" && e.validator.IsUnavailableText(product.Availability) {
		logger.GetGlobalLogger("crawler/amazon").WithFields(logrus.Fields{
			"asin":         product.Asin,
			"availability": product.Availability,
		}).Warn("❌ 产品不可用（根据Availability字段），跳过价格提取并设置 IsAvailable=false")
		product.FinalPrice = 0
		product.InitialPrice = 0
		product.Currency = defaultCurrency
		product.IsAvailable = false
		return nil
	}

	// 提取价格文本
	priceTextStartedAt := time.Now()
	priceText := e.extractPriceText(page)
	logger.GetGlobalLogger("crawler/amazon").Infof("价格文本提取完成 (耗时=%s)", time.Since(priceTextStartedAt).Round(time.Millisecond))
	if priceText == "" {
		logger.GetGlobalLogger("crawler/amazon").Warn("未找到价格信息，使用默认值")
		product.FinalPrice = 0
		product.InitialPrice = 0
		product.Currency = defaultCurrency
		return nil
	}

	// 解析价格
	price := e.parser.ParsePrice(priceText)
	if price > 0 {
		product.FinalPrice = price
		product.InitialPrice = 0

		// 从价格文本中提取货币
		extractedCurrency := e.currencyMgr.ExtractCurrency(priceText)

		// 获取站点的默认货币
		expectedCurrency := e.currencyMgr.GetDefaultCurrencyByMarketplace()

		// 验证货币是否匹配
		if extractedCurrency != expectedCurrency {
			logger.GetGlobalLogger("crawler/amazon").Warnf("⚠️ 货币不匹配 - 页面显示: %s, 站点期望: %s",
				extractedCurrency, expectedCurrency)
			logger.GetGlobalLogger("crawler/amazon").Warnf("💡 提示: 货币设置可能失败，请检查货币设置器的日志")
			// 使用页面显示的货币（因为这是实际显示的价格）
			product.Currency = extractedCurrency
		} else {
			product.Currency = extractedCurrency
		}

		logger.GetGlobalLogger("crawler/amazon").Infof("解析到价格: %.2f %s", price, product.Currency)
	} else {
		logger.GetGlobalLogger("crawler/amazon").Warnf("价格解析失败: %s", priceText)
		product.FinalPrice = 0
		product.InitialPrice = 0
		product.Currency = defaultCurrency
	}

	// 提取原价（list price）
	listPriceStartedAt := time.Now()
	if e.listPriceExt.ShouldExtract(page, product) {
		e.listPriceExt.ExtractListPrice(page, product)
	}
	e.syncInitialPriceWithListPrice(product)
	logger.GetGlobalLogger("crawler/amazon").Infof("原价提取完成 (耗时=%s)", time.Since(listPriceStartedAt).Round(time.Millisecond))

	return nil
}

func (e *PriceExtractor) syncInitialPriceWithListPrice(product *model.Product) {
	if product == nil {
		return
	}

	product.InitialPrice = 0
	if product.PricesBreakdown.ListPrice != nil && *product.PricesBreakdown.ListPrice > 0 {
		product.InitialPrice = *product.PricesBreakdown.ListPrice
	}
}

// extractPriceText 提取价格文本
func (e *PriceExtractor) extractPriceText(page playwright.Page) string {
	primarySelectors := buildScopedSelectors(primaryPriceContainers, []string{
		".a-price.aok-align-center .a-offscreen",
		".a-price.priceToPay .a-offscreen",
		".a-price.apex-pricetopay-value .a-offscreen",
		".a-price .a-offscreen",
	})
	primarySelectors = append(primarySelectors, "#tp_price_block_total_price_ww .a-offscreen")

	if priceText := firstNonEmptyText(page, primarySelectors); priceText != "" {
		return priceText
	}

	if priceText := e.parser.ExtractCombinedPriceInScopes(page, primaryPriceContainers); priceText != "" {
		logger.GetGlobalLogger("crawler/amazon").Infof("主价格容器组合价格提取成功: %s", priceText)
		return priceText
	}

	secondarySelectors := []string{
		"#tp_price_block_total_price_ww .a-offscreen",
		".a-price.a-text-price.a-size-medium.apexPriceToPay .a-offscreen",
		"span.a-price.aok-align-center .a-offscreen",
		".a-price.aok-align-center .a-offscreen",
		"#priceblock_dealprice",
		"#priceblock_ourprice",
		".a-price.a-text-price .a-offscreen",
		"span.a-price-range",
	}

	if priceText := firstNonEmptyText(page, secondarySelectors); priceText != "" {
		return priceText
	}

	if priceText := e.parser.ExtractCombinedPrice(page); priceText != "" {
		logger.GetGlobalLogger("crawler/amazon").Infof("组合价格提取成功: %s", priceText)
		return priceText
	}

	return firstNonEmptyText(page, []string{
		".a-price .a-offscreen",
	})
}

func firstNonEmptyText(page playwright.Page, selectors []string) string {
	for _, selector := range selectors {
		element, err := page.QuerySelector(selector)
		if err != nil || element == nil {
			continue
		}
		text, err := element.TextContent()
		if err != nil {
			continue
		}
		if trimmed := strings.TrimSpace(text); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func buildScopedSelectors(scopes []string, selectors []string) []string {
	if len(scopes) == 0 {
		return append([]string(nil), selectors...)
	}

	scoped := make([]string, 0, len(scopes)*len(selectors))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		for _, selector := range selectors {
			selector = strings.TrimSpace(selector)
			if selector == "" {
				continue
			}
			scoped = append(scoped, scope+" "+selector)
		}
	}
	return scoped
}
