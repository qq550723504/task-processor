// Package handlers 提供TEMU平台的各种处理器，包括默认属性填充等功能
package handlers

import (
	"fmt"
	"strconv"

	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// DefaultPropertyFiller 默认属性填充器
type DefaultPropertyFiller struct {
	logger       *logrus.Entry
	deduplicator *PropertyDeduplicator
}

// NewDefaultPropertyFiller 创建新的默认属性填充器
func NewDefaultPropertyFiller(logger *logrus.Entry) *DefaultPropertyFiller {
	return &DefaultPropertyFiller{
		logger:       logger,
		deduplicator: NewPropertyDeduplicator(logger),
	}
}

// FillRequiredPropertiesWithDefaults 为所有必填属性填充默认值
func (f *DefaultPropertyFiller) FillRequiredPropertiesWithDefaults(templateProps []types.TemplateRespGoodsProperty, ext *types.ExtensionInfo) {
	f.logger.Info("🔧 开始为必填属性填充默认值")

	filledCount := 0
	for _, templateProp := range templateProps {
		if templateProp.Required {
			// 检查是否已经填充
			if !f.isPropertyAlreadyFilled(templateProp, ext) {
				f.FillSingleRequiredProperty(templateProp, ext)
				filledCount++
			}
		}
	}

	// 特殊处理：验证和修正面料成分总和
	f.validateAndFixMaterialComposition(templateProps, ext)

	// 填充完成后进行去重
	ext.GoodsProperty.GoodsProperties = f.deduplicator.DeduplicateByPidOnly(ext.GoodsProperty.GoodsProperties)

	f.logger.Infof("✅ 默认值填充和去重完成，共填充 %d 个必填属性", filledCount)
}

// FillSingleRequiredProperty 填充单个必填属性
func (f *DefaultPropertyFiller) FillSingleRequiredProperty(templateProp types.TemplateRespGoodsProperty, ext *types.ExtensionInfo) {
	defaultValue := f.getDefaultValueForProperty(templateProp)

	if defaultValue != nil {
		ext.GoodsProperty.GoodsProperties = append(ext.GoodsProperty.GoodsProperties, *defaultValue)
		f.logger.Infof("✅ 已填充必填属性: %s (PID=%d, RefPID=%d, VID=%d, Value=%s)",
			templateProp.Name, defaultValue.Pid, defaultValue.RefPid, defaultValue.Vid, defaultValue.Value)
	} else {
		f.logger.Warnf("⚠️ 无法为必填属性生成默认值: %s (PID=%d, RefPID=%d)",
			templateProp.Name, templateProp.PID, templateProp.RefPID)
	}
}

// getDefaultValueForProperty 获取属性的默认值
func (f *DefaultPropertyFiller) getDefaultValueForProperty(templateProp types.TemplateRespGoodsProperty) *types.PropertyItem {
	// 特殊处理control_type为16的属性（需要数值输入的选择类型）
	if templateProp.ControlType == 16 {
		return f.getDefaultNumberInputSelectionValue(templateProp)
	}

	// 处理PropertyValueType为0但有Values的属性（通常是选择类型）
	if templateProp.PropertyValueType == 0 && len(templateProp.Values) > 0 {
		f.logger.Infof("🔧 处理PropertyValueType=0但有Values的属性: %s", templateProp.Name)
		return f.getDefaultSelectionValue(templateProp)
	}

	switch templateProp.PropertyValueType {
	case 1: // 文本类型
		return f.getDefaultTextValue(templateProp)
	case 2: // 数值类型
		return f.getDefaultNumericValue(templateProp)
	case 3: // 选择类型
		return f.getDefaultSelectionValue(templateProp)
	default:
		f.logger.Warnf("⚠️ 未知的属性值类型: %d，尝试作为选择类型处理", templateProp.PropertyValueType)
		// 如果有可选值，尝试作为选择类型处理
		if len(templateProp.Values) > 0 {
			return f.getDefaultSelectionValue(templateProp)
		}
		return nil
	}
}

// getDefaultTextValue 获取文本类型的默认值
func (f *DefaultPropertyFiller) getDefaultTextValue(templateProp types.TemplateRespGoodsProperty) *types.PropertyItem {
	defaultText := f.generateDefaultTextByName(templateProp.Name)

	return &types.PropertyItem{
		Pid:    templateProp.PID,
		RefPid: templateProp.RefPID,
		Vid:    0, // 文本类型通常不需要VID
		Value:  defaultText,
	}
}

// getDefaultNumericValue 获取数值类型的默认值
func (f *DefaultPropertyFiller) getDefaultNumericValue(templateProp types.TemplateRespGoodsProperty) *types.PropertyItem {
	defaultNumber := f.generateDefaultNumberByName(templateProp.Name)

	return &types.PropertyItem{
		Pid:    templateProp.PID,
		RefPid: templateProp.RefPID,
		Vid:    0,
		Value:  strconv.Itoa(defaultNumber),
	}
}

// getDefaultSelectionValue 获取选择类型的默认值
func (f *DefaultPropertyFiller) getDefaultSelectionValue(templateProp types.TemplateRespGoodsProperty) *types.PropertyItem {
	// 优先使用第一个候选值
	if len(templateProp.Values) > 0 {
		// 如果有父条件依赖，需要选择匹配的值
		selectedValue := f.selectValueBasedOnParentCondition(templateProp)
		if selectedValue != nil {
			return &types.PropertyItem{
				Pid:              templateProp.PID,
				RefPid:           templateProp.RefPID,
				TemplatePid:      templateProp.TemplatePID,
				TemplateModuleID: templateProp.TemplateModuleID,
				Vid:              selectedValue.VID,
				Value:            selectedValue.Value,
			}
		}

		// 如果没有找到匹配的值，使用第一个候选值
		firstCandidate := templateProp.Values[0]
		return &types.PropertyItem{
			Pid:              templateProp.PID,
			RefPid:           templateProp.RefPID,
			TemplatePid:      templateProp.TemplatePID,
			TemplateModuleID: templateProp.TemplateModuleID,
			Vid:              firstCandidate.VID,
			Value:            firstCandidate.Value,
		}
	}

	// 如果没有候选值，生成通用默认值
	return &types.PropertyItem{
		Pid:              templateProp.PID,
		RefPid:           templateProp.RefPID,
		TemplatePid:      templateProp.TemplatePID,
		TemplateModuleID: templateProp.TemplateModuleID,
		Vid:              1, // 通用默认VID
		Value:            "Default",
	}
}

// selectValueBasedOnParentCondition 根据父条件选择合适的值
func (f *DefaultPropertyFiller) selectValueBasedOnParentCondition(templateProp types.TemplateRespGoodsProperty) *types.PropertyValue {
	// 如果没有父条件依赖，返回nil
	if len(templateProp.TemplatePropertyValueParentList) == 0 {
		return nil
	}

	f.logger.Infof("🔍 为属性 %s 根据父条件选择合适的值", templateProp.Name)

	// 这里简化处理：选择第一个有效的值
	// 在实际应用中，应该根据已填充的父属性值来选择
	for _, parentList := range templateProp.TemplatePropertyValueParentList {
		if len(parentList.VIDs) > 0 {
			// 查找对应的值
			for _, value := range templateProp.Values {
				for _, vid := range parentList.VIDs {
					if value.VID == vid {
						f.logger.Infof("✅ 选择了匹配父条件的值: %s (vid=%d)", value.Value, value.VID)
						return &value
					}
				}
			}
		}
	}

	return nil
}

// getDefaultNumberInputSelectionValue 获取需要数值输入的选择类型默认值（control_type: 16）
func (f *DefaultPropertyFiller) getDefaultNumberInputSelectionValue(templateProp types.TemplateRespGoodsProperty) *types.PropertyItem {
	f.logger.Infof("🔢 处理数值输入选择类型属性: %s (control_type: %d)", templateProp.Name, templateProp.ControlType)

	// 选择一个合适的材料类型
	var selectedValue types.PropertyValue
	if len(templateProp.Values) > 0 {
		// 对于面料成分，优先选择常见材料
		selectedValue = f.selectBestMaterialValue(templateProp)
	} else {
		f.logger.Warnf("⚠️ 属性 %s 没有可选值列表", templateProp.Name)
		return nil
	}

	// 获取默认的数值输入值
	defaultNumberValue := f.getDefaultNumberInputValue(templateProp)

	// 获取单位
	var valueUnit string
	if len(templateProp.ValueUnit) > 0 {
		valueUnit = templateProp.ValueUnit[0]
	} else if len(templateProp.ValueUnitDTOList) > 0 {
		valueUnit = templateProp.ValueUnitDTOList[0].ValueUnit
	}

	propertyItem := &types.PropertyItem{
		Pid:              templateProp.PID,
		RefPid:           templateProp.RefPID,
		TemplatePid:      templateProp.TemplatePID,
		TemplateModuleID: templateProp.TemplateModuleID,
		Vid:              selectedValue.VID,
		Value:            selectedValue.Value,
		NumberInputValue: defaultNumberValue,
		ValueUnit:        valueUnit,
	}

	f.logger.Infof("✅ 生成数值输入选择属性: %s=%s, 数值=%s%s (VID=%d)",
		templateProp.Name, selectedValue.Value, defaultNumberValue, valueUnit, selectedValue.VID)

	return propertyItem
}

// selectBestMaterialValue 选择最佳的材料值
func (f *DefaultPropertyFiller) selectBestMaterialValue(templateProp types.TemplateRespGoodsProperty) types.PropertyValue {
	// 对于面料成分，优先选择常见材料
	preferredMaterials := []string{"棉", "涤纶", "尼龙", "氨纶", "聚酯纤维"}

	// 首先尝试匹配优选材料
	for _, preferred := range preferredMaterials {
		for _, value := range templateProp.Values {
			if value.Value == preferred {
				f.logger.Infof("🎯 选择优选材料: %s (VID=%d)", value.Value, value.VID)
				return value
			}
		}
	}

	// 如果没有匹配的优选材料，使用第一个可用值
	if len(templateProp.Values) > 0 {
		firstValue := templateProp.Values[0]
		f.logger.Infof("📝 使用第一个可用材料: %s (VID=%d)", firstValue.Value, firstValue.VID)
		return firstValue
	}

	// 兜底情况
	return types.PropertyValue{VID: 1, Value: "其他"}
}

// getDefaultNumberInputValue 获取默认的数值输入值
func (f *DefaultPropertyFiller) getDefaultNumberInputValue(templateProp types.TemplateRespGoodsProperty) string {
	// 根据属性名称和约束生成合适的默认值
	switch templateProp.Name {
	case "面料成分":
		// 面料成分需要确保总和为100%
		// 这里先填充100%，如果有多个材料，需要在后续逻辑中调整分配
		return "100"
	default:
		// 检查是否有最大值约束
		if templateProp.MaxValue != "" {
			return templateProp.MaxValue
		}
		// 检查是否有最小值约束
		if templateProp.MinValue != "" {
			return templateProp.MinValue
		}
		// 默认值
		return "1"
	}
}

// generateDefaultTextByName 根据属性名生成默认文本
func (f *DefaultPropertyFiller) generateDefaultTextByName(name string) string {
	// 根据常见属性名生成合适的默认值
	switch name {
	case "品牌", "Brand", "brand":
		return "No Brand"
	case "型号", "Model", "model":
		return "General"
	case "颜色", "Color", "color":
		return "mulite-color"
	case "尺寸", "Size", "size":
		return "one-size"
	case "材质", "Material", "material":
		return "other"
	case "产地", "Origin", "origin":
		return "China"
	default:
		return fmt.Sprintf("Default%s", name)
	}
}

// generateDefaultNumberByName 根据属性名生成默认数值
func (f *DefaultPropertyFiller) generateDefaultNumberByName(name string) int {
	// 根据常见属性名生成合适的默认数值
	switch name {
	case "重量", "Weight", "weight":
		return 100 // 100g
	case "长度", "Length", "length":
		return 10 // 10cm
	case "宽度", "Width", "width":
		return 10 // 10cm
	case "高度", "Height", "height":
		return 10 // 10cm
	case "数量", "Quantity", "quantity":
		return 1
	default:
		return 1
	}
}

// isPropertyAlreadyFilled 检查属性是否已经填充
func (f *DefaultPropertyFiller) isPropertyAlreadyFilled(templateProp types.TemplateRespGoodsProperty, ext *types.ExtensionInfo) bool {
	for _, prop := range ext.GoodsProperty.GoodsProperties {
		if prop.Pid == templateProp.PID && prop.RefPid == templateProp.RefPID {
			return true
		}
	}
	return false
}

// validateAndFixMaterialComposition 验证和修正面料成分总和
func (f *DefaultPropertyFiller) validateAndFixMaterialComposition(templateProps []types.TemplateRespGoodsProperty, ext *types.ExtensionInfo) {
	// 找到面料成分属性
	var materialCompositionTemplate *types.TemplateRespGoodsProperty
	for _, templateProp := range templateProps {
		if templateProp.Name == "面料成分" && templateProp.ControlType == 16 {
			materialCompositionTemplate = &templateProp
			break
		}
	}

	if materialCompositionTemplate == nil {
		return // 没有面料成分属性
	}

	// 收集所有面料成分属性项
	var materialProps []*types.PropertyItem
	for i := range ext.GoodsProperty.GoodsProperties {
		prop := &ext.GoodsProperty.GoodsProperties[i]
		if prop.Pid == materialCompositionTemplate.PID && prop.RefPid == materialCompositionTemplate.RefPID {
			materialProps = append(materialProps, prop)
		}
	}

	if len(materialProps) == 0 {
		return // 没有面料成分数据
	}

	// 计算当前总和
	totalPercentage := f.calculateMaterialCompositionTotal(materialProps)
	f.logger.Infof("📊 当前面料成分总和: %d%%", totalPercentage)

	// 如果总和不是100%，进行修正
	if totalPercentage != 100 {
		f.fixMaterialCompositionTotal(materialProps, totalPercentage)
	}
}

// calculateMaterialCompositionTotal 计算面料成分总和
func (f *DefaultPropertyFiller) calculateMaterialCompositionTotal(materialProps []*types.PropertyItem) int {
	total := 0
	for _, prop := range materialProps {
		if prop.NumberInputValue != "" {
			if percentage, err := strconv.Atoi(prop.NumberInputValue); err == nil {
				total += percentage
			}
		}
	}
	return total
}

// fixMaterialCompositionTotal 修正面料成分总和为100%
func (f *DefaultPropertyFiller) fixMaterialCompositionTotal(materialProps []*types.PropertyItem, currentTotal int) {
	if len(materialProps) == 0 {
		return
	}

	f.logger.Infof("🔧 修正面料成分总和: %d%% -> 100%%", currentTotal)

	if len(materialProps) == 1 {
		// 只有一个材料，直接设置为100%
		materialProps[0].NumberInputValue = "100"
		f.logger.Infof("✅ 单一材料修正为100%%: %s", materialProps[0].Value)
	} else {
		// 多个材料，按比例调整
		if currentTotal > 0 {
			// 按比例缩放到100%
			remaining := 100
			for i, prop := range materialProps {
				if prop.NumberInputValue != "" {
					if currentPercentage, err := strconv.Atoi(prop.NumberInputValue); err == nil {
						if i == len(materialProps)-1 {
							// 最后一个材料使用剩余百分比
							prop.NumberInputValue = strconv.Itoa(remaining)
						} else {
							// 按比例计算
							newPercentage := (currentPercentage * 100) / currentTotal
							if newPercentage < 1 {
								newPercentage = 1 // 最小1%
							}
							prop.NumberInputValue = strconv.Itoa(newPercentage)
							remaining -= newPercentage
						}
						f.logger.Infof("✅ 材料 %s 调整为: %s%%", prop.Value, prop.NumberInputValue)
					}
				}
			}
		} else {
			// 当前总和为0，平均分配
			averagePercentage := 100 / len(materialProps)
			remaining := 100 - (averagePercentage * (len(materialProps) - 1))

			for i, prop := range materialProps {
				if i == len(materialProps)-1 {
					prop.NumberInputValue = strconv.Itoa(remaining)
				} else {
					prop.NumberInputValue = strconv.Itoa(averagePercentage)
				}
				f.logger.Infof("✅ 材料 %s 平均分配为: %s%%", prop.Value, prop.NumberInputValue)
			}
		}
	}
}
