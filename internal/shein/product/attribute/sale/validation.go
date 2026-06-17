package sale

import (
	"fmt"
	"strings"

	"task-processor/internal/core/logger"
	"task-processor/internal/model"
	"task-processor/internal/pkg/types"
	sheinattr "task-processor/internal/shein/product/attribute"
	sheinsku "task-processor/internal/shein/product/sku"
)

// validateAndFixSaleAttributeData 验证并修复销售属性数据
func (h *SaleAttributeHandler) validateAndFixSaleAttributeData(data sheinattr.ResultSaleAttribute, productsData []map[string]string) sheinattr.ResultSaleAttribute {
	logger.GetGlobalLogger("shein/product").Debug("开始验证和修复AI生成的销售属性数据")

	// 1. 修复属性值ID重复问题
	data = h.fixAttributeValueIDsWithManager(data)

	// 2. 标准化尺寸单位
	data = h.standardizeDimensionUnits(data)

	// 3. 验证每个ASIN都有对应的变体
	data = h.validateVariantCompleteness(data, productsData)

	// 4. 验证变体属性完整性（关键修复）
	data = h.validateVariantAttributes(data, productsData)

	// 5. 修复缺失或不可解析的变体重量，避免后续被兜底为 0.01g
	data = h.repairVariantWeights(data, productsData)

	logger.GetGlobalLogger("shein/product").Debug("销售属性数据验证和修复完成")
	return data
}

// fixAttributeValueIDsWithManager 标记需要映射的属性值ID
func (h *SaleAttributeHandler) fixAttributeValueIDsWithManager(data sheinattr.ResultSaleAttribute) sheinattr.ResultSaleAttribute {
	logger.GetGlobalLogger("shein/product").Debug("标记属性值ID为需要映射状态")

	// 注意：ResultSaleAttribute.SaleAttributes 是 []ResultAttribute 类型
	for i := range data.SaleAttributes {
		saleAttr := &data.SaleAttributes[i]
		logger.GetGlobalLogger("shein/product").Debugf("处理属性ID %d，属性值数量: %d", saleAttr.AttrID, len(saleAttr.AttrValue))

		for j := range saleAttr.AttrValue {
			attrValue := &saleAttr.AttrValue[j]
			currentID := attrValue.ID.Int()

			// 不再分配简单的递增ID，而是标记为需要映射
			if currentID <= 0 {
				// 使用-1标记需要映射到SHEIN平台ID的属性值
				attrValue.ID = types.FlexibleID(-1)
				logger.GetGlobalLogger("shein/product").Debugf("标记属性值需要映射: %s (原ID: %d -> 标记ID: -1)", attrValue.Value, currentID)
			} else {
				// 如果已经有有效ID，保持不变
				logger.GetGlobalLogger("shein/product").Debugf("保持有效属性值ID: %s (ID: %d)", attrValue.Value, currentID)
			}
		}
	}

	logger.GetGlobalLogger("shein/product").Debug("属性值ID标记完成，后续将通过mapAttributeValuesToSheinIDs进行真正的ID映射")
	return data
}

