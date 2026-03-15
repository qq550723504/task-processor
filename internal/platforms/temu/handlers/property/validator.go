// Package handlers 提供TEMU平台的各种处理器，包括属性验证等功能
package property

import (
	"fmt"
	"strings"

	models "task-processor/internal/platforms/temu/api/product"
	temucontext "task-processor/internal/platforms/temu/context"
	temutemplate "task-processor/internal/platforms/temu/api/template"

	"github.com/sirupsen/logrus"
)

// PropertyValidator 属性验证器
type PropertyValidator struct {
	logger *logrus.Entry
}

// NewPropertyValidator 创建新的属性验证器
func NewPropertyValidator(logger *logrus.Entry) *PropertyValidator {
	return &PropertyValidator{
		logger: logger,
	}
}

// ValidateProperties 验证属性列表
func (v *PropertyValidator) ValidateProperties(properties []models.PropertyItem, templateProps []temutemplate.GoodsProperty) error {
	v.logger.Info("🔍 开始验证属性列表")

	// 验证属性完整性
	if err := v.validatePropertyCompleteness(properties); err != nil {
		return fmt.Errorf("属性完整性验证失败: %w", err)
	}

	// 验证必填属性
	if err := v.validateRequiredProperties(properties, templateProps); err != nil {
		return fmt.Errorf("必填属性验证失败: %w", err)
	}

	v.logger.Infof("✅ 属性验证通过，共 %d 个属性", len(properties))
	return nil
}

// validatePropertyCompleteness 验证属性完整性
func (v *PropertyValidator) validatePropertyCompleteness(properties []models.PropertyItem) error {
	for i, prop := range properties {
		if prop.Pid == 0 {
			return fmt.Errorf("属性 %d 缺少PID", i)
		}

		if prop.Value == "" {
			v.logger.Warnf("⚠️ 属性 %d (PID=%d) 值为空", i, prop.Pid)
		}
	}

	return nil
}

// validateRequiredProperties 验证必填属性
func (v *PropertyValidator) validateRequiredProperties(properties []models.PropertyItem, templateProps []temutemplate.GoodsProperty) error {
	// 创建已填充属性的映射
	filledMap := make(map[string]bool)
	for _, prop := range properties {
		key := fmt.Sprintf("%d_%d", prop.Pid, prop.RefPid)
		filledMap[key] = true
	}

	// 检查必填属性
	missingRequired := []string{}
	for _, templateProp := range templateProps {
		if templateProp.Required {
			key := fmt.Sprintf("%d_%d", templateProp.PID, templateProp.RefPID)
			if !filledMap[key] {
				missingRequired = append(missingRequired, templateProp.Name)
			}
		}
	}

	if len(missingRequired) > 0 {
		return fmt.Errorf("缺少必填属性: %v", missingRequired)
	}

	return nil
}

// ValidatePropertyValue 验证单个属性值
func (v *PropertyValidator) ValidatePropertyValue(prop models.PropertyItem, templateProp temutemplate.GoodsProperty) error {
	// 验证属性ID匹配
	if prop.Pid != templateProp.PID {
		return fmt.Errorf("属性ID不匹配: 期望 %d，实际 %d", templateProp.PID, prop.Pid)
	}

	// 验证属性值类型
	switch templateProp.PropertyValueType {
	case 1: // 文本类型
		if prop.Value == "" {
			return fmt.Errorf("文本属性值不能为空")
		}
	case 2: // 数值类型
		if prop.Vid == 0 && prop.Value == "" {
			return fmt.Errorf("数值属性必须有值或VID")
		}
	case 3: // 选择类型
		if prop.Vid == 0 {
			return fmt.Errorf("选择类型属性必须有VID")
		}
	}

	return nil
}

// IsPropertyFilled 检查属性是否已填充
func (v *PropertyValidator) IsPropertyFilled(properties []models.PropertyItem, pid, refPid int) bool {
	for _, prop := range properties {
		if prop.Pid == pid && prop.RefPid == refPid {
			return true
		}
	}
	return false
}

// GetPropertyByPID 根据PID获取属性
func (v *PropertyValidator) GetPropertyByPID(properties []models.PropertyItem, pid int) *models.PropertyItem {
	for _, prop := range properties {
		if prop.Pid == pid {
			return &prop
		}
	}
	return nil
}

