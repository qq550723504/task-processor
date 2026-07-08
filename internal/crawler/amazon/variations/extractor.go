package variations

import (
	"strconv"
	"strings"
	"task-processor/internal/core/logger"

	"github.com/mxschmitt/playwright-go"
)

// Extractor 变体信息提取器
type Extractor struct {
	config     *Config
	parser     *Parser
	matcher    *Matcher
	combinator *Combinator
	mapper     *Mapper
}

// NewExtractor 创建变体信息提取器实例
func NewExtractor() *Extractor {
	config := GetDefaultConfig()
	return &Extractor{
		config:     config,
		parser:     NewParser(config),
		matcher:    NewMatcher(config),
		combinator: NewCombinator(config),
		mapper:     NewMapper(config),
	}
}

// NewExtractorWithConfig 使用自定义配置创建变体信息提取器实例
func NewExtractorWithConfig(config *Config) *Extractor {
	return &Extractor{
		config:     config,
		parser:     NewParser(config),
		matcher:    NewMatcher(config),
		combinator: NewCombinator(config),
		mapper:     NewMapper(config),
	}
}

// ExtractFromPage 从页面提取变体数据
func (e *Extractor) ExtractFromPage(page playwright.Page) (*VariationsData, error) {
	return e.parser.ParseVariationsData(page)
}

// BuildVariations 从变体数据构建变体列表
func (e *Extractor) BuildVariations(
	variationsValues []VariationValue,
	asinMapping map[string]map[string]string,
	priceMapping map[string]any,
	defaultPrice float64,
	defaultCurrency string,
) []Variation {
	var variations []Variation

	if e.config.EnableDebugLogging {
		logger.GetGlobalLogger("crawler/amazon").Infof("[DEBUG] Building variations from %d variation values", len(variationsValues))
	}

	// 获取所有变体维度
	dimensions := make(map[string][]string)
	for _, vv := range variationsValues {
		if vv.VariantName == "" {
			continue
		}
		if len(vv.Values) > 0 {
			dimensions[vv.VariantName] = vv.Values
		}
	}

	if len(dimensions) == 0 {
		return variations
	}

	// 生成所有可能的组合
	combinations := e.combinator.Generate(dimensions)
	if len(combinations) == 0 {
		return variations
	}

	if e.config.EnableDebugLogging {
		logger.GetGlobalLogger("crawler/amazon").Infof("[DEBUG] Generated %d combinations", len(combinations))
	}

	successCount := 0
	for _, combo := range combinations {
		// 查找匹配的 ASIN
		matchedASIN := e.matcher.FindMatchingASIN(combo, asinMapping)
		if matchedASIN == "" {
			continue
		}

		// 获取价格和货币信息
		_, currency := e.getPriceForASIN(matchedASIN, priceMapping, defaultPrice, defaultCurrency)

		// 映射属性名为语义化名称
		mappedAttributes := e.mapper.MapAttributeNames(combo)
		if len(mappedAttributes) == 0 {
			continue
		}

		variation := Variation{
			Name: e.buildNameFromAttributes(mappedAttributes),
			Asin: matchedASIN,
			//Price:      price,
			Currency:   currency,
			Attributes: mappedAttributes,
		}
		variations = append(variations, variation)
		successCount++
	}

	if e.config.EnableDebugLogging {
		logger.GetGlobalLogger("crawler/amazon").Infof("[DEBUG] Successfully created %d variations out of %d combinations", successCount, len(combinations))
	}

	return variations
}

// buildNameFromAttributes 从属性构建变体名称
func (e *Extractor) buildNameFromAttributes(attributes map[string]any) string {
	var parts []string

	// 先添加优先级高的属性
	for _, key := range e.config.AttributePriority {
		if value, exists := attributes[key]; exists {
			if str, ok := value.(string); ok && str != "" {
				parts = append(parts, str)
			}
		}
	}

	// 添加其他属性
	for key, value := range attributes {
		found := false
		for _, priorityKey := range e.config.AttributePriority {
			if key == priorityKey {
				found = true
				break
			}
		}
		if !found {
			if str, ok := value.(string); ok && str != "" {
				parts = append(parts, str)
			}
		}
	}

	if len(parts) == 0 {
		return "Default"
	}

	return strings.Join(parts, " ")
}

// getPriceForASIN 从价格映射中获取特定ASIN的价格信息
func (e *Extractor) getPriceForASIN(asin string, priceMapping map[string]any, defaultPrice float64, defaultCurrency string) (float64, string) {
	if priceMapping == nil || asin == "" {
		return defaultPrice, defaultCurrency
	}

	if priceData, exists := priceMapping[asin]; exists {
		if priceMap, ok := priceData.(map[string]any); ok {
			var price float64 = defaultPrice
			var currency string = defaultCurrency

			// 查找价格字段
			if priceValue, exists := priceMap["price"]; exists {
				if priceFloat, ok := priceValue.(float64); ok {
					price = priceFloat
				} else if priceStr, ok := priceValue.(string); ok {
					if parsedPrice, err := parsePrice(priceStr); err == nil {
						price = parsedPrice
					}
				}
			} else if displayPrice, exists := priceMap["displayPrice"]; exists {
				if priceStr, ok := displayPrice.(string); ok {
					if parsedPrice, err := parsePrice(priceStr); err == nil {
						price = parsedPrice
					}
				}
			}

			// 查找货币字段
			if currencyValue, exists := priceMap["currency"]; exists {
				if currencyStr, ok := currencyValue.(string); ok {
					currency = currencyStr
				}
			}

			return price, currency
		}
	}

	return defaultPrice, defaultCurrency
}

// parsePrice 解析价格字符串
func parsePrice(priceStr string) (float64, error) {
	// 移除货币符号和空格
	cleanPrice := strings.ReplaceAll(priceStr, "$", "")
	cleanPrice = strings.ReplaceAll(cleanPrice, "€", "")
	cleanPrice = strings.ReplaceAll(cleanPrice, "£", "")
	cleanPrice = strings.ReplaceAll(cleanPrice, "¥", "")
	cleanPrice = strings.ReplaceAll(cleanPrice, ",", "")
	cleanPrice = strings.TrimSpace(cleanPrice)

	return strconv.ParseFloat(cleanPrice, 64)
}

// BuildVariationName 为产品构建变体名称
func (e *Extractor) BuildVariationName(productDetails []ProductDetail) string {
	var parts []string

	for _, detail := range productDetails {
		detailTypeLower := strings.ToLower(detail.Type)

		for _, typeVariants := range e.config.AttributeTypeMapping {
			for _, typeVariant := range typeVariants {
				if detailTypeLower == strings.ToLower(typeVariant) {
					if detail.Value != "" {
						parts = append(parts, detail.Value)
					}
					goto nextDetail
				}
			}
		}
	nextDetail:
	}

	if len(parts) == 0 {
		return "Default"
	}

	return strings.Join(parts, " ")
}

// ExtractCurrentAttributes 从产品详情中提取当前属性
func (e *Extractor) ExtractCurrentAttributes(productDetails []ProductDetail) map[string]any {
	attributes := make(map[string]any)

	for _, detail := range productDetails {
		detailTypeLower := strings.ToLower(detail.Type)

		for attributeKey, typeVariants := range e.config.AttributeTypeMapping {
			for _, typeVariant := range typeVariants {
				if detailTypeLower == strings.ToLower(typeVariant) {
					if detail.Value != "" {
						attributes[attributeKey] = detail.Value
					}
					goto nextDetail
				}
			}
		}
	nextDetail:
	}

	return attributes
}
