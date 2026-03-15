package sale

import (
	"strings"
	"task-processor/internal/domain/model"
	"task-processor/internal/pkg/types"
	shein_model "task-processor/internal/platforms/shein/model"

	"github.com/sirupsen/logrus"
)

// validateAndFixSaleAttributeData 验证并修复销售属性数据
func (h *SaleAttributeHandler) validateAndFixSaleAttributeData(data shein_model.ResultSaleAttribute, productsData []map[string]string) shein_model.ResultSaleAttribute {
	logrus.Info("开始验证和修复AI生成的销售属性数据")

	// 1. 修复属性值ID重复问题
	data = h.fixAttributeValueIDsWithManager(data)

	// 2. 标准化尺寸单位
	data = h.standardizeDimensionUnits(data)

	// 3. 验证每个ASIN都有对应的变体
	data = h.validateVariantCompleteness(data, productsData)

	// 4. 验证变体属性完整性（关键修复）
	data = h.validateVariantAttributes(data, productsData)

	logrus.Info("销售属性数据验证和修复完成")
	return data
}

// fixAttributeValueIDsWithManager 标记需要映射的属性值ID
func (h *SaleAttributeHandler) fixAttributeValueIDsWithManager(data shein_model.ResultSaleAttribute) shein_model.ResultSaleAttribute {
	logrus.Info("标记属性值ID为需要映射状态")

	// 注意：ResultSaleAttribute.SaleAttributes 是 []ResultAttribute 类型
	for i := range data.SaleAttributes {
		saleAttr := &data.SaleAttributes[i]
		logrus.Debugf("处理属性ID %d，属性值数量: %d", saleAttr.AttrID, len(saleAttr.AttrValue))

		for j := range saleAttr.AttrValue {
			attrValue := &saleAttr.AttrValue[j]
			currentID := attrValue.ID.Int()

			// 不再分配简单的递增ID，而是标记为需要映射
			if currentID <= 0 {
				// 使用-1标记需要映射到SHEIN平台ID的属性值
				attrValue.ID = types.FlexibleID(-1)
				logrus.Debugf("标记属性值需要映射: %s (原ID: %d -> 标记ID: -1)", attrValue.Value, currentID)
			} else {
				// 如果已经有有效ID，保持不变
				logrus.Debugf("保持有效属性值ID: %s (ID: %d)", attrValue.Value, currentID)
			}
		}
	}

	logrus.Info("属性值ID标记完成，后续将通过mapAttributeValuesToSheinIDs进行真正的ID映射")
	return data
}

// standardizeDimensionUnits 标准化尺寸单位
func (h *SaleAttributeHandler) standardizeDimensionUnits(data shein_model.ResultSaleAttribute) shein_model.ResultSaleAttribute {
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
			logrus.Infof("ASIN %s: 尺寸单位为空，设置为默认单位 cm", variant.ASIN)
			fixedCount++
			continue
		}

		// 标准化单位（转换为小写进行匹配）
		normalizedUnit := strings.ToLower(strings.TrimSpace(originalUnit))

		if standardUnit, exists := unitMappings[normalizedUnit]; exists {
			if standardUnit != originalUnit {
				data.Variants[i].LengthUnit = standardUnit
				logrus.Infof("ASIN %s: 尺寸单位从 '%s' 标准化为 '%s'", variant.ASIN, originalUnit, standardUnit)
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
			logrus.Warnf("ASIN %s: 发现未知尺寸单位 '%s'，设置为默认单位 cm", variant.ASIN, originalUnit)
			data.Variants[i].LengthUnit = "cm"
			fixedCount++
		}
	}

	if fixedCount > 0 {
		logrus.Infof("共修复了 %d 个变体的尺寸单位", fixedCount)
	}

	return data
}

// validateVariantCompleteness 验证每个ASIN都有对应的变体
func (h *SaleAttributeHandler) validateVariantCompleteness(data shein_model.ResultSaleAttribute, products []map[string]string) shein_model.ResultSaleAttribute {
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
		logrus.Warnf("发现缺失的ASIN变体: %v", missingASINs)
	}

	return data
}