// ValidateAndFixProperties 验证和修复属性值（集成新的严格验证逻辑）
// 参数:
//   - properties: AI返回的属性列表
//   - data: 属性映射数据
//
// 返回值:
//   - []common.PropertyItem: 验证和修复后的属性列表
func (v *PropertyValidator) ValidateAndFixProperties(properties []models.PropertyItem, data temucontext.PropertyMappingData) []models.PropertyItem {
	v.logger.Info("🔍 开始严格验证和修复AI返回的属性")

	// 创建专门的修复器
	fixer := NewPropertyValueFixer(v.logger)

	// 优先使用最新的模板信息，如果没有则使用映射数据中的模板信息
	templateProps := data.TemuProperties
	if len(templateProps) == 0 {
		v.logger.Warn("⚠️ 映射数据中没有模板属性信息，属性修复可能不准确")
	}

	// 使用新的修复器进行批量修复
	fixedProperties := fixer.FixAllInvalidProperties(properties, templateProps)

	// 应用属性关联过滤规则（基于ShowCondition）
	filteredProperties := v.filterByPropertyRelations(fixedProperties, data)

	// 进行最终验证
	validatedProperties := make([]models.PropertyItem, 0, len(filteredProperties))

	for i, prop := range filteredProperties {
		v.logger.Debugf("最终验证属性 %d: PID=%d, VID=%d, Value=%s", i, prop.Pid, prop.Vid, prop.Value)

		// 基本验证
		if prop.Pid == 0 {
			v.logger.Warnf("⚠️ 属性 %d 缺少PID，跳过", i)
			continue
		}

		// 查找对应的模板属性
		var templateProp *temutemplate.TemplateRespGoodsProperty
		for _, tmplProp := range data.TemuProperties {
			if tmplProp.PID == prop.Pid {
				templateProp = &tmplProp
				break
			}
		}

		if templateProp == nil {
			v.logger.Warnf("⚠️ 未找到PID=%d对应的模板属性，跳过", prop.Pid)
			continue
		}

		// 设置基本信息
		finalProp := prop
		finalProp.RefPid = templateProp.RefPID
		finalProp.TemplatePid = templateProp.TemplatePID

		// 对选择类型属性进行最终的严格验证（包含条件依赖）
		if templateProp.PropertyValueType == 1 {
			validator := NewPropertyValueValidator(v.logger)

			// 检查是否有条件依赖
			if len(templateProp.TemplatePropertyValueParentList) > 0 {
				// 使用条件依赖验证
				isValid, validVID, validValue, err := validator.ValidateSelectionValueWithDependency(finalProp, *templateProp, validatedProperties)

				if !isValid {
					v.logger.Errorf("❌ 条件依赖属性值验证失败: PID=%d, Error=%v", prop.Pid, err)

					// 尝试自动修复：从有效值中选择第一个
					validValues := validator.GetValidValuesWithDependency(*templateProp, validatedProperties)
					if len(validValues) > 0 {
						// 选择最佳默认值
						bestValue := v.selectBestDefaultValueFromList(validValues)
						finalProp.Vid = bestValue.VID
						finalProp.Value = bestValue.Value
						v.logger.Warnf("🔧 自动修复条件依赖属性: PID=%d, 使用默认值 '%s' (VID=%d)",
							prop.Pid, bestValue.Value, bestValue.VID)
					} else {
						v.logger.Errorf("❌ 无法修复条件依赖属性: PID=%d, 没有有效值", prop.Pid)
						continue
					}
				} else {
					// 确保使用验证通过的值
					finalProp.Vid = validVID
					finalProp.Value = validValue
				}
			} else {
				// 使用普通验证
				isValid, validVID, validValue, err := validator.ValidateSelectionValue(finalProp, *templateProp)

				if !isValid {
					v.logger.Errorf("❌ 属性值最终验证失败: PID=%d, Error=%v", prop.Pid, err)
					continue
				}

				// 确保使用验证通过的值
				finalProp.Vid = validVID
				finalProp.Value = validValue
			}
		}

		validatedProperties = append(validatedProperties, finalProp)
		v.logger.Debugf("✅ 属性最终验证通过: PID=%d, VID=%d, Value=%s",
			finalProp.Pid, finalProp.Vid, finalProp.Value)
	}

	v.logger.Infof("✅ 严格属性验证完成，有效属性: %d/%d", len(validatedProperties), len(properties))
	return validatedProperties
}

// fixPropertyValue 修复单个属性值
func (v *PropertyValidator) fixPropertyValue(prop models.PropertyItem, templateProp temutemplate.GoodsProperty) *models.PropertyItem {
	fixedProp := prop

	// 设置基本信息
	fixedProp.RefPid = templateProp.RefPID
	fixedProp.TemplatePid = templateProp.TemplatePID

	// 根据属性类型进行验证和修复
	switch templateProp.PropertyValueType {
	case 1: // 选择类型
		return v.fixSelectionProperty(fixedProp, templateProp)
	case 2: // 数值类型
		return v.fixNumericProperty(fixedProp, templateProp)
	case 3: // 文本类型
		return v.fixTextProperty(fixedProp, templateProp)
	default:
		v.logger.Warnf("⚠️ 未知属性类型: %d", templateProp.PropertyValueType)
		return &fixedProp
	}
}

