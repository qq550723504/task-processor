// Package extractor 提供Amazon价格验证功能
package extractor

import (
	"strings"
	"task-processor/internal/core/logger"

	"github.com/mxschmitt/playwright-go"
	"github.com/sirupsen/logrus"
)

// PriceValidator 价格验证器
type PriceValidator struct {
	marketplace string
}

// NewPriceValidator 创建价格验证器
func NewPriceValidator(marketplace string) *PriceValidator {
	return &PriceValidator{
		marketplace: marketplace,
	}
}

// HasValidPrice 快速检查产品是否有有效价格
func (v *PriceValidator) HasValidPrice(page playwright.Page) bool {
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
				if v.containsCurrencySymbol(text) {
					logger.GetGlobalLogger("crawler/amazon").Infof("找到有效价格: %s", strings.TrimSpace(text))
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
					logger.GetGlobalLogger("crawler/amazon").Infof("产品不可用: %s", text)
					return false
				}
			}
		}
	}

	logger.GetGlobalLogger("crawler/amazon").Info("未找到有效价格")
	return false
}

// IsUnavailableText 检查可用性文本是否表示不可用
func (v *PriceValidator) IsUnavailableText(availabilityText string) bool {
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
			logger.GetGlobalLogger("crawler/amazon").WithFields(logrus.Fields{
				"availability": availabilityText,
				"keyword":      keyword,
			}).Info("❌ isUnavailableText 匹配到不可用关键词")
			return true
		}
	}

	logger.GetGlobalLogger("crawler/amazon").WithFields(logrus.Fields{
		"availability": availabilityText,
	}).Debug("✅ isUnavailableText 未匹配到不可用关键词")
	return false
}

// containsCurrencySymbol 检查文本是否包含货币符号或代码
func (v *PriceValidator) containsCurrencySymbol(text string) bool {
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