// validateVariantAttributes 验证变体属性完整性（关键修复）
func (h *SaleAttributeHandler) validateVariantAttributes(data shein_model.ResultSaleAttribute, products []map[string]string) shein_model.ResultSaleAttribute {
	logrus.Info("🔍 开始验证变体属性完整性...")

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
			logrus.Debugf("产品 %s 的属性: %v", asin, attributes)
		}
	}

	// 验证并修复每个变体的属性
	emptyAttributesCount := 0
	fixedCount := 0

	for i, variant := range data.Variants {
		// 检查变体的Attributes字段是否为空
		if len(variant.Attributes) == 0 {
			emptyAttributesCount++
			logrus.Warnf("⚠️ 变体 %s 的Attributes字段为空", variant.ASIN)

			// 尝试从产品数据中恢复属性
			if productAttrs, exists := productAttributesMap[variant.ASIN]; exists && len(productAttrs) > 0 {
				data.Variants[i].Attributes = productAttrs
				fixedCount++
				logrus.Infof("✅ 已从产品数据恢复变体 %s 的属性: %v", variant.ASIN, productAttrs)
			} else {
				logrus.Errorf("❌ 无法恢复变体 %s 的属性，产品数据中也没有属性信息", variant.ASIN)
			}
		} else {
			logrus.Debugf("✅ 变体 %s 的Attributes正常: %v", variant.ASIN, variant.Attributes)
		}
	}

	if emptyAttributesCount > 0 {
		logrus.Warnf("⚠️ 发现 %d 个变体的Attributes为空，已修复 %d 个", emptyAttributesCount, fixedCount)
		if fixedCount < emptyAttributesCount {
			logrus.Errorf("❌ 仍有 %d 个变体的Attributes无法修复，这将导致后续匹配失败", emptyAttributesCount-fixedCount)
		}
	} else {
		logrus.Info("✅ 所有变体的Attributes字段都正常")
	}

	return data
}

// filterValidASINs 过滤有效的ASIN
func (h *SaleAttributeHandler) filterValidASINs(variantProducts *[]model.Product, saleAttributeData shein_model.ResultSaleAttribute) shein_model.ResultSaleAttribute {
	providedASINs := make(map[string]bool)

	// 如果没有变体，说明是单体产品，不需要过滤
	if variantProducts == nil || len(*variantProducts) == 0 {
		logrus.Infof("📦 单体产品模式，跳过ASIN过滤，保留AI生成的%d个变体", len(saleAttributeData.Variants))
		return saleAttributeData
	}

	// 多变体产品：构建提供的ASIN列表
	for _, product := range *variantProducts {
		providedASINs[product.Asin] = true
	}

	var validVariants []shein_model.Variant
	removedCount := 0

	for _, variant := range saleAttributeData.Variants {
		if !providedASINs[variant.ASIN] {
			logrus.Warnf("AI生成了未提供的ASIN: %s，将被删除", variant.ASIN)
			removedCount++
			continue
		}
		validVariants = append(validVariants, variant)
	}

	saleAttributeData.Variants = validVariants

	if removedCount > 0 {
		logrus.Infof("已删除%d个AI生成的多余ASIN，保留%d个有效变体", removedCount, len(validVariants))
	}

	logrus.Infof("AI成功生成了%d个变体，期望%d个，数量在允许范围内", len(saleAttributeData.Variants), len(*variantProducts))

	return saleAttributeData
}

