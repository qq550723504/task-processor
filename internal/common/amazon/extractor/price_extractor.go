// Package extractor 提供Amazon价格提取器的核心功能
package extractor

import (
	"strings"
	"task-processor/internal/model"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

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

	// 检查产品可用性（仅在明确不可用时才跳过价格提取）
	if product.Availability != "" && e.validator.IsUnavailableText(product.Availability) {
		logrus.WithFields(logrus.Fields{
			"asin":         product.Asin,
			"availability": product.Availability,
		}).Warn("❌ 产品不可用（根据Availability字段），跳过价格提取并设置 IsAvailable=false")
		product.FinalPrice = 0
		product.InitialPrice = 0
		product.Currency = "USD"
		product.IsAvailable = false
		return nil
	}

	// 提取价格文本
	priceText := e.extractPriceText(page)
	if priceText == "" {
		logrus.Warn("未找到价格信息，使用默认值")
		product.FinalPrice = 0
		product.InitialPrice = 0
		product.Currency = "USD"
		return nil
	}

	// 解析价格
	price := e.parser.ParsePrice(priceText)
	if price > 0 {
		product.FinalPrice = price
		product.InitialPrice = price
		product.Currency = e.currencyMgr.ExtractCurrency(priceText)
		logrus.Infof("解析到价格: %.2f %s", price, product.Currency)
	} else {
		logrus.Warnf("价格解析失败: %s", priceText)
		product.FinalPrice = 0
		product.InitialPrice = 0
		product.Currency = "USD"
	}

	// 提取原价（list price）
	e.listPriceExt.ExtractListPrice(page, product)

	return nil
}

// extractPriceText 提取价格文本
func (e *PriceExtractor) extractPriceText(page playwright.Page) string {
	// 完整价格选择器，按优先级排序
	completeSelectors := []string{
		"span.a-price.aok-align-center .a-offscreen",
		"#tp_price_block_total_price_ww .a-offscreen",
		".a-price.aok-align-center .a-offscreen",
		".a-price.a-text-price.a-size-medium.apexPriceToPay .a-offscreen",
		"#apex_desktop .a-price .a-offscreen",
		"#priceblock_dealprice",
		"#priceblock_ourprice",
		".a-price.a-text-price .a-offscreen",
		".a-price .a-offscreen",
		"span.a-price-range",
	}

	var priceText string
	for _, selector := range completeSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			text, _ := element.TextContent()
			if strings.TrimSpace(text) != "" {
				priceText = text
				break
			}
		}
	}

	// 如果没找到完整价格，尝试组合
	if priceText == "" {
		priceText = e.parser.ExtractCombinedPrice(page)
		if priceText != "" {
			logrus.Infof("组合价格提取成功: %s", priceText)
		}
	}

	return priceText
}
