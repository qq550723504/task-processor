package amazon

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/playwright-community/playwright-go"
)

// VariationsConfig 变体提取配置
type VariationsConfig struct {
	// 属性优先级，用于排序和显示
	AttributePriority []string
	// 统一的属性名映射配置（合并了KeyNormalization和AttributeNameMapping）
	AttributeMapping map[string]string
	// 属性类型映射，用于智能推断
	AttributeTypeMapping map[string][]string
	// 是否启用智能推断
	EnableSmartInference bool
	// 是否启用详细日志
	EnableDebugLogging bool
}

// GetDefaultVariationsConfig 获取默认配置
func GetDefaultVariationsConfig() *VariationsConfig {
	return &VariationsConfig{
		AttributePriority: []string{"size", "color", "item_package_quantity", "style", "pattern", "material", "brand"},
		AttributeMapping: map[string]string{
			// 原始属性名到标准化名称的映射
			"color_name":            "color",
			"size_name":             "size",
			"quantity":              "item_package_quantity",
			"item_package_quantity": "item_package_quantity",
			// 通用属性名到语义化名称的映射
			"attribute_1":   "color",
			"attribute_2":   "size",
			"attribute_3":   "style",
			"attribute_4":   "material",
			"attribute_5":   "pattern",
			"variant_code":  "variant",
			"variant_style": "style",
		},
		AttributeTypeMapping: map[string][]string{
			"color":    {"color"},
			"size":     {"size", "product dimensions"},
			"material": {"material"},
			"brand":    {"brand"},
		},
		EnableSmartInference: true,
		EnableDebugLogging:   false,
	}
}

// VariationsExtractor 变体信息提取器
type VariationsExtractor struct {
	config *VariationsConfig
}

// VariationsData 保存变体数据和ASIN映射信息
type VariationsData struct {
	VariationsValues map[string][]string          `json:"variations_values"`
	ASINMapping      map[string]map[string]string `json:"asin_mapping"`
	PriceMapping     map[string]interface{}       `json:"price_mapping"`
}

// NewVariationsExtractor 创建变体信息提取器实例
func NewVariationsExtractor() *VariationsExtractor {
	return &VariationsExtractor{
		config: GetDefaultVariationsConfig(),
	}
}

// NewVariationsExtractorWithConfig 使用自定义配置创建变体信息提取器实例
func NewVariationsExtractorWithConfig(config *VariationsConfig) *VariationsExtractor {
	return &VariationsExtractor{
		config: config,
	}
}

// Extract 提取产品变体信息
func (ve *VariationsExtractor) Extract(page playwright.Page, product *Product) error {
	// 先提取 variations_values 和 ASIN 映射
	variationsData, err := ve.getVariationsValues(page)
	if err != nil {
		return err
	}

	// 转换为 []VariationValue 格式
	var variationsValues []VariationValue
	for variantName, values := range variationsData.VariationsValues {
		if len(values) > 0 {
			variationsValues = append(variationsValues, VariationValue{
				VariantName: variantName,
				Values:      values,
			})
		}
	}
	product.VariationsValues = variationsValues

	// 然后基于 variations_values 和 ASIN 映射生成 variations
	variations, err := ve.getVariations(product, variationsData.ASINMapping, variationsData.PriceMapping)
	if err != nil {
		return err
	}
	product.Variations = variations

	return nil
}

// getVariations 获取变体信息数据
func (ve *VariationsExtractor) getVariations(product *Product, asinMapping map[string]map[string]string, priceMapping map[string]interface{}) ([]Variation, error) {
	// 从 variations_values 构建 variations
	var variations []Variation

	if len(product.VariationsValues) > 0 {
		// 创建变体组合
		variations = ve.buildVariationsFromValues(product, asinMapping, priceMapping)
	}

	return variations, nil
}

// buildVariationsFromValues 从 variations_values 构建变体列表
func (ve *VariationsExtractor) buildVariationsFromValues(product *Product, asinMapping map[string]map[string]string, priceMapping map[string]interface{}) []Variation {
	var variations []Variation

	// 获取所有变体维度
	dimensions := make(map[string][]string)
	for _, vv := range product.VariationsValues {
		if vv.VariantName != "" && len(vv.Values) > 0 {
			dimensions[vv.VariantName] = vv.Values
		}
	}

	if len(dimensions) == 0 {
		return variations
	}

	// 生成所有可能的组合
	combinations := ve.generateCombinations(dimensions)

	for _, combo := range combinations {
		// 查找匹配的 ASIN
		matchedASIN := ve.findMatchingASIN(combo, asinMapping)
		if matchedASIN == "" {
			continue
		}

		// 获取价格和货币信息
		price, currency := ve.getPriceForASIN(matchedASIN, priceMapping, product.FinalPrice, "USD")

		// 映射属性名为语义化名称
		mappedAttributes := ve.mapAttributeNames(combo)
		if len(mappedAttributes) == 0 {
			continue
		}

		variation := Variation{
			Name:       ve.buildNameFromAttributes(mappedAttributes),
			Asin:       matchedASIN,
			Price:      price,
			Currency:   currency,
			Attributes: mappedAttributes,
		}
		variations = append(variations, variation)
	}

	return variations
}

