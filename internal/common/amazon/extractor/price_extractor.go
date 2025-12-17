package extractor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"task-processor/internal/common/amazon/model"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// PriceExtractor 价格提取器
type PriceExtractor struct {
	// Marketplace 用于区分不同站点 (US, JP, UK, DE, FR, IT, ES, etc.)
	Marketplace string
}

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
				// 检查各种货币符号和代码
				if e.containsCurrencySymbol(text) {
					logrus.Infof("找到有效价格: %s", strings.TrimSpace(text))
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
					logrus.Infof("产品不可用: %s", text)
					return false
				}
			}
		}
	}

	logrus.Info("未找到有效价格")
	return false
}

func (e *PriceExtractor) Extract(page playwright.Page, product *model.Product) error {
	// 记录进入价格提取器时的状态
	logrus.WithFields(logrus.Fields{
		"asin":         product.Asin,
		"availability": product.Availability,
		"is_available": product.IsAvailable,
	}).Info("🔍 进入价格提取器")

	// 检查产品可用性（仅在明确不可用时才跳过价格提取）
	if product.Availability != "" && e.isUnavailableText(product.Availability) {
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

	// 注意：不再通过 isProductAvailable 覆盖 IsAvailable 字段
	// IsAvailable 应该由 AvailabilityExtractor 负责设置
	// 这里只负责提取价格信息
	logrus.WithFields(logrus.Fields{
		"asin":         product.Asin,
		"is_available": product.IsAvailable,
	}).Info("✅ 产品可用，继续提取价格")

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
		priceText = e.extractCombinedPrice(page)
		if priceText != "" {
			logrus.Infof("组合价格提取成功: %s", priceText)
		}
	}

	if priceText == "" {
		logrus.Warn("未找到价格信息，使用默认值")
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
		logrus.Infof("解析到价格: %.2f %s", price, product.Currency)
	} else {
		logrus.Warnf("价格解析失败: %s", priceText)
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

	symbolSelectors := []string{
		".a-price-symbol",
		".a-price .a-price-symbol",
		"span.a-price-symbol",
	}

	var wholePart, fractionPart, currencySymbol string

	// 提取货币符号
	for _, selector := range symbolSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			text, _ := element.TextContent()
			currencySymbol = strings.TrimSpace(text)
			if currencySymbol != "" {
				break
			}
		}
	}

	// 提取整数部分
	for _, selector := range wholeSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			text, _ := element.TextContent()
			wholePart = strings.TrimSpace(text)
			if wholePart != "" {
				break
			}
		}
	}

	// 提取小数部分
	for _, selector := range fractionSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			text, _ := element.TextContent()
			fractionPart = strings.TrimSpace(text)
			if fractionPart != "" {
				break
			}
		}
	}

	if wholePart != "" {
		// 移除可能的尾随点号或逗号
		wholePart = strings.TrimSuffix(wholePart, ".")
		wholePart = strings.TrimSuffix(wholePart, ",")

		// 如果没有找到货币符号，使用默认符号
		if currencySymbol == "" {
			currencySymbol = e.getDefaultCurrencySymbol()
		}

		// 根据站点决定小数分隔符
		decimalSeparator := e.getDecimalSeparator()

		if fractionPart != "" {
			return fmt.Sprintf("%s%s%s%s", currencySymbol, wholePart, decimalSeparator, fractionPart)
		}

		// 日本站通常不显示小数
		if e.Marketplace == "JP" || e.Marketplace == "co.jp" {
			return fmt.Sprintf("%s%s", currencySymbol, wholePart)
		}

		return fmt.Sprintf("%s%s%s00", currencySymbol, wholePart, decimalSeparator)
	}

	return ""
}

// getDefaultCurrencySymbol 根据站点返回默认货币符号
func (e *PriceExtractor) getDefaultCurrencySymbol() string {
	symbolMap := map[string]string{
		"US": "$", "com": "$",
		"UK": "£", "co.uk": "£",
		"DE": "€", "de": "€",
		"FR": "€", "fr": "€",
		"IT": "€", "it": "€",
		"ES": "€", "es": "€",
		"JP": "¥", "co.jp": "¥",
		"CA": "C$", "ca": "C$",
		"AU": "A$", "com.au": "A$",
		"IN": "₹", "in": "₹",
		"CN": "¥", "cn": "¥",
		"MX": "$", "com.mx": "$",
		"BR": "R$", "com.br": "R$",
		"SG": "S$", "sg": "S$",
		"SA": "SAR", "sa": "SAR",
		"AE": "AED", "ae": "AED",
	}

	if symbol, ok := symbolMap[e.Marketplace]; ok {
		return symbol
	}
	return "$"
}

