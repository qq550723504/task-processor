// Package attribute 提供SHEIN平台属性验证和修复功能
package attribute

import (
	"strings"
	"task-processor/internal/core/logger"

	"task-processor/internal/pkg/types"
	"task-processor/internal/shein/api/attribute"
)

// AttributeSelectionValidator 属性选择验证器
type AttributeSelectionValidator struct {
	importanceCalc *AttributeImportanceCalculator
}

// NewAttributeSelectionValidator 创建新的属性选择验证器
func NewAttributeSelectionValidator() *AttributeSelectionValidator {
	return &AttributeSelectionValidator{
		importanceCalc: NewAttributeImportanceCalculator(),
	}
}

// ValidateAndFixAttributeSelection 增强版属性验证和修复
func (v *AttributeSelectionValidator) ValidateAndFixAttributeSelection(attributeData AttributeData, attributeInfo BuildAttributeInfo, attributeTemplates *attribute.AttributeTemplateInfo) AttributeData {
	// 创建属性ID到可用值的映射
	attrValueMap := make(map[int]map[int]string) // AttrID -> ValueID -> Value
	attrRequiredMap := make(map[int]bool)        // AttrID -> Required
	attrTypeMap := make(map[int]int)             // AttrID -> Type

	for _, attr := range attributeInfo.AttributeData {
		valueMap := make(map[int]string)
		for _, value := range attr.AttrValue {
			valueMap[value.ID] = value.Value
		}
		attrValueMap[attr.AttrID] = valueMap
		attrRequiredMap[attr.AttrID] = attr.Required
		attrTypeMap[attr.AttrID] = attr.Type
	}

	// 验证和修复每个AI选择的属性值
	var fixedAttributeData []ResultAttribute
	processedAttrIDs := make(map[int]bool)  // 跟踪已处理的属性ID
	selectedAttrValues := make(map[int]int) // 跟踪已选择的属性值，用于依赖检查

	for _, selectedAttr := range attributeData.AttributeData {
		if len(selectedAttr.AttrValue) == 0 {
			continue
		}

		selectedValue := selectedAttr.AttrValue[0]
		attrID := selectedAttr.AttrID
		processedAttrIDs[attrID] = true

		// 检查属性ID是否存在
		availableValues, exists := attrValueMap[attrID]
		if !exists {
			logger.GetGlobalLogger("shein/product").Warnf("属性ID %d 不在可用列表中，跳过", attrID)
			continue
		}

		selectedValueID := selectedValue.ID.Int()
		fixedAttrValue := selectedValue

		// 记录选择的属性值，用于依赖关系检查
		selectedAttrValues[attrID] = selectedValueID

		// ID为0的自定义值总是有效的（仅当type=0时）
		if selectedValueID == 0 {
			if attrType, typeExists := attrTypeMap[attrID]; typeExists && attrType != 0 {
				logger.GetGlobalLogger("shein/product").Warnf("属性ID %d 类型为%d，不支持自定义值，尝试找到替代值", attrID, attrType)
				// 为非自定义类型找到合适的默认值
				if defaultValue := v.findBestDefaultValue(attrID, selectedValue.Value, availableValues, attributeTemplates); defaultValue != nil {
					fixedAttrValue = *defaultValue
					selectedAttrValues[attrID] = fixedAttrValue.ID.Int()
					logger.GetGlobalLogger("shein/product").Infof("为属性ID %d 找到替代值: ID=%d, Value=%s", attrID, fixedAttrValue.ID.Int(), fixedAttrValue.Value)
				}
			}
		} else if selectedValueID != 0 {
			// 检查选择的值是否在可用列表中
			expectedValue, valueExists := availableValues[selectedValueID]
			if !valueExists {
				logger.GetGlobalLogger("shein/product").Warnf("属性ID %d 的值ID %d 不在可用列表中，尝试修复", attrID, selectedValueID)

				// 尝试找到匹配的值
				if foundValue := v.findMatchingValue(selectedValue.Value, availableValues); foundValue != nil {
					fixedAttrValue = *foundValue
					selectedAttrValues[attrID] = fixedAttrValue.ID.Int()
					logger.GetGlobalLogger("shein/product").Infof("为属性ID %d 找到匹配值: ID=%d, Value=%s", attrID, fixedAttrValue.ID.Int(), fixedAttrValue.Value)
				} else {
					// 找不到匹配值，使用增强版默认策略
					if defaultValue := v.findBestDefaultValue(attrID, selectedValue.Value, availableValues, attributeTemplates); defaultValue != nil {
						fixedAttrValue = *defaultValue
						selectedAttrValues[attrID] = fixedAttrValue.ID.Int()
						logger.GetGlobalLogger("shein/product").Infof("为属性ID %d 使用增强默认值: ID=%d, Value=%s", attrID, fixedAttrValue.ID.Int(), fixedAttrValue.Value)
					}
				}
			} else {
				// 检查Value是否匹配
				if expectedValue != selectedValue.Value {
					logger.GetGlobalLogger("shein/product").Warnf("属性ID %d 的值ID %d 对应的Value不匹配，修复为: %s", attrID, selectedValueID, expectedValue)
					fixedAttrValue = struct {
						ID    types.FlexibleID `json:"id"`
						Value string           `json:"value"`
					}{
						ID:    types.FlexibleID(selectedValueID),
						Value: expectedValue,
					}
				}
			}
		}

		fixedAttributeData = append(fixedAttributeData, ResultAttribute{
			AttrID:    attrID,
			AttrValue: []AttributeValue{fixedAttrValue},
		})
	}

	// 使用增强版依赖关系处理
	v.handleAttributeDependencies(&fixedAttributeData, selectedAttrValues, attrValueMap, processedAttrIDs, attributeTemplates)

	// 检查并添加缺失的必填属性
	for attrID, required := range attrRequiredMap {
		if required && !processedAttrIDs[attrID] {
			availableValues := attrValueMap[attrID]

			// 为必填属性寻找最佳默认值
			if defaultValue := v.findBestDefaultValue(attrID, "", availableValues, attributeTemplates); defaultValue != nil {
				logger.GetGlobalLogger("shein/product").Infof("为必填属性ID %d 添加增强默认值: ID=%d, Value=%s", attrID, defaultValue.ID.Int(), defaultValue.Value)
				fixedAttributeData = append(fixedAttributeData, ResultAttribute{
					AttrID:    attrID,
					AttrValue: []AttributeValue{*defaultValue},
				})
			}
		}
	}

	return AttributeData{
		AttributeData: fixedAttributeData,
	}
}

