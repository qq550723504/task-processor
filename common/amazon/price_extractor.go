package amazon

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/playwright-community/playwright-go"
)

// PriceExtractor 价格提取器
type PriceExtractor struct{}

// HasValidPrice 快速检查产品是否有有效价格
func (e *PriceExtractor) HasValidPrice(page playwright.Page) bool {
	priceSelectors := []string{
		".a-price .a-offscreen",
		"#priceblock_ourprice",
		"#priceblock_dealprice",
		"#priceblock_saleprice",
		".a-price-whole",
		"#corePrice_feature_div .a-price .a-offscreen",
		"#corePriceDisplay_desktop_feature_div .a-price .a-offscreen",
		".priceToPay .a-offscreen",
		"[data-a-color='price'] .a-offscreen",
		".apexPriceToPay .a-offscreen",
	}

	for _, selector := range priceSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			text, err := element.TextContent()
			if err == nil && text != "" {
				// 检查各种货币符号
				if strings.Contains(text, "$") || strings.Contains(text, "£") ||
					strings.Contains(text, "€") || strings.Contains(text, "¥") ||
					strings.Contains(text, "C$") || strings.Contains(text, "A$") ||
					strings.Contains(text, "CAD") || strings.Contains(text, "AUD") {
					log.Printf("找到有效价格: %s", strings.TrimSpace(text))
					return true
				}
			}
		}
	}

	unavailableSelectors := []string{
		"#availability .a-color-state",
		"#availability .a-color-price",
		"#availability span",
	}

	for _, selector := range unavailableSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			text, err := element.TextContent()
			if err == nil {
				lowerText := strings.ToLower(strings.TrimSpace(text))
				if strings.Contains(lowerText, "unavailable") ||
					strings.Contains(lowerText, "out of stock") ||
					strings.Contains(lowerText, "currently unavailable") {
					log.Printf("产品不可用: %s", text)
					return false
				}
			}
		}
	}

	log.Println("未找到有效价格")
	return false
}

func (e *PriceExtractor) Extract(page playwright.Page, product *Product) error {
	// 检查产品可用性
	if product.Availability != "" && e.isUnavailableText(product.Availability) {
		log.Printf("产品不可用（根据Availability字段: %s），跳过价格提取", product.Availability)
		product.FinalPrice = 0
		product.InitialPrice = 0
		product.Currency = "USD"
		product.IsAvailable = false
		return nil
	}

	// 如果Availability字段为空或不明确，再检查页面
	if !e.isProductAvailable(page) {
		log.Println("产品不可用（根据页面检查），跳过价格提取")
		product.FinalPrice = 0
		product.InitialPrice = 0
		product.Currency = "USD"
		product.IsAvailable = false
		return nil
	}

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
				log.Printf("从选择器 %s 获取到完整价格: %s", selector, priceText)
				break
			}
		}
	}

	// 如果没找到完整价格，尝试组合
	if priceText == "" {
		priceText = e.extractCombinedPrice(page)
		if priceText != "" {
			log.Printf("组合价格提取成功: %s", priceText)
		}
	}

	if priceText == "" {
		log.Println("未找到价格信息，使用默认值")
		product.FinalPrice = 0
		product.InitialPrice = 0
		product.Currency = "USD"
		return nil
	}

	// 解析价格
	price := e.parsePrice(priceText)
	if price > 0 {
		product.FinalPrice = price
		product.InitialPrice = price
		product.Currency = e.extractCurrency(priceText)
		log.Printf("解析到价格: %.2f %s", price, product.Currency)
	} else {
		log.Printf("价格解析失败: %s", priceText)
		product.FinalPrice = 0
		product.InitialPrice = 0
		product.Currency = "USD"
	}

	// 提取原价（list price）
	e.extractListPrice(page, product)

	return nil
}

func (e *PriceExtractor) extractCombinedPrice(page playwright.Page) string {
	wholeSelectors := []string{
		".a-price-whole",
		".a-price .a-price-whole",
		"span.a-price-whole",
	}

	fractionSelectors := []string{
		".a-price-fraction",
		".a-price .a-price-fraction",
		"span.a-price-fraction",
	}

	var wholePart, fractionPart string

	for _, selector := range wholeSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			text, _ := element.TextContent()
			wholePart = strings.TrimSpace(text)
			if wholePart != "" {
				log.Printf("获取到整数部分: %s (选择器: %s)", wholePart, selector)
				break
			}
		}
	}

	for _, selector := range fractionSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			text, _ := element.TextContent()
			fractionPart = strings.TrimSpace(text)
			if fractionPart != "" {
				log.Printf("获取到小数部分: %s (选择器: %s)", fractionPart, selector)
				break
			}
		}
	}

	if wholePart != "" {
		wholePart = strings.TrimSuffix(wholePart, ".")
		if fractionPart != "" {
			return fmt.Sprintf("$%s.%s", wholePart, fractionPart)
		}
		return fmt.Sprintf("$%s.00", wholePart)
	}

	return ""
}

