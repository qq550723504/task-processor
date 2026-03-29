// Package extractor 提供Amazon价格解析功能
package extractor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"task-processor/internal/core/logger"

	"github.com/playwright-community/playwright-go"
)

// PriceParser 价格解析器
type PriceParser struct {
	marketplace string
}

// NewPriceParser 创建价格解析器
func NewPriceParser(marketplace string) *PriceParser {
	return &PriceParser{
		marketplace: marketplace,
	}
}

// ParsePrice 解析价格文本为数值
func (p *PriceParser) ParsePrice(priceText string) float64 {
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
		logger.GetGlobalLogger("crawler/amazon").Warnf("价格解析错误: %s -> %s (欧洲格式: %v)", priceText, priceStr, isEuropeanFormat)
		return 0
	}

	return price
}

// ExtractCombinedPrice 提取组合价格（整数部分+小数部分+货币符号）
func (p *PriceParser) ExtractCombinedPrice(page playwright.Page) string {
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
			currencySymbol = p.getDefaultCurrencySymbol()
		}

		// 根据站点决定小数分隔符
		decimalSeparator := p.getDecimalSeparator()

		if fractionPart != "" {
			return fmt.Sprintf("%s%s%s%s", currencySymbol, wholePart, decimalSeparator, fractionPart)
		}

		// 日本站通常不显示小数
		if p.marketplace == "JP" || p.marketplace == "co.jp" {
			return fmt.Sprintf("%s%s", currencySymbol, wholePart)
		}

		return fmt.Sprintf("%s%s%s00", currencySymbol, wholePart, decimalSeparator)
	}

	return ""
}

// getDefaultCurrencySymbol 根据站点返回默认货币符号
func (p *PriceParser) getDefaultCurrencySymbol() string {
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

	if symbol, ok := symbolMap[p.marketplace]; ok {
		return symbol
	}
	return "$"
}

// getDecimalSeparator 根据站点返回小数分隔符
func (p *PriceParser) getDecimalSeparator() string {
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

	if europeanMarkets[p.marketplace] {
		return ","
	}
	return "."
}