// handleAttributeDependencies 增强版属性依赖关系处理
func (v *AttributeSelectionValidator) handleAttributeDependencies(fixedAttributeData *[]ResultAttribute, selectedAttrValues map[int]int, attrValueMap map[int]map[int]string, processedAttrIDs map[int]bool, attributeTemplates *attribute.AttributeTemplateInfo) {
	// 定义属性依赖关系
	dependencies := map[int][]int{
		1002187: {1002188, 1002189}, // 主料类型2依赖：当选择主料类型2时，主料克重2和主料克重1变为必填
	}

	for sourceAttrID, dependentAttrIDs := range dependencies {
		// 检查是否选择了源属性
		if selectedValueID, hasSelected := selectedAttrValues[sourceAttrID]; hasSelected && selectedValueID != 0 {
			logger.GetGlobalLogger("shein/product").Infof("检测到属性ID %d 已选择值 %d，检查依赖属性", sourceAttrID, selectedValueID)

			// 为每个依赖属性添加默认值（如果尚未处理）
			for _, dependentAttrID := range dependentAttrIDs {
				if !processedAttrIDs[dependentAttrID] {
					// 检查依赖属性是否可用
					if availableValues, exists := attrValueMap[dependentAttrID]; exists {
						// 使用增强版默认值查找
						defaultValue := v.findBestDefaultValue(dependentAttrID, "", availableValues, attributeTemplates)

						if defaultValue != nil {
							logger.GetGlobalLogger("shein/product").Infof("为依赖属性ID %d 自动添加增强默认值: ID=%d, Value=%s", dependentAttrID, defaultValue.ID.Int(), defaultValue.Value)
							*fixedAttributeData = append(*fixedAttributeData, ResultAttribute{
								AttrID:    dependentAttrID,
								AttrValue: []AttributeValue{*defaultValue},
							})
							processedAttrIDs[dependentAttrID] = true
						}
					}
				}
			}
		}
	}
}

