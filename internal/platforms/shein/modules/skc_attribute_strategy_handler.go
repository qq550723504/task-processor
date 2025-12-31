package modules

import (
	"fmt"
	"task-processor/internal/common/shein/api/attribute"

	"github.com/sirupsen/logrus"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// AttributeStrategyHandler 属性策略处理器
type AttributeStrategyHandler struct{}

// NewAttributeStrategyHandler 创建新的属性策略处理器
func NewAttributeStrategyHandler() *AttributeStrategyHandler {
	return &AttributeStrategyHandler{}
}

// DetermineAttributeStrategy 根据销售属性动态确定构建策略
func (h *AttributeStrategyHandler) DetermineAttributeStrategy(saleAttributeData ResultSaleAttribute, config AttributePriorityConfig, attributeTemplates *attribute.AttributeTemplateInfo) AttributeStrategy {
	var primaryAttr ResultAttribute
	var secondaryAttr ResultAttribute
	strategyType := "single_variant"

	// 0. 优先检查必填属性作为主规格
	foundPrimaryAttr := false
	logrus.Infof("🎯 开始确定属性策略，优先检查必填属性")

	// 首先尝试使用必填属性作为主要属性
	for _, attr := range saleAttributeData.SaleAttributes {
		// 检查是否为必填属性且有有效值
		if len(attr.AttrValue) > 0 {
			if h.validateAttributeInVariants(attr.AttrID, attr.AttrValue, saleAttributeData.Variants, attributeTemplates) {
				// 检查该属性是否为必填属性（通过检查是否在模板中标记为必填）
				if h.isRequiredAttribute(attr.AttrID, attributeTemplates) {
					primaryAttr = attr
					if len(attr.AttrValue) > 1 {
						strategyType = "multi_variant"
					} else {
						strategyType = "single_primary_variant"
					}
					logrus.Infof("🎯 使用必填属性作为主规格: ID=%d, 变体数量=%d, 策略类型=%s",
						attr.AttrID, len(attr.AttrValue), strategyType)
					foundPrimaryAttr = true
					break
				}
			}
		}
	}

	// 1. 如果没有找到必填属性，按优先级寻找主要属性
	if !foundPrimaryAttr {
		logrus.Infof("未找到必填属性，按优先级寻找主要属性")
		for _, priorityID := range config.SKCPrimaryAttributePriority {
			for _, attr := range saleAttributeData.SaleAttributes {
				if attr.AttrID == priorityID && len(attr.AttrValue) > 0 {
					// 验证该属性值在变体中实际存在
					if h.validateAttributeInVariants(attr.AttrID, attr.AttrValue, saleAttributeData.Variants, attributeTemplates) {
						primaryAttr = attr
						if len(attr.AttrValue) > 1 {
							strategyType = "multi_variant"
						} else {
							strategyType = "single_primary_variant"
						}
						logrus.Infof("选择主要属性: ID=%d, 变体数量=%d, 策略类型=%s",
							attr.AttrID, len(attr.AttrValue), strategyType)
						foundPrimaryAttr = true
						break
					} else {
						logrus.Warnf("属性 ID=%d 在变体中验证失败，跳过", attr.AttrID)
					}
				}
			}
			if foundPrimaryAttr {
				break
			}
		}
	}

	// 如果没有找到合适的主要属性，检查是否有必填属性可以使用
	if !foundPrimaryAttr {
		logrus.Infof("未找到合适的主要属性，检查必填属性")

		// 优先使用必填属性作为主要属性
		for _, attr := range saleAttributeData.SaleAttributes {
			// 检查是否为必填属性且有有效值
			if len(attr.AttrValue) > 0 {
				if h.validateAttributeInVariants(attr.AttrID, attr.AttrValue, saleAttributeData.Variants, attributeTemplates) {
					primaryAttr = attr
					if len(attr.AttrValue) > 1 {
						strategyType = "multi_variant"
					} else {
						strategyType = "single_primary_variant"
					}
					logrus.Infof("使用必填属性作为主要属性: ID=%d, 变体数量=%d, 策略类型=%s",
						attr.AttrID, len(attr.AttrValue), strategyType)
					foundPrimaryAttr = true
					break
				}
			}
		}

		// 如果仍未找到，检查是否只有尺寸属性而没有颜色属性
		if !foundPrimaryAttr {
			logrus.Infof("未找到必填属性，检查是否只有尺寸属性")

			// 检查是否只有尺寸属性
			hasSizeAttribute := false
			var sizeAttribute ResultAttribute
			for _, attr := range saleAttributeData.SaleAttributes {
				if attr.AttrID == 87 && len(attr.AttrValue) > 0 { // 87是尺寸属性ID
					if h.validateAttributeInVariants(attr.AttrID, attr.AttrValue, saleAttributeData.Variants, attributeTemplates) {
						hasSizeAttribute = true
						sizeAttribute = attr
						break
					}
				}
			}

			// 如果只有尺寸属性，创建一个默认的颜色属性作为主要属性
			if hasSizeAttribute && !h.hasColorAttribute(saleAttributeData) {
				logrus.Warnf("检测到只有尺寸属性而没有颜色属性，但这可能导致主规格错误")
				logrus.Infof("建议：优先使用其他可用属性而不是创建默认颜色属性")

				// 不再自动创建颜色属性，而是使用尺寸属性作为主属性
				primaryAttr = sizeAttribute
				strategyType = "single_primary_variant"
				logrus.Infof("使用尺寸属性作为主要属性: ID=%d, 变体数量=%d", primaryAttr.AttrID, len(primaryAttr.AttrValue))
				foundPrimaryAttr = true
			}
		}
	}

	// 2. 如果没有找到合适的主要属性，使用默认属性或创建默认单变体属性
	if primaryAttr.AttrID <= 0 {
		primaryAttr = h.createDefaultPrimaryAttribute(saleAttributeData, config)
		strategyType = "single_variant"
	}

	// 3. 寻找次要属性（用于SKU分组）
	secondaryAttr = h.findBestSecondaryAttribute(saleAttributeData, primaryAttr.AttrID, config.SKUSecondaryAttributePriority, attributeTemplates)

	// 4. 确定最终策略类型
	strategyType = h.determineStrategyType(primaryAttr, secondaryAttr, attributeTemplates)

	logrus.Infof("属性策略确定完成 - 主要属性: %d, 次要属性: %d, 策略类型: %s",
		primaryAttr.AttrID, secondaryAttr.AttrID, strategyType)

	return AttributeStrategy{
		PrimaryAttribute:   primaryAttr,
		SecondaryAttribute: secondaryAttr,
		StrategyType:       strategyType,
	}
}

// createDefaultPrimaryAttribute 创建默认的主要属性
func (h *AttributeStrategyHandler) createDefaultPrimaryAttribute(saleAttributeData ResultSaleAttribute, config AttributePriorityConfig) ResultAttribute {
	// 首先尝试从现有销售属性中找默认属性
	for _, attr := range saleAttributeData.SaleAttributes {
		if attr.AttrID == config.DefaultSKCAttributeID {
			return attr
		}
	}

	// 如果仍然没有找到，且只有一个变体，创建一个默认的单变体属性
	if len(saleAttributeData.Variants) == 1 {
		primaryAttr := ResultAttribute{
			AttrID: config.DefaultSKCAttributeID, // 默认使用颜色属性(27)
			AttrValue: []AttributeValue{
				{
					ID:    -1,        // 标记为需要映射的自定义值
					Value: "Default", // 默认值
				},
			},
		}
		logrus.Infof("单变体情况：创建默认主要属性 ID=%d, Value=Default", config.DefaultSKCAttributeID)
		return primaryAttr
	}

	return ResultAttribute{AttrID: -1}
}

// findBestSecondaryAttribute 动态查找最佳次要属性
func (h *AttributeStrategyHandler) findBestSecondaryAttribute(saleAttributeData ResultSaleAttribute, primaryAttrID int, priorityList []int, attributeTemplates *attribute.AttributeTemplateInfo) ResultAttribute {
	var bestSecondaryAttr ResultAttribute

	// 第一步：按照优先级列表查找，但跳过只有一个值的属性，并验证在变体中的存在性
	for _, priorityID := range priorityList {
		for _, attr := range saleAttributeData.SaleAttributes {
			if attr.AttrID == priorityID && attr.AttrID != primaryAttrID && len(attr.AttrValue) > 1 {
				if h.validateAttributeInVariants(attr.AttrID, attr.AttrValue, saleAttributeData.Variants, attributeTemplates) {
					bestSecondaryAttr = attr
					logrus.Infof("按优先级找到次要属性: ID=%d, 值数量=%d (已验证在变体中存在)", attr.AttrID, len(attr.AttrValue))
					return bestSecondaryAttr
				} else {
					logrus.Warnf("优先级属性 ID=%d 在变体中验证失败，跳过", attr.AttrID)
				}
			}
		}
	}

	// 第二步：智能补充必填的销售属性
	if bestSecondaryAttr.AttrID < 0 && attributeTemplates != nil && len(attributeTemplates.Data) > 0 {
		bestSecondaryAttr = h.createMissingRequiredAttribute(primaryAttrID, priorityList, attributeTemplates)
	}

	return bestSecondaryAttr
}

// validateAttributeInVariants 验证属性值在变体中的存在性
func (h *AttributeStrategyHandler) validateAttributeInVariants(attrID int, attrValues []AttributeValue, variants []Variant, attributeTemplates *attribute.AttributeTemplateInfo) bool {
	// 获取属性名称的可能变体
	attrNames := h.getAttributeNameVariations(attrID)

	// 检查是否有变体包含这些属性值
	matchedCount := 0
	for _, attrValue := range attrValues {
		for _, variant := range variants {
			for _, attrName := range attrNames {
				if variantValue, exists := variant.Attributes[attrName]; exists {
					if h.isValueMatch(variantValue, attrValue.Value) {
						matchedCount++
						goto nextValue
					}
				}
			}
		nextValue:
		}
	}

	validationRate := float64(matchedCount) / float64(len(attrValues))
	logrus.Debugf("属性ID %d 在变体中的验证率: %.2f%% (%d/%d)",
		attrID, validationRate*100, matchedCount, len(attrValues))

	// 检查是否为必填属性，如果是则采用宽松验证策略
	if h.isRequiredAttribute(attrID, attributeTemplates) {
		logrus.Infof("属性ID %d 是SHEIN必填属性，采用宽松验证策略", attrID)
		// 对于必填属性，如果有任何匹配或者属性值不为空，就认为通过验证
		return matchedCount > 0 || len(attrValues) > 0
	}

	// 对于非必填属性，要求至少30%的匹配率
	return validationRate >= 0.3
}

// isRequiredAttribute 检查属性是否为必填属性
func (h *AttributeStrategyHandler) isRequiredAttribute(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) bool {
	if attributeTemplates == nil || len(attributeTemplates.Data) == 0 {
		return false
	}

	for _, attr := range attributeTemplates.Data[0].AttributeInfos {
		if attr.AttributeID == attrID && attr.AttributeType == 1 { // 销售属性
			// 检查是否为必填属性
			if attr.AttributeLabel == 1 {
				logrus.Debugf("属性ID %d (%s) 是必填销售属性 (AttributeLabel=1)",
					attrID, attr.AttributeNameEn)
				return true
			}
		}
	}
	return false
}

// getAttributeNameVariations 获取属性名称的可能变体
func (h *AttributeStrategyHandler) getAttributeNameVariations(attrID int) []string {
	switch attrID {
	case 27:
		return []string{"color", "Color", "颜色"}
	case 87:
		return []string{"size", "Size", "尺寸", "尺码"}
	case 1001184: // Style属性
		return []string{"style", "Style", "风格", "款式", "pattern", "Pattern"}
	case 1001365: // Scent Type
		return []string{"scent", "Scent", "香味", "scent_type", "Scent Type"}
	case 1001410: // Net Content
		return []string{"net_content", "Net Content", "净含量", "content"}
	default:
		// 对于未知属性，尝试通用的属性名称
		return []string{"attr_" + fmt.Sprintf("%d", attrID), "attribute", "value"}
	}
}

// isValueMatch 判断两个值是否匹配
func (h *AttributeStrategyHandler) isValueMatch(variantValue, attrValue string) bool {
	return variantValue == attrValue ||
		h.normalizeValue(variantValue) == h.normalizeValue(attrValue)
}

// normalizeValue 标准化值用于比较
func (h *AttributeStrategyHandler) normalizeValue(value string) string {
	// 简单的标准化：转小写并去除首尾空格
	return value // 保持原样，避免过度标准化
}

// createMissingRequiredAttribute 为必填但缺失的销售属性创建默认值
func (h *AttributeStrategyHandler) createMissingRequiredAttribute(primaryAttrID int, priorityList []int, attributeTemplates *attribute.AttributeTemplateInfo) ResultAttribute {
	for _, priorityID := range priorityList {
		if priorityID == primaryAttrID {
			continue // 跳过已选为主要属性的
		}

		// 检查这个属性是否在模板中且必填
		for _, templateAttr := range attributeTemplates.Data[0].AttributeInfos {
			if templateAttr.AttributeID == priorityID && templateAttr.AttributeType == 1 {
				// 检查是否为必填的销售属性
				isRequired := len(templateAttr.AttributeRemarkList) > 0 ||
					templateAttr.AttributeLabel == 1 ||
					templateAttr.IsSample == 1

				if isRequired {
					// 为必填但缺失的销售属性创建默认值
					attrName := templateAttr.AttributeName
					if attrName == "" {
						attrName = templateAttr.AttributeNameEn
					}
					titleCaser := cases.Title(language.English)
					defaultValue := fmt.Sprintf("Default %s", titleCaser.String(attrName))

					secondaryAttr := ResultAttribute{
						AttrID: priorityID,
						AttrValue: []AttributeValue{
							{
								ID:    -1,           // 标记为需要映射的自定义值
								Value: defaultValue, // 默认值
							},
						},
					}
					logrus.Infof("为必填但缺失的销售属性创建默认值: ID=%d (%s), Value=%s",
						priorityID, templateAttr.AttributeNameEn, defaultValue)
					return secondaryAttr
				}
			}
		}
	}
	return ResultAttribute{AttrID: -1}
}

// determineStrategyType 确定最终策略类型
func (h *AttributeStrategyHandler) determineStrategyType(primaryAttr, secondaryAttr ResultAttribute, attributeTemplates *attribute.AttributeTemplateInfo) string {
	if primaryAttr.AttrID != 0 && secondaryAttr.AttrID >= 0 {
		// 安全地获取属性名称
		primaryName := h.getAttributeNameSafe(primaryAttr.AttrID, attributeTemplates)
		secondaryName := h.getAttributeNameSafe(secondaryAttr.AttrID, attributeTemplates)
		return fmt.Sprintf("%s_%s", primaryName, secondaryName)
	} else if primaryAttr.AttrID != 0 {
		primaryName := h.getAttributeNameSafe(primaryAttr.AttrID, attributeTemplates)
		strategyType := primaryName + "_only"

		// SHEIN规则检查：只有主规格没有次规格时，确保每个SKC只有一个SKU
		if len(primaryAttr.AttrValue) > 1 {
			logrus.Warnf("SHEIN规则警告：只有主规格(%s)没有次规格，每个SKC只能有一个SKU", primaryName)
		}
		return strategyType
	}
	return "single_variant"
}

// getAttributeNameSafe 安全地获取属性名称
func (h *AttributeStrategyHandler) getAttributeNameSafe(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) string {
	if attributeTemplates != nil && len(attributeTemplates.Data) > 0 {
		for i := range attributeTemplates.Data[0].AttributeInfos {
			if attributeTemplates.Data[0].AttributeInfos[i].AttributeID == attrID {
				attrInfo := &attributeTemplates.Data[0].AttributeInfos[i]
				if attrInfo.AttributeName != "" {
					return attrInfo.AttributeName
				}
				if attrInfo.AttributeNameEn != "" {
					return attrInfo.AttributeNameEn
				}
			}
		}
	}

	// 降级到硬编码
	switch attrID {
	case 27:
		return "color"
	case 87:
		return "size"
	case 1001184:
		return "style"
	case 1001365:
		return "scent_type"
	case 1001410:
		return "net_content"
	default:
		return fmt.Sprintf("attr_%d", attrID)
	}
}

// GetDynamicAttributePriorityConfig 根据属性模板数据动态生成属性优先级配置
func (h *AttributeStrategyHandler) GetDynamicAttributePriorityConfig(attributeTemplates *attribute.AttributeTemplateInfo) AttributePriorityConfig {
	if attributeTemplates == nil || len(attributeTemplates.Data) == 0 {
		return h.getDefaultAttributePriorityConfig()
	}

	// 分析销售属性的重要性
	var saleAttributes []AttributeImportance

	for _, attribute := range attributeTemplates.Data[0].AttributeInfos {
		if attribute.AttributeType == 1 { // 销售属性
			// 使用统一的重要性计算函数
			calc := NewAttributeImportanceCalculator()
			importanceResult := calc.CalculateAttributeImportance(&attribute)
			saleAttributes = append(saleAttributes, AttributeImportance{
				AttrID:     attribute.AttributeID,
				Importance: importanceResult.Importance,
			})
		}
	}

	// 按重要性排序 (从高到低)
	for i := 0; i < len(saleAttributes)-1; i++ {
		for j := i + 1; j < len(saleAttributes); j++ {
			if saleAttributes[i].Importance < saleAttributes[j].Importance {
				saleAttributes[i], saleAttributes[j] = saleAttributes[j], saleAttributes[i]
			}
		}
	}

	// 构建优先级配置
	config := h.getDefaultAttributePriorityConfig()

	// 填充主要属性优先级（用于SKC分组）
	config.SKCPrimaryAttributePriority = []int{}
	for _, attr := range saleAttributes {
		config.SKCPrimaryAttributePriority = append(config.SKCPrimaryAttributePriority, attr.AttrID)
	}

	// 填充次要属性优先级（用于SKU分组）
	config.SKUSecondaryAttributePriority = []int{}
	for i := len(saleAttributes) - 1; i >= 0; i-- {
		config.SKUSecondaryAttributePriority = append(config.SKUSecondaryAttributePriority, saleAttributes[i].AttrID)
	}

	// 设置默认主规格属性为重要性最高的属性
	if len(saleAttributes) > 0 {
		config.DefaultSKCAttributeID = saleAttributes[0].AttrID
		logrus.Infof("动态生成的属性策略 - 主规格优先级: %v, 默认主规格: %d",
			config.SKCPrimaryAttributePriority, config.DefaultSKCAttributeID)
	} else {
		logrus.Infof("未找到有效的销售属性，使用默认配置")
	}

	return config
}

// getDefaultAttributePriorityConfig 获取默认的属性优先级配置
func (h *AttributeStrategyHandler) getDefaultAttributePriorityConfig() AttributePriorityConfig {
	return AttributePriorityConfig{
		// 主要属性(SKC分组)：按通用优先级排序，实际使用时会根据属性重要性动态调整
		SKCPrimaryAttributePriority: []int{27, 87, 1001365, 1001410},
		// 次要属性(SKU分组)：按通用优先级排序
		SKUSecondaryAttributePriority: []int{87, 1001365, 1001410},
		DefaultSKCAttributeID:         27, // 默认使用颜色
		AttributeNameToID: map[string]int{
			"color":       27,
			"size":        87,
			"Scent Type":  1001365,
			"Net Content": 1001410,
		},
	}
}

// hasColorAttribute 检查是否存在颜色属性
func (h *AttributeStrategyHandler) hasColorAttribute(saleAttributeData ResultSaleAttribute) bool {
	for _, attr := range saleAttributeData.SaleAttributes {
		if attr.AttrID == 27 { // 27是颜色属性ID
			return true
		}
	}
	return false
}