// standardizeDimensionUnits 标准化尺寸单位
func (h *SaleAttributeHandler) standardizeDimensionUnits(data sheinattr.ResultSaleAttribute) sheinattr.ResultSaleAttribute {
	// 定义单位映射表
	// 注意：SHEIN平台只接受 cm 作为长宽高单位，所有其他单位都需要转换为 cm
	unitMappings := map[string]string{
		// 英寸相关（转换为厘米）
		"inch":   "cm",
		"inches": "cm",
		"in":     "cm",
		"\"":     "cm",

		// 厘米相关
		"cm":          "cm",
		"centimeter":  "cm",
		"centimeters": "cm",
		"centimetre":  "cm",
		"centimetres": "cm",

		// 英尺相关（转换为厘米）
		"ft":   "cm",
		"foot": "cm",
		"feet": "cm",
		"'":    "cm",

		// 毫米相关（转换为厘米）
		"mm":          "cm",
		"millimeter":  "cm",
		"millimeters": "cm",
		"millimetre":  "cm",
		"millimetres": "cm",

		// 米相关（转换为厘米）
		"m":      "cm",
		"meter":  "cm",
		"meters": "cm",
		"metre":  "cm",
		"metres": "cm",
	}

	fixedCount := 0

	for i, variant := range data.Variants {
		originalUnit := variant.LengthUnit
		if originalUnit == "" {
			// 如果没有单位，默认使用cm
			data.Variants[i].LengthUnit = "cm"
			logger.GetGlobalLogger("shein/product").Debugf("ASIN %s: 尺寸单位为空，设置为默认单位 cm", variant.ASIN)
			fixedCount++
			continue
		}

		// 标准化单位（转换为小写进行匹配）
		normalizedUnit := strings.ToLower(strings.TrimSpace(originalUnit))

		if standardUnit, exists := unitMappings[normalizedUnit]; exists {
			if standardUnit != originalUnit {
				data.Variants[i].LengthUnit = standardUnit
				logger.GetGlobalLogger("shein/product").Debugf("ASIN %s: 尺寸单位从 '%s' 标准化为 '%s'", variant.ASIN, originalUnit, standardUnit)
				fixedCount++

				// 根据原始单位进行数值转换
				switch normalizedUnit {
				case "inch", "inches", "in", "\"":
					convertInchesToCentimeters(&data.Variants[i])
				case "ft", "foot", "feet", "'":
					convertFeetToCentimeters(&data.Variants[i])
				case "mm", "millimeter", "millimeters", "millimetre", "millimetres":
					convertMillimetersToCentimeters(&data.Variants[i])
				case "m", "meter", "meters", "metre", "metres":
					convertMetersToCentimeters(&data.Variants[i])
				}
			}
		} else {
			// 未知单位，记录警告并设置为默认单位
			logger.GetGlobalLogger("shein/product").Warnf("ASIN %s: 发现未知尺寸单位 '%s'，设置为默认单位 cm", variant.ASIN, originalUnit)
			data.Variants[i].LengthUnit = "cm"
			fixedCount++
		}
	}

	if fixedCount > 0 {
		logger.GetGlobalLogger("shein/product").Debugf("共修复了 %d 个变体的尺寸单位", fixedCount)
	}

	return data
}

// validateVariantCompleteness 验证每个ASIN都有对应的变体
func (h *SaleAttributeHandler) validateVariantCompleteness(data sheinattr.ResultSaleAttribute, products []map[string]string) sheinattr.ResultSaleAttribute {
	productASINs := make(map[string]bool)
	for _, product := range products {
		productASINs[product["asin"]] = true
	}

	variantASINs := make(map[string]bool)
	for _, variant := range data.Variants {
		variantASINs[variant.ASIN] = true
	}

	var missingASINs []string
	for asin := range productASINs {
		if !variantASINs[asin] {
			missingASINs = append(missingASINs, asin)
		}
	}

	if len(missingASINs) > 0 {
		logger.GetGlobalLogger("shein/product").Warnf("发现缺失的ASIN变体: %v", missingASINs)
	}

	return data
}

// validateVariantAttributes 验证变体属性完整性（关键修复）
func (h *SaleAttributeHandler) validateVariantAttributes(data sheinattr.ResultSaleAttribute, products []map[string]string) sheinattr.ResultSaleAttribute {
	logger.GetGlobalLogger("shein/product").Debug("🔍 开始验证变体属性完整性...")

	// 构建产品数据映射：ASIN -> 产品属性
	productAttributesMap := make(map[string]map[string]string)
	for _, product := range products {
		asin := product["asin"]
		attributes := make(map[string]string)
		for key, value := range product {
			// 排除基本字段，只保留属性字段
			if key != "asin" && key != "title" && key != "price" && key != "currency" && key != "productdimensions" {
				attributes[key] = value
			}
		}
		if len(attributes) > 0 {
			productAttributesMap[asin] = attributes
			logger.GetGlobalLogger("shein/product").Debugf("产品 %s 的属性: %v", asin, attributes)
		}
	}

	// 验证并修复每个变体的属性
	emptyAttributesCount := 0
	fixedCount := 0

	for i, variant := range data.Variants {
		// 检查变体的Attributes字段是否为空
		if len(variant.Attributes) == 0 {
			emptyAttributesCount++
			logger.GetGlobalLogger("shein/product").Warnf("⚠️ 变体 %s 的Attributes字段为空", variant.ASIN)

			// 尝试从产品数据中恢复属性
			if productAttrs, exists := productAttributesMap[variant.ASIN]; exists && len(productAttrs) > 0 {
				data.Variants[i].Attributes = productAttrs
				fixedCount++
				logger.GetGlobalLogger("shein/product").Debugf("✅ 已从产品数据恢复变体 %s 的属性: %v", variant.ASIN, productAttrs)
			} else {
				logger.GetGlobalLogger("shein/product").Errorf("❌ 无法恢复变体 %s 的属性，产品数据中也没有属性信息", variant.ASIN)
			}
		} else {
			logger.GetGlobalLogger("shein/product").Debugf("✅ 变体 %s 的Attributes正常: %v", variant.ASIN, variant.Attributes)
		}
	}

	if emptyAttributesCount > 0 {
		logger.GetGlobalLogger("shein/product").Warnf("⚠️ 发现 %d 个变体的Attributes为空，已修复 %d 个", emptyAttributesCount, fixedCount)
		if fixedCount < emptyAttributesCount {
			logger.GetGlobalLogger("shein/product").Errorf("❌ 仍有 %d 个变体的Attributes无法修复，这将导致后续匹配失败", emptyAttributesCount-fixedCount)
		}
	} else {
		logger.GetGlobalLogger("shein/product").Debug("✅ 所有变体的Attributes字段都正常")
	}

	return data
}