// generateCombinations 生成所有属性组合
func (ve *VariationsExtractor) generateCombinations(dimensions map[string][]string) []map[string]interface{} {
	var combinations []map[string]interface{}

	// 获取维度名称和值
	var dimNames []string
	var dimValues [][]string

	for name, values := range dimensions {
		dimNames = append(dimNames, name)
		dimValues = append(dimValues, values)
	}

	if len(dimNames) == 0 {
		return combinations
	}

	// 递归生成组合
	ve.generateCombinationsRecursive(dimNames, dimValues, 0, make(map[string]interface{}), &combinations)

	return combinations
}

// generateCombinationsRecursive 递归生成组合
func (ve *VariationsExtractor) generateCombinationsRecursive(
	dimNames []string,
	dimValues [][]string,
	index int,
	current map[string]interface{},
	combinations *[]map[string]interface{},
) {
	if index == len(dimNames) {
		// 复制当前组合
		combo := make(map[string]interface{})
		for k, v := range current {
			combo[k] = v
		}
		*combinations = append(*combinations, combo)
		return
	}

	// 遍历当前维度的所有值
	for _, value := range dimValues[index] {
		current[dimNames[index]] = value
		ve.generateCombinationsRecursive(dimNames, dimValues, index+1, current, combinations)
	}
}