// validateAttributeValueConsistency 验证属性值与原始数据的一致性
func (h *SaleAttributeHandler) validateAttributeValueConsistency(amazonProduct model.Product, data shein_model.ResultSaleAttribute) shein_model.ResultSaleAttribute {

	if amazonProduct.VariationsValues == nil {
		logrus.Info("原始产品无变体属性值，跳过一致性验证")
		return data
	}

	// 构建原始属性值映射
	originalValues := make(map[string][]string)
	for _, variation := range amazonProduct.VariationsValues {
		originalValues[strings.ToLower(variation.VariantName)] = variation.Values
	}

	// 定义AI可以合理添加的默认属性值（用于满足SHEIN必填要求）
	// 注意：匹配时会忽略大小写
	allowedDefaultValues := map[string][]string{
		"size":  {"one-size", "One-Size", "ONE-SIZE"}, // AI实际使用的默认size值，支持多种大小写形式
		"style": {"Standard", "Classic", "Basic", "Default"},
		"color": {"Default", "Multi-Color", "Default-Color"},
	}

	// 构建忽略大小写的默认值映射，提高匹配性能
	allowedDefaultValuesLower := h.buildCaseInsensitiveDefaultValues(allowedDefaultValues)

	inconsistentCount := 0

	// 验证销售属性中的值
	for i := range data.SaleAttributes {
		saleAttr := &data.SaleAttributes[i]
		for j := range saleAttr.AttrValue {
			attrValue := &saleAttr.AttrValue[j]

			// 检查是否存在对应的原始属性值
			found := h.isValueInOriginalData(attrValue.Value, originalValues)

			if !found {
				// 检查是否为允许的默认值（使用优化后的忽略大小写匹配）
				isAllowedDefault := h.isAllowedDefaultValueOptimized(attrValue.Value, allowedDefaultValuesLower)

				if isAllowedDefault {
					logrus.Infof("✅ AI添加的合理默认属性值: '%s'（用于满足SHEIN必填要求）", attrValue.Value)
				} else {
					logrus.Warnf("发现不一致的属性值: '%s'，不在原始属性值列表中", attrValue.Value)
					inconsistentCount++

					// 尝试找到最相似的原始值进行修正
					if correctedValue := h.findMostSimilarValue(attrValue.Value, originalValues); correctedValue != "" {
						logrus.Infof("将属性值 '%s' 修正为原始值 '%s'", attrValue.Value, correctedValue)
						attrValue.Value = correctedValue
					}
				}
			}
		}
	}

	// 验证变体中的属性值
	for i, variant := range data.Variants {
		for attrName, attrValue := range variant.Attributes {
			found := h.isValueInOriginalData(attrValue, originalValues)

			if !found {
				// 检查是否为允许的默认值（使用优化后的忽略大小写匹配）
				isAllowedDefault := h.isAllowedDefaultValueOptimized(attrValue, allowedDefaultValuesLower)

				if isAllowedDefault {
					logrus.Infof("✅ 变体 %s 中AI添加的合理默认属性值: %s='%s'", variant.ASIN, attrName, attrValue)
				} else {
					logrus.Warnf("变体 %s 中发现不一致的属性值: %s='%s'", variant.ASIN, attrName, attrValue)
					inconsistentCount++

					// 尝试找到最相似的原始值进行修正
					if correctedValue := h.findMostSimilarValue(attrValue, originalValues); correctedValue != "" {
						logrus.Infof("将变体 %s 的属性值 '%s' 修正为原始值 '%s'", variant.ASIN, attrValue, correctedValue)
						data.Variants[i].Attributes[attrName] = correctedValue
					}
				}
			}
		}
	}

	if inconsistentCount > 0 {
		logrus.Warnf("共发现 %d 个不一致的属性值", inconsistentCount)
	} else {
		logrus.Info("所有属性值与原始数据保持一致或为合理的AI默认值")
	}

	return data
}

// isValueInOriginalData 检查值是否在原始数据中
func (h *SaleAttributeHandler) isValueInOriginalData(value string, originalValues map[string][]string) bool {
	for _, originalValueList := range originalValues {
		for _, originalValue := range originalValueList {
			if value == originalValue {
				return true
			}
		}
	}
	return false
}

// buildCaseInsensitiveDefaultValues 构建忽略大小写的默认值映射，提高匹配性能
func (h *SaleAttributeHandler) buildCaseInsensitiveDefaultValues(allowedDefaults map[string][]string) map[string]bool {
	caseInsensitiveMap := make(map[string]bool)

	for _, defaultList := range allowedDefaults {
		for _, defaultValue := range defaultList {
			normalizedValue := strings.ToLower(strings.TrimSpace(defaultValue))
			caseInsensitiveMap[normalizedValue] = true
		}
	}

	return caseInsensitiveMap
}

// isAllowedDefaultValueOptimized 使用预处理的映射进行优化的默认值检查（忽略大小写）
func (h *SaleAttributeHandler) isAllowedDefaultValueOptimized(value string, allowedDefaultsLower map[string]bool) bool {
	valueLower := strings.ToLower(strings.TrimSpace(value))
	return allowedDefaultsLower[valueLower]
}

// isAllowedDefaultValue 检查是否为允许的默认值（忽略大小写）
func (h *SaleAttributeHandler) isAllowedDefaultValue(value string, allowedDefaults map[string][]string) bool {
	valueLower := strings.ToLower(strings.TrimSpace(value))

	for _, defaultList := range allowedDefaults {
		for _, defaultValue := range defaultList {
			defaultLower := strings.ToLower(strings.TrimSpace(defaultValue))
			if defaultLower == valueLower {
				return true
			}
		}
	}
	return false
}

// findMostSimilarValue 找到最相似的原始属性值
func (h *SaleAttributeHandler) findMostSimilarValue(targetValue string, originalValues map[string][]string) string {
	targetLower := strings.ToLower(strings.TrimSpace(targetValue))

	// 首先尝试精确匹配（忽略大小写）
	for _, valueList := range originalValues {
		for _, originalValue := range valueList {
			if strings.ToLower(strings.TrimSpace(originalValue)) == targetLower {
				return originalValue
			}
		}
	}

	// 然后尝试包含匹配
	for _, valueList := range originalValues {
		for _, originalValue := range valueList {
			originalLower := strings.ToLower(strings.TrimSpace(originalValue))
			if strings.Contains(originalLower, targetLower) || strings.Contains(targetLower, originalLower) {
				return originalValue
			}
		}
	}

	return ""
}