func (h *SaleAttributeHandler) repairVariantWeights(data sheinattr.ResultSaleAttribute, products []map[string]string) sheinattr.ResultSaleAttribute {
	logger.GetGlobalLogger("shein/product").Debug("🔧 开始修复变体重量")

	weightParser := sheinsku.NewSKUUtils()
	sourceWeights := make(map[string]string, len(products))
	fallbackWeight := ""

	for _, product := range products {
		asin := strings.TrimSpace(product["asin"])
		weight := strings.TrimSpace(product["weight"])
		if asin != "" && weight != "" {
			sourceWeights[asin] = weight
		}
		if fallbackWeight == "" && weightParser.ParseWeight(weight) > 0 {
			fallbackWeight = formatVariantWeightInGrams(weightParser.ParseWeight(weight))
		}
	}

	fixedCount := 0
	for i := range data.Variants {
		currentWeight := strings.TrimSpace(data.Variants[i].Weight.String())
		if weightParser.ParseWeight(currentWeight) > 0 {
			continue
		}

		repairedWeight := ""
		if sourceWeight := strings.TrimSpace(sourceWeights[data.Variants[i].ASIN]); weightParser.ParseWeight(sourceWeight) > 0 {
			repairedWeight = formatVariantWeightInGrams(weightParser.ParseWeight(sourceWeight))
		} else if fallbackWeight != "" {
			repairedWeight = fallbackWeight
		}

		if repairedWeight == "" {
			logger.GetGlobalLogger("shein/product").Warnf("⚠️ 变体 %s 缺少可用重量，无法修复", data.Variants[i].ASIN)
			continue
		}

		data.Variants[i].Weight = types.FlexibleString(repairedWeight)
		fixedCount++
		logger.GetGlobalLogger("shein/product").Debugf("✅ 已修复变体 %s 的重量为 %sg", data.Variants[i].ASIN, repairedWeight)
	}

	if fixedCount > 0 {
		logger.GetGlobalLogger("shein/product").Debugf("✅ 共修复 %d 个变体重量", fixedCount)
	}
	return data
}

func formatVariantWeightInGrams(weight float64) string {
	trimmed := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", weight), "0"), ".")
	if trimmed == "" {
		return "0"
	}
	return trimmed
}

// filterValidASINs 过滤有效的ASIN
func (h *SaleAttributeHandler) filterValidASINs(variantProducts *[]model.Product, saleAttributeData sheinattr.ResultSaleAttribute) sheinattr.ResultSaleAttribute {
	providedASINs := make(map[string]bool)

	// 如果没有变体，说明是单体产品，不需要过滤
	if variantProducts == nil || len(*variantProducts) == 0 {
		logger.GetGlobalLogger("shein/product").Debugf("📦 单体产品模式，跳过ASIN过滤，保留AI生成的%d个变体", len(saleAttributeData.Variants))
		return saleAttributeData
	}

	// 多变体产品：构建提供的ASIN列表
	for _, product := range *variantProducts {
		providedASINs[product.Asin] = true
	}

	var validVariants []sheinattr.Variant
	removedCount := 0

	for _, variant := range saleAttributeData.Variants {
		if !providedASINs[variant.ASIN] {
			logger.GetGlobalLogger("shein/product").Warnf("AI生成了未提供的ASIN: %s，将被删除", variant.ASIN)
			removedCount++
			continue
		}
		validVariants = append(validVariants, variant)
	}

	saleAttributeData.Variants = validVariants

	if removedCount > 0 {
		logger.GetGlobalLogger("shein/product").Debugf("已删除%d个AI生成的多余ASIN，保留%d个有效变体", removedCount, len(validVariants))
	}

	logger.GetGlobalLogger("shein/product").Debugf("AI成功生成了%d个变体，期望%d个，数量在允许范围内", len(saleAttributeData.Variants), len(*variantProducts))

	return saleAttributeData
}