// findBestDefaultValue 增强版默认值查找
func (v *AttributeSelectionValidator) findBestDefaultValue(attrID int, originalValue string, availableValues map[int]string, attributeTemplates *attribute.AttributeTemplateInfo) *AttributeValue {
	// 如果原始值不为空，优先尝试匹配原始值
	if originalValue != "" {
		if matchedValue := v.findMatchingValue(originalValue, availableValues); matchedValue != nil {
			return matchedValue
		}
	}

	// 从属性模板中获取推荐值
	if len(attributeTemplates.Data) > 0 {
		for _, attribute := range attributeTemplates.Data[0].AttributeInfos {
			if attribute.AttributeID == attrID {
				// 如果有备注列表，优先使用备注中的推荐值
				if len(attribute.AttributeRemarkList) > 0 {
					for _, remarkInterface := range attribute.AttributeRemarkList {
						if remark, ok := remarkInterface.(string); ok {
							if matchedValue := v.findMatchingValue(remark, availableValues); matchedValue != nil {
								logger.GetGlobalLogger("shein/product").Infof("使用属性备注推荐值: %s", remark)
								return matchedValue
							}
						}
					}
				}
				break
			}
		}
	}

	// 特殊属性的默认值策略
	switch attrID {
	case 160: // 面料弹性相关
		return v.findElasticityDefaultValue(availableValues)
	case 1001184: // 风格属性
		return v.findStyleDefaultValue(availableValues)
	case 1002188, 1002189: // 主料克重
		// 为克重属性提供合理的默认值
		return &AttributeValue{
			ID:    types.FlexibleID(0),
			Value: "150", // 常见的面料克重
		}
	default:
		return v.findGenericDefaultValue(availableValues)
	}
}

// findMatchingValue 查找匹配的属性值
func (v *AttributeSelectionValidator) findMatchingValue(targetValue string, availableValues map[int]string) *AttributeValue {
	targetLower := strings.ToLower(targetValue)

	for valueID, value := range availableValues {
		valueLower := strings.ToLower(value)
		if valueLower == targetLower || strings.Contains(valueLower, targetLower) || strings.Contains(targetLower, valueLower) {
			return &AttributeValue{
				ID:    types.FlexibleID(valueID),
				Value: value,
			}
		}
	}
	return nil
}

// findElasticityDefaultValue 为面料弹性属性找默认值
func (v *AttributeSelectionValidator) findElasticityDefaultValue(availableValues map[int]string) *AttributeValue {
	// 优先选择面料弹性相关的值
	elasticityKeywords := []string{"无弹", "微弹", "弹力", "弹性", "不弹", "other", "其他"}

	for _, keyword := range elasticityKeywords {
		for valueID, value := range availableValues {
			if strings.Contains(strings.ToLower(value), keyword) {
				return &AttributeValue{
					ID:    types.FlexibleID(valueID),
					Value: value,
				}
			}
		}
	}

	// 如果没找到，使用通用默认值
	return v.findGenericDefaultValue(availableValues)
}

// findStyleDefaultValue 为风格属性找默认值
func (v *AttributeSelectionValidator) findStyleDefaultValue(availableValues map[int]string) *AttributeValue {
	// 优先选择通用风格
	styleKeywords := []string{"休闲", "简约", "基础", "classic", "casual", "simple", "other", "其他"}

	for _, keyword := range styleKeywords {
		for valueID, value := range availableValues {
			if strings.Contains(strings.ToLower(value), keyword) {
				return &AttributeValue{
					ID:    types.FlexibleID(valueID),
					Value: value,
				}
			}
		}
	}

	return v.findGenericDefaultValue(availableValues)
}

// findGenericDefaultValue 查找通用默认值
func (v *AttributeSelectionValidator) findGenericDefaultValue(availableValues map[int]string) *AttributeValue {
	// 优先查找通用默认值
	genericKeywords := []string{"other", "none", "其他", "不适用", "无", "默认"}

	for _, keyword := range genericKeywords {
		for valueID, value := range availableValues {
			lowerValue := strings.ToLower(value)
			if strings.Contains(lowerValue, keyword) {
				return &AttributeValue{
					ID:    types.FlexibleID(valueID),
					Value: value,
				}
			}
		}
	}

	// 如果没找到通用值，使用第一个可用值
	if len(availableValues) > 0 {
		for valueID, value := range availableValues {
			return &AttributeValue{
				ID:    types.FlexibleID(valueID),
				Value: value,
			}
		}
	}

	// 最后选择自定义值
	return &AttributeValue{
		ID:    types.FlexibleID(0),
		Value: "/",
	}
}