// getDecimalSeparator 根据站点返回小数分隔符
func (e *PriceExtractor) getDecimalSeparator() string {
	// 欧洲大部分国家使用逗号作为小数分隔符
	europeanMarkets := map[string]bool{
		"DE": true, "de": true,
		"FR": true, "fr": true,
		"IT": true, "it": true,
		"ES": true, "es": true,
		"PL": true, "pl": true,
		"SE": true, "se": true,
		"BR": true, "com.br": true,
	}

	if europeanMarkets[e.Marketplace] {
		return ","
	}
	return "."
}

func (e *PriceExtractor) parsePrice(priceText string) float64 {
	cleanPrice := strings.TrimSpace(priceText)

	// 检测欧洲格式 (1.234,56) vs 美国格式 (1,234.56)
	isEuropeanFormat := false
	if strings.Count(cleanPrice, ",") == 1 && strings.Count(cleanPrice, ".") >= 1 {
		// 如果逗号在点号后面，可能是欧洲格式
		commaPos := strings.LastIndex(cleanPrice, ",")
		dotPos := strings.LastIndex(cleanPrice, ".")
		if commaPos > dotPos {
			isEuropeanFormat = true
		}
	} else if strings.Count(cleanPrice, ",") == 1 && strings.Count(cleanPrice, ".") == 0 {
		// 只有逗号，检查逗号后是否是2位数字（小数）
		parts := strings.Split(cleanPrice, ",")
		if len(parts) == 2 && len(parts[1]) == 2 {
			isEuropeanFormat = true
		}
	}

	// 提取数字
	var re *regexp.Regexp
	if isEuropeanFormat {
		// 欧洲格式：1.234,56 -> 移除点号，逗号替换为点号
		re = regexp.MustCompile(`\d{1,3}(?:\.\d{3})*(?:,\d{2})?|\d+,\d{2}|\d+`)
	} else {
		// 美国格式：1,234.56 -> 移除逗号
		re = regexp.MustCompile(`\d{1,3}(?:,\d{3})*(?:\.\d{2})?|\d+\.\d{2}|\d+`)
	}

	matches := re.FindAllString(cleanPrice, -1)
	if len(matches) == 0 {
		return 0
	}

	priceStr := matches[0]
	if isEuropeanFormat {
		// 欧洲格式转换：移除点号，逗号改为点号
		priceStr = strings.ReplaceAll(priceStr, ".", "")
		priceStr = strings.ReplaceAll(priceStr, ",", ".")
	} else {
		// 美国格式：只移除逗号
		priceStr = strings.ReplaceAll(priceStr, ",", "")
	}

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		logrus.Warnf("价格解析错误: %s -> %s (欧洲格式: %v)", priceText, priceStr, isEuropeanFormat)
		return 0
	}

	return price
}

func (e *PriceExtractor) extractCurrency(priceText string) string {
	// 优先检查明确的货币代码
	currencyCodes := map[string]string{
		"USD": "USD", "CAD": "CAD", "AUD": "AUD",
		"EUR": "EUR", "GBP": "GBP", "JPY": "JPY",
		"CNY": "CNY", "SGD": "SGD", "INR": "INR",
		"MXN": "MXN", "BRL": "BRL", "SEK": "SEK",
		"PLN": "PLN", "TRY": "TRY", "AED": "AED",
		"SAR": "SAR",
	}

	upperText := strings.ToUpper(priceText)
	for code, currency := range currencyCodes {
		if strings.Contains(upperText, code) {
			return currency
		}
	}

	// 检查货币符号
	if strings.Contains(priceText, "€") {
		return "EUR"
	} else if strings.Contains(priceText, "£") {
		return "GBP"
	} else if strings.Contains(priceText, "¥") {
		// 根据站点区分日元和人民币
		switch e.Marketplace {
		case "JP", "co.jp":
			return "JPY"
		case "CN", "cn":
			return "CNY"
		}
		// 默认返回JPY（因为Amazon主要是日本站用¥）
		return "JPY"
	} else if strings.Contains(priceText, "C$") || strings.Contains(priceText, "CA$") {
		return "CAD"
	} else if strings.Contains(priceText, "A$") || strings.Contains(priceText, "AU$") {
		return "AUD"
	} else if strings.Contains(priceText, "S$") {
		return "SGD"
	} else if strings.Contains(priceText, "₹") {
		return "INR"
	} else if strings.Contains(priceText, "kr") {
		return "SEK"
	} else if strings.Contains(priceText, "zł") {
		return "PLN"
	} else if strings.Contains(priceText, "$") {
		// 根据站点区分不同的美元
		switch e.Marketplace {
		case "CA", "ca":
			return "CAD"
		case "AU", "com.au":
			return "AUD"
		case "SG", "sg":
			return "SGD"
		case "MX", "com.mx":
			return "MXN"
		case "BR", "com.br":
			return "BRL"
		default:
			return "USD"
		}
	}

	// 根据站点返回默认货币
	return e.getDefaultCurrencyByMarketplace()
}