// validateAttributeValueConsistency 验证属性值与原始数据的一致性
func (h *SaleAttributeHandler) validateAttributeValueConsistency(amazonProduct model.Product, data sheinattr.ResultSaleAttribute) sheinattr.ResultSaleAttribute {

	if amazonProduct.VariationsValues == nil {
		logger.GetGlobalLogger("shein/product").Debug("原始产品无变体属性值，跳过一致性验证")
		return data
	}

	// 构建原始属性值映射：维度名 -> 原始值列表
	originalValues := make(map[string][]string)
	for _, variation := range amazonProduct.VariationsValues {
		dimensionName := normalizeAttributeKey(variation.VariantName)
		if dimensionName == "" {
			continue
		}
		originalValues[dimensionName] = variation.Values
	}

	// 定义AI可以合理添加的默认属性值（用于满足SHEIN必填要求）
	// 注意：匹配时会忽略大小写
	allowedDefaultValues := map[string][]string{
		"size":  {"one-size", "One-Size", "ONE-SIZE"}, // AI实际使用的默认size值，支持多种大小写形式
		"style": {"Standard", "Classic", "Basic", "Default"},
		"color": {"Default", "Multi-Color", "Default-Color"},
	}

	// 构建按维度忽略大小写的默认值映射
	allowedDefaultValuesLower := h.buildCaseInsensitiveDefaultValuesByDimension(allowedDefaultValues)

	inconsistentCount := 0

	// 验证销售属性中的值
	for i := range data.SaleAttributes {
		saleAttr := &data.SaleAttributes[i]
		dimensionName := h.inferOriginalDimensionForSaleAttribute(*saleAttr, originalValues, allowedDefaultValuesLower)
		if dimensionName == "" {
			continue
		}

		var cleanedValues []sheinattr.AttributeValue
		seenValues := make(map[string]bool)
		for _, attrValue := range saleAttr.AttrValue {
			resolvedValue, keepValue := h.resolveValueWithinDimension(attrValue.Value, dimensionName, originalValues, allowedDefaultValuesLower)
			if !keepValue {
				logger.GetGlobalLogger("shein/product").Warnf("发现跨维度污染的销售属性值: attrID=%d, dimension=%s, value='%s'，已移除", saleAttr.AttrID, dimensionName, attrValue.Value)
				inconsistentCount++
				continue
			}
			attrValue.Value = resolvedValue
			normalizedValue := normalizeAttributeValue(resolvedValue)
			if normalizedValue != "" && seenValues[normalizedValue] {
				continue
			}
			seenValues[normalizedValue] = true
			cleanedValues = append(cleanedValues, attrValue)
		}
		saleAttr.AttrValue = cleanedValues
	}

	// 验证变体中的属性值
	for i, variant := range data.Variants {
		for attrName, attrValue := range variant.Attributes {
			dimensionName := normalizeAttributeKey(attrName)
			if _, exists := originalValues[dimensionName]; !exists {
				continue
			}

			resolvedValue, keepValue := h.resolveValueWithinDimension(attrValue, dimensionName, originalValues, allowedDefaultValuesLower)
			if !keepValue {
				logger.GetGlobalLogger("shein/product").Warnf("变体 %s 中发现跨维度污染的属性值: %s='%s'，已移除", variant.ASIN, attrName, attrValue)
				delete(data.Variants[i].Attributes, attrName)
				inconsistentCount++
				continue
			}
			data.Variants[i].Attributes[attrName] = resolvedValue
			if resolvedValue != attrValue {
				logger.GetGlobalLogger("shein/product").Debugf("将变体 %s 的属性值 '%s' 修正为原始值 '%s'", variant.ASIN, attrValue, resolvedValue)
			}
		}
	}

	if inconsistentCount > 0 {
		logger.GetGlobalLogger("shein/product").Warnf("共发现 %d 个不一致的属性值", inconsistentCount)
	} else {
		logger.GetGlobalLogger("shein/product").Debug("所有属性值与原始数据保持一致或为合理的AI默认值")
	}

	return data
}