// fixSelectionProperty 修复选择类型属性
func (v *PropertyValidator) fixSelectionProperty(prop models.PropertyItem, templateProp temutemplate.GoodsProperty) *models.PropertyItem {
	// 如果已有有效的VID，验证是否在候选列表中
	if prop.Vid != 0 {
		for _, value := range templateProp.Values {
			if value.VID == prop.Vid {
				prop.Value = value.Value // 确保Value与VID匹配
				return &prop
			}
		}
	}

	// 如果有Value但没有VID，尝试匹配
	if prop.Value != "" {
		for _, value := range templateProp.Values {
			if value.Value == prop.Value {
				prop.Vid = value.VID
				return &prop
			}
		}
	}

	// 如果都没有匹配，选择默认值
	if len(templateProp.Values) > 0 {
		defaultValue := v.selectBestDefaultValue(templateProp)
		prop.Vid = defaultValue.VID
		prop.Value = defaultValue.Value
		v.logger.Debugf("🔧 选择类型属性使用默认值: %s (VID=%d)", prop.Value, prop.Vid)
		return &prop
	}

	v.logger.Warnf("⚠️ 选择类型属性无候选值: PID=%d", prop.Pid)
	return nil
}

// fixNumericProperty 修复数值类型属性
func (v *PropertyValidator) fixNumericProperty(prop models.PropertyItem, templateProp temutemplate.GoodsProperty) *models.PropertyItem {
	// 如果有候选值列表，按选择类型处理
	if len(templateProp.Values) > 0 {
		return v.fixSelectionProperty(prop, templateProp)
	}

	// 纯数值类型，验证范围
	if prop.Value == "" {
		if templateProp.MinValue != "" {
			prop.Value = templateProp.MinValue
		} else {
			prop.Value = "1"
		}
		v.logger.Debugf("🔧 数值类型属性使用默认值: %s", prop.Value)
	}

	// 设置单位
	if len(templateProp.ValueUnit) > 0 {
		prop.ValueUnit = templateProp.ValueUnit[0]
	}

	return &prop
}

// fixTextProperty 修复文本类型属性
func (v *PropertyValidator) fixTextProperty(prop models.PropertyItem, templateProp temutemplate.GoodsProperty) *models.PropertyItem {
	// 如果有候选值列表，按选择类型处理
	if len(templateProp.Values) > 0 {
		return v.fixSelectionProperty(prop, templateProp)
	}

	// 纯文本类型
	if prop.Value == "" {
		prop.Value = "Not specified"
		v.logger.Debugf("🔧 文本类型属性使用默认值: %s", prop.Value)
	}

	return &prop
}

// selectBestDefaultValue 选择最佳的默认值（优先选择英文中性选项）
func (v *PropertyValidator) selectBestDefaultValue(templateProp temutemplate.GoodsProperty) temutemplate.PropertyValue {
	// 优先选择的英文关键词
	englishNeutralKeywords := []string{
		"Other", "N/A", "None", "Not Applicable", "No", "Without",
		"Mixed", "General", "Universal", "Standard",
	}

	// 中文关键词作为备选
	chineseNeutralKeywords := []string{
		"其他", "其它", "不适用", "无需", "混合", "通用",
	}

	// 首先尝试找到包含英文中性关键词的选项
	for _, keyword := range englishNeutralKeywords {
		for _, value := range templateProp.Values {
			if strings.Contains(value.Value, keyword) {
				return value
			}
		}
	}

	// 尝试找中文中性选项
	for _, keyword := range chineseNeutralKeywords {
		for _, value := range templateProp.Values {
			if strings.Contains(value.Value, keyword) {
				return value
			}
		}
	}

	// 返回第一个可选值
	return templateProp.Values[0]
}

// selectBestDefaultValueFromList 从给定的值列表中选择最佳默认值
func (v *PropertyValidator) selectBestDefaultValueFromList(values []temutemplate.PropertyValue) temutemplate.PropertyValue {
	// 优先选择的英文关键词
	englishNeutralKeywords := []string{
		"Other", "N/A", "None", "Not Applicable", "No", "Without",
		"Mixed", "General", "Universal", "Standard",
	}

	// 中文关键词作为备选
	chineseNeutralKeywords := []string{
		"其他", "其它", "不适用", "无需", "混合", "通用",
	}

	// 首先尝试找到包含英文中性关键词的选项
	for _, keyword := range englishNeutralKeywords {
		for _, value := range values {
			if strings.Contains(value.Value, keyword) {
				return value
			}
		}
	}

	// 尝试找中文中性选项
	for _, keyword := range chineseNeutralKeywords {
		for _, value := range values {
			if strings.Contains(value.Value, keyword) {
				return value
			}
		}
	}

	// 返回第一个可选值
	return values[0]
}