// getDefaultCurrencyByMarketplace 根据站点返回默认货币
func (e *PriceExtractor) getDefaultCurrencyByMarketplace() string {
	marketplaceCurrency := map[string]string{
		"US": "USD", "com": "USD",
		"UK": "GBP", "co.uk": "GBP",
		"DE": "EUR", "de": "EUR",
		"FR": "EUR", "fr": "EUR",
		"IT": "EUR", "it": "EUR",
		"ES": "EUR", "es": "EUR",
		"JP": "JPY", "co.jp": "JPY",
		"CA": "CAD", "ca": "CAD",
		"AU": "AUD", "com.au": "AUD",
		"IN": "INR", "in": "INR",
		"CN": "CNY", "cn": "CNY",
		"MX": "MXN", "com.mx": "MXN",
		"BR": "BRL", "com.br": "BRL",
		"SG": "SGD", "sg": "SGD",
		"AE": "AED", "ae": "AED",
		"SE": "SEK", "se": "SEK",
		"PL": "PLN", "pl": "PLN",
		"TR": "TRY", "com.tr": "TRY",
	}

	if currency, ok := marketplaceCurrency[e.Marketplace]; ok {
		return currency
	}

	return "USD" // 默认返回USD
}

// extractListPrice 提取原价（list price）
func (e *PriceExtractor) extractListPrice(page playwright.Page, product *model.Product) {
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
						return
					}
				}
			}
		}
	}

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
		// 日语
		"在庫切れ", "取り扱い終了", "現在お取り扱いできません",
		// 德语
		"derzeit nicht verfügbar", "nicht auf lager",
		// 法语
		"actuellement indisponible", "en rupture de stock",
		// 西班牙语
		"actualmente no disponible", "agotado",
		// 意大利语
		"attualmente non disponibile", "esaurito",
	}

	for _, keyword := range unavailableKeywords {
		if strings.Contains(lowerText, keyword) {
			logrus.WithFields(logrus.Fields{
				"availability": availabilityText,
				"keyword":      keyword,
			}).Info("❌ isUnavailableText 匹配到不可用关键词")
			return true
		}
	}

	logrus.WithFields(logrus.Fields{
		"availability": availabilityText,
	}).Debug("✅ isUnavailableText 未匹配到不可用关键词")
	return false
}

// containsCurrencySymbol 检查文本是否包含货币符号或代码
func (e *PriceExtractor) containsCurrencySymbol(text string) bool {
	// 货币符号
	currencySymbols := []string{
		"$", "£", "€", "¥", "₹", "₽", "₩", "₪", "₱", "₫", "₴", "₦", "₡", "₨",
		"kr", "zł", "Kč", "Ft", "lei", "kn", "din", "ден", "лв",
	}

	for _, symbol := range currencySymbols {
		if strings.Contains(text, symbol) {
			return true
		}
	}

	// 货币代码
	currencyCodes := []string{
		"USD", "CAD", "AUD", "EUR", "GBP", "JPY", "CNY", "SGD", "INR",
		"MXN", "BRL", "SEK", "PLN", "TRY", "AED", "SAR", "EGP", "ZAR",
		"RUB", "KRW", "THB", "IDR", "MYR", "PHP", "VND", "NZD", "CHF",
	}

	upperText := strings.ToUpper(text)
	for _, code := range currencyCodes {
		if strings.Contains(upperText, code) {
			return true
		}
	}

	return false
}