// buildCaseInsensitiveDefaultValuesByDimension 构建按维度忽略大小写的默认值映射
func (h *SaleAttributeHandler) buildCaseInsensitiveDefaultValuesByDimension(allowedDefaults map[string][]string) map[string]map[string]bool {
	caseInsensitiveMap := make(map[string]map[string]bool)

	for dimensionName, defaultList := range allowedDefaults {
		normalizedDimension := normalizeAttributeKey(dimensionName)
		if normalizedDimension == "" {
			continue
		}
		if caseInsensitiveMap[normalizedDimension] == nil {
			caseInsensitiveMap[normalizedDimension] = make(map[string]bool)
		}
		for _, defaultValue := range defaultList {
			normalizedValue := strings.ToLower(strings.TrimSpace(defaultValue))
			caseInsensitiveMap[normalizedDimension][normalizedValue] = true
		}
	}

	return caseInsensitiveMap
}

// isAllowedDefaultValueOptimized 使用预处理的映射进行优化的默认值检查（忽略大小写）
func (h *SaleAttributeHandler) isAllowedDefaultValueOptimized(value string, dimensionName string, allowedDefaultsLower map[string]map[string]bool) bool {
	valueLower := strings.ToLower(strings.TrimSpace(value))
	dimensionDefaults := allowedDefaultsLower[normalizeAttributeKey(dimensionName)]
	return dimensionDefaults[valueLower]
}

func (h *SaleAttributeHandler) inferOriginalDimensionForSaleAttribute(
	saleAttr sheinattr.ResultAttribute,
	originalValues map[string][]string,
	allowedDefaultsLower map[string]map[string]bool,
) string {
	bestDimension := ""
	bestScore := 0

	for dimensionName, valueList := range originalValues {
		score := 0
		for _, attrValue := range saleAttr.AttrValue {
			if h.isValueInDimension(attrValue.Value, valueList) || h.isAllowedDefaultValueOptimized(attrValue.Value, dimensionName, allowedDefaultsLower) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestDimension = dimensionName
		}
	}

	if bestScore == 0 {
		return ""
	}
	return bestDimension
}

func (h *SaleAttributeHandler) resolveValueWithinDimension(
	value string,
	dimensionName string,
	originalValues map[string][]string,
	allowedDefaultsLower map[string]map[string]bool,
) (string, bool) {
	valueList, exists := originalValues[normalizeAttributeKey(dimensionName)]
	if !exists {
		return value, true
	}

	if h.isValueInDimension(value, valueList) {
		return value, true
	}

	if h.isAllowedDefaultValueOptimized(value, dimensionName, allowedDefaultsLower) {
		logger.GetGlobalLogger("shein/product").Debugf("✅ AI添加的合理默认属性值: dimension=%s, value='%s'", dimensionName, value)
		return value, true
	}

	if correctedValue := h.findMostSimilarValueInDimension(value, valueList); correctedValue != "" {
		return correctedValue, true
	}

	return "", false
}

func (h *SaleAttributeHandler) isValueInDimension(value string, originalValues []string) bool {
	for _, originalValue := range originalValues {
		if normalizeAttributeValue(originalValue) == normalizeAttributeValue(value) {
			return true
		}
	}
	return false
}

// findMostSimilarValueInDimension 找到同一维度内最相似的原始属性值
func (h *SaleAttributeHandler) findMostSimilarValueInDimension(targetValue string, originalValues []string) string {
	targetLower := strings.ToLower(strings.TrimSpace(targetValue))

	// 首先尝试精确匹配（忽略大小写）
	for _, originalValue := range originalValues {
		if strings.ToLower(strings.TrimSpace(originalValue)) == targetLower {
			return originalValue
		}
	}

	// 然后尝试包含匹配
	for _, originalValue := range originalValues {
		originalLower := strings.ToLower(strings.TrimSpace(originalValue))
		if strings.Contains(originalLower, targetLower) || strings.Contains(targetLower, originalLower) {
			return originalValue
		}
	}

	return ""
}

func normalizeAttributeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeAttributeValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