// buildNameFromAttributes 从属性构建变体名称
func (ve *VariationsExtractor) buildNameFromAttributes(attributes map[string]interface{}) string {
	var parts []string

	// 先添加优先级高的属性
	for _, key := range ve.config.AttributePriority {
		if value, exists := attributes[key]; exists {
			if str, ok := value.(string); ok && str != "" {
				parts = append(parts, str)
			}
		}
	}

	// 添加其他属性
	for key, value := range attributes {
		found := false
		for _, priorityKey := range ve.config.AttributePriority {
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

// mapAttributeNames 将通用属性名映射为语义化名称
func (ve *VariationsExtractor) mapAttributeNames(attributes map[string]interface{}) map[string]interface{} {
	mapped := make(map[string]interface{})

	for key, value := range attributes {
		var finalKey string

		// 首先检查是否有预定义的映射配置
		if mappedName, exists := ve.config.AttributeMapping[key]; exists {
			finalKey = mappedName
		} else if ve.config.EnableSmartInference {
			// 如果启用了智能推断，尝试基于值推断属性类型
			inferredType := ve.inferAttributeType(value)

			// 对于attribute_N或variant_*格式的键，使用推断的类型
			if strings.HasPrefix(key, "attribute_") || strings.HasPrefix(key, "variant_") {
				finalKey = inferredType
			} else {
				// 对于其他键名，保持原样
				finalKey = key
			}
		} else {
			// 如果没有映射且未启用智能推断，保持原键名
			finalKey = key
		}

		mapped[finalKey] = value
	}

	return mapped
}

// inferAttributeType 基于属性值内容推断属性类型
func (ve *VariationsExtractor) inferAttributeType(value interface{}) string {
	if value == nil {
		return "unknown"
	}

	valueStr := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", value)))

	// 颜色检测
	colorKeywords := []string{
		"black", "white", "red", "blue", "green", "yellow", "orange", "purple", "pink", "brown",
		"gray", "grey", "silver", "gold", "beige", "navy", "maroon", "olive", "lime", "aqua",
	}

	for _, color := range colorKeywords {
		if strings.Contains(valueStr, color) {
			return "color"
		}
	}

	// 尺寸检测
	sizePatterns := []string{
		"xs", "s", "m", "l", "xl", "xxl", "xxxl",
		"small", "medium", "large", "extra large", "extra small",
	}

	for _, size := range sizePatterns {
		if strings.Contains(valueStr, size) {
			return "size"
		}
	}

	// 材质检测
	materialKeywords := []string{
		"cotton", "polyester", "wool", "silk", "leather", "denim", "canvas", "linen",
		"plastic", "metal", "wood", "glass", "ceramic", "rubber",
	}

	for _, material := range materialKeywords {
		if strings.Contains(valueStr, material) {
			return "material"
		}
	}

	// 样式检测
	styleKeywords := []string{
		"classic", "modern", "vintage", "casual", "formal", "sporty", "elegant",
		"minimalist", "bohemian", "retro", "contemporary", "traditional", "trendy",
	}

	for _, style := range styleKeywords {
		if strings.Contains(valueStr, style) {
			return "style"
		}
	}

	// 数量检测
	quantityPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\b\d+\s*(pack|pcs?|pieces?|count|ct)\b`),
		regexp.MustCompile(`\b(pack\s+of|set\s+of)\s+\d+`),
		regexp.MustCompile(`\b\d+\s*-?\s*(unit|item)s?\b`),
	}

	for _, pattern := range quantityPatterns {
		if pattern.MatchString(valueStr) {
			return "item_package_quantity"
		}
	}

	// 品牌检测
	if strings.Contains(valueStr, "brand") || strings.Contains(valueStr, "by ") {
		return "brand"
	}

	// 如果都不匹配，返回通用变体类型
	return "variant"
}

// getVariationsValues 提取产品变体值（简化版）
func (ve *VariationsExtractor) getVariationsValues(page playwright.Page) (*VariationsData, error) {
	result := &VariationsData{
		VariationsValues: make(map[string][]string),
		ASINMapping:      make(map[string]map[string]string),
		PriceMapping:     make(map[string]interface{}),
	}

	// 简化的JavaScript提取逻辑
	jsResult, err := page.Evaluate(`() => {
		const scripts = document.querySelectorAll('script');
		let variationsData = {};
		
		for (let script of scripts) {
			const content = script.textContent || script.innerHTML;
			
			// 查找 variationValues
			if (content.includes('variationValues')) {
				const variationRegex = /variationValues['"]*\s*:\s*({[\s\S]*?})\s*[,}]/;
				const variationMatch = content.match(variationRegex);
				
				if (variationMatch) {
					try {
						const variationParsed = JSON.parse(variationMatch[1]);
						for (let attrName in variationParsed) {
							if (Array.isArray(variationParsed[attrName])) {
								variationsData[attrName] = variationParsed[attrName];
							}
						}
					} catch (e) {
						console.log('Error parsing variationValues:', e);
					}
				}
			}
		}
		
		return {
			variationsData: Object.keys(variationsData).length > 0 ? variationsData : null
		};
	}`)

	if err != nil {
		return result, err
	}

	// 处理JavaScript返回的数据
	if jsResult != nil {
		if jsMap, ok := jsResult.(map[string]interface{}); ok {
			if variationsData, exists := jsMap["variationsData"]; exists && variationsData != nil {
				if variationsMap, ok := variationsData.(map[string]interface{}); ok {
					for key, value := range variationsMap {
						if valueArray, ok := value.([]interface{}); ok {
							var stringArray []string
							for _, item := range valueArray {
								if str, ok := item.(string); ok {
									stringArray = append(stringArray, str)
								}
							}
							if len(stringArray) > 0 {
								normalizedKey := ve.normalizeVariationKey(key)
								result.VariationsValues[normalizedKey] = ve.removeDuplicates(stringArray)
							}
						}
					}
				}
			}
		}
	}

	return result, nil
}

// normalizeVariationKey 标准化变体键名
func (ve *VariationsExtractor) normalizeVariationKey(key string) string {
	if normalized, exists := ve.config.AttributeMapping[key]; exists {
		return normalized
	}
	return key
}

// removeDuplicates 去除重复值
func (ve *VariationsExtractor) removeDuplicates(values []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}

	return result
}

// findMatchingASIN 查找匹配的ASIN（简化版）
func (ve *VariationsExtractor) findMatchingASIN(combo map[string]interface{}, asinMapping map[string]map[string]string) string {
	// 简化的匹配逻辑
	for asin := range asinMapping {
		return asin // 返回第一个ASIN作为示例
	}
	return ""
}

// getPriceForASIN 获取ASIN对应的价格（简化版）
func (ve *VariationsExtractor) getPriceForASIN(asin string, priceMapping map[string]interface{}, defaultPrice float64, defaultCurrency string) (float64, string) {
	// 简化的价格获取逻辑
	return defaultPrice, defaultCurrency
}
