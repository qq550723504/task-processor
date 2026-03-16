// Package extractor 提供Amazon货币管理功能
package extractor

import "strings"

// CurrencyManager 货币管理器
type CurrencyManager struct {
	marketplace string
}

// NewCurrencyManager 创建货币管理器
func NewCurrencyManager(marketplace string) *CurrencyManager {
	return &CurrencyManager{
		marketplace: marketplace,
	}
}

// ExtractCurrency 从价格文本中提取货币代码
func (c *CurrencyManager) ExtractCurrency(priceText string) string {
	// 优先检查明确的货币代码
	currencyCodes := map[string]string{
		"USD": "USD", "CAD": "CAD", "AUD": "AUD",
		"EUR": "EUR", "GBP": "GBP", "JPY": "JPY",
		"CNY": "CNY", "SGD": "SGD", "INR": "INR",
		"MXN": "MXN", "BRL": "BRL", "SEK": "SEK",
		"PLN": "PLN", "TRY": "TRY", "AED": "AED",
		"SAR": "SAR", "HKD": "HKD",
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
	} else if strings.Contains(priceText, "HK$") {
		return "HKD"
	} else if strings.Contains(priceText, "¥") {
		// 根据站点区分日元和人民币
		switch c.marketplace {
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
		switch c.marketplace {
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
	return c.GetDefaultCurrencyByMarketplace()
}

// GetDefaultCurrencyByMarketplace 根据站点返回默认货币
func (c *CurrencyManager) GetDefaultCurrencyByMarketplace() string {
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

	if currency, ok := marketplaceCurrency[c.marketplace]; ok {
		return currency
	}

	return "USD" // 默认返回USD
}