func (e *PriceExtractor) parsePrice(priceText string) float64 {
	cleanPrice := strings.TrimSpace(priceText)
	re := regexp.MustCompile(`\d{1,3}(?:,\d{3})*(?:\.\d{2})?|\d+\.\d{2}|\d+`)
	matches := re.FindAllString(cleanPrice, -1)

	if len(matches) == 0 {
		return 0
	}

	priceStr := strings.ReplaceAll(matches[0], ",", "")
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		log.Printf("价格解析错误: %s -> %s", priceText, priceStr)
		return 0
	}

	log.Printf("价格解析成功: %s -> %s -> %.2f", priceText, priceStr, price)
	return price
}

func (e *PriceExtractor) extractCurrency(priceText string) string {
	// 检查货币符号
	if strings.Contains(priceText, "€") {
		return "EUR"
	} else if strings.Contains(priceText, "£") {
		return "GBP"
	} else if strings.Contains(priceText, "¥") {
		return "JPY"
	} else if strings.Contains(priceText, "C$") || strings.Contains(priceText, "CAD") {
		return "CAD"
	} else if strings.Contains(priceText, "A$") || strings.Contains(priceText, "AU$") {
		return "AUD"
	} else if strings.Contains(priceText, "$") {
		return "USD" // Default $ to USD
	}

	// 如果没有找到货币符号，返回USD作为默认值
	return "USD"
}

// extractListPrice 提取原价（list price）
func (e *PriceExtractor) extractListPrice(page playwright.Page, product *Product) {
	listPriceSelectors := []string{
		"span.a-size-small.aok-offscreen",
		".a-price.a-text-price .a-offscreen",
		"span.a-price.a-text-price .a-offscreen",
		"#priceblock_listprice",
		".a-text-strike .a-offscreen",
	}

	for _, selector := range listPriceSelectors {
		elements, err := page.QuerySelectorAll(selector)
		if err != nil {
			continue
		}

		for _, element := range elements {
			text, err := element.TextContent()
			if err != nil {
				continue
			}

			text = strings.TrimSpace(text)

			if strings.Contains(text, "List Price:") ||
				(strings.Contains(text, "$") && e.parsePrice(text) > 0) {

				if strings.Contains(text, "List Price:") {
					parts := strings.Split(text, "List Price:")
					if len(parts) > 1 {
						text = strings.TrimSpace(parts[1])
					}
				}

				listPrice := e.parsePrice(text)
				if listPrice > 0 && listPrice != product.FinalPrice {
					if listPrice > product.FinalPrice {
						product.PricesBreakdown.ListPrice = &listPrice
						log.Printf("提取到原价: %.2f (选择器: %s, 原文: %s)", listPrice, selector, text)
						return
					}
				}
			}
		}
	}

	log.Println("未找到有效的原价信息")
}

// isProductAvailable 检查产品是否可用
func (e *PriceExtractor) isProductAvailable(page playwright.Page) bool {
	availabilitySelectors := []string{
		"#availability span",
		"#availability .a-color-state",
		"#availability .a-color-price",
		"#availability .a-color-success",
		"#availability",
	}

	for _, selector := range availabilitySelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			text, err := element.TextContent()
			if err == nil && text != "" {
				lowerText := strings.ToLower(strings.TrimSpace(text))

				unavailableKeywords := []string{
					"currently unavailable",
					"unavailable",
					"out of stock",
					"temporarily out of stock",
					"not available",
					"discontinued",
				}

				for _, keyword := range unavailableKeywords {
					if strings.Contains(lowerText, keyword) {
						log.Printf("产品不可用: %s", text)
						return false
					}
				}

				availableKeywords := []string{
					"in stock",
					"available",
					"add to cart",
					"buy now",
				}

				for _, keyword := range availableKeywords {
					if strings.Contains(lowerText, keyword) {
						log.Printf("产品可用: %s", text)
						return true
					}
				}
			}
		}
	}

	// 检查是否有"Add to Cart"按钮
	addToCartSelectors := []string{
		"#add-to-cart-button",
		"#buy-now-button",
		"input[name='submit.add-to-cart']",
	}

	for _, selector := range addToCartSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			visible, _ := element.IsVisible()
			disabled, _ := element.IsDisabled()
			if visible && !disabled {
				log.Println("找到可用的购买按钮，产品可用")
				return true
			}
		}
	}

	log.Println("未找到明确的可用性信息，假设产品不可用")
	return false
}

// isUnavailableText 检查可用性文本是否表示不可用
func (e *PriceExtractor) isUnavailableText(availabilityText string) bool {
	lowerText := strings.ToLower(strings.TrimSpace(availabilityText))

	unavailableKeywords := []string{
		"currently unavailable",
		"unavailable",
		"out of stock",
		"temporarily out of stock",
		"not available",
		"discontinued",
		"sold out",
	}

	for _, keyword := range unavailableKeywords {
		if strings.Contains(lowerText, keyword) {
			return true
		}
	}

	return false
}
