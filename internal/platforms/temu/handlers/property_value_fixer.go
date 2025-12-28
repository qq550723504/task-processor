// Package handlers 提供TEMU平台的属性值修复功能
package handlers

import (
	"strings"

	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// PropertyValueFixer 属性值修复器 - 专门修复无效的属性值
type PropertyValueFixer struct {
	logger    *logrus.Entry
	validator *PropertyValueValidator
}

// NewPropertyValueFixer 创建新的属性值修复器
func NewPropertyValueFixer(logger *logrus.Entry) *PropertyValueFixer {
	return &PropertyValueFixer{
		logger:    logger,
		validator: NewPropertyValueValidator(logger),
	}
}

// FixInvalidSelectionValue 修复无效的选择类型属性值
func (f *PropertyValueFixer) FixInvalidSelectionValue(
	prop types.PropertyItem,
	templateProp types.TemplateRespGoodsProperty,
) *types.PropertyItem {

	f.logger.Debugf("🔧 开始修复属性值: PID=%d, Value='%s', VID=%d",
		prop.Pid, prop.Value, prop.Vid)

	// 先验证当前值是否有效
	isValid, validVID, validValue, _ := f.validator.ValidateSelectionValue(prop, templateProp)
	if isValid {
		// 值有效，直接返回
		fixedProp := prop
		fixedProp.Vid = validVID
		fixedProp.Value = validValue
		f.logger.Infof("✅ 属性值验证通过: PID=%d, Value='%s', VID=%d",
			fixedProp.Pid, fixedProp.Value, fixedProp.Vid)
		return &fixedProp
	}

	// 值无效，需要修复
	f.logger.Warnf("⚠️ 检测到无效属性值，开始修复: PID=%d, 属性名='%s', 当前值='%s', 当前VID=%d",
		prop.Pid, templateProp.Name, prop.Value, prop.Vid)

	// 尝试智能匹配
	if matchedValue := f.tryIntelligentMatch(prop, templateProp); matchedValue != nil {
		fixedProp := prop
		fixedProp.Vid = matchedValue.VID
		fixedProp.Value = matchedValue.Value
		f.logger.Infof("🎯 智能匹配成功: PID=%d, '%s' (VID=%d) → '%s' (VID=%d)",
			prop.Pid, prop.Value, prop.Vid, matchedValue.Value, matchedValue.VID)
		return &fixedProp
	}

	// 智能匹配失败，使用默认值
	defaultValue := f.selectBestDefaultValue(templateProp)
	fixedProp := prop
	fixedProp.Vid = defaultValue.VID
	fixedProp.Value = defaultValue.Value

	f.logger.Warnf("🔧 智能匹配失败，使用默认值修复: PID=%d, '%s' (VID=%d) → '%s' (VID=%d)",
		prop.Pid, prop.Value, prop.Vid, defaultValue.Value, defaultValue.VID)

	return &fixedProp
}

// tryIntelligentMatch 尝试智能匹配属性值
func (f *PropertyValueFixer) tryIntelligentMatch(
	prop types.PropertyItem,
	templateProp types.TemplateRespGoodsProperty,
) *types.PropertyValue {

	if prop.Value == "" {
		return nil
	}

	propValueLower := strings.ToLower(prop.Value)

	// 1. 精确匹配（忽略大小写）
	for _, validValue := range templateProp.Values {
		if strings.ToLower(validValue.Value) == propValueLower {
			f.logger.Debugf("🎯 精确匹配: '%s' → '%s'", prop.Value, validValue.Value)
			return &validValue
		}
	}

	// 2. 包含匹配
	for _, validValue := range templateProp.Values {
		validValueLower := strings.ToLower(validValue.Value)
		if strings.Contains(validValueLower, propValueLower) ||
			strings.Contains(propValueLower, validValueLower) {
			f.logger.Debugf("🎯 包含匹配: '%s' → '%s'", prop.Value, validValue.Value)
			return &validValue
		}
	}

	// 3. 材质属性特殊匹配
	if f.isMaterialProperty(templateProp.Name) {
		return f.matchMaterialValue(prop.Value, templateProp.Values)
	}

	// 4. 颜色属性特殊匹配
	if f.isColorProperty(templateProp.Name) {
		return f.matchColorValue(prop.Value, templateProp.Values)
	}

	return nil
}

// matchMaterialValue 材质属性特殊匹配
func (f *PropertyValueFixer) matchMaterialValue(value string, validValues []types.PropertyValue) *types.PropertyValue {
	valueLower := strings.ToLower(value)

	// 材质关键词映射
	materialKeywords := map[string][]string{
		"steel":   {"钢", "不锈钢", "金属", "steel", "stainless"},
		"plastic": {"塑料", "PP", "ABS", "plastic", "polymer"},
		"wood":    {"木", "木质", "竹", "wood", "bamboo"},
		"glass":   {"玻璃", "glass"},
		"fabric":  {"布", "纺织", "fabric", "textile", "cloth"},
		"leather": {"皮", "皮革", "leather"},
		"ceramic": {"陶瓷", "ceramic"},
		"rubber":  {"橡胶", "rubber"},
	}

	// 查找匹配的材质
	for material, keywords := range materialKeywords {
		for _, keyword := range keywords {
			if strings.Contains(valueLower, keyword) {
				// 在有效值中查找对应的材质
				for _, validValue := range validValues {
					validValueLower := strings.ToLower(validValue.Value)
					if strings.Contains(validValueLower, material) ||
						strings.Contains(validValueLower, keyword) {
						f.logger.Debugf("🎯 材质匹配: '%s' → '%s'", value, validValue.Value)
						return &validValue
					}
				}
			}
		}
	}

	return nil
}

// matchColorValue 颜色属性特殊匹配
func (f *PropertyValueFixer) matchColorValue(value string, validValues []types.PropertyValue) *types.PropertyValue {
	valueLower := strings.ToLower(value)

	// 颜色关键词映射
	colorKeywords := map[string][]string{
		"black":  {"黑", "black"},
		"white":  {"白", "white"},
		"red":    {"红", "red"},
		"blue":   {"蓝", "blue"},
		"green":  {"绿", "green"},
		"yellow": {"黄", "yellow"},
		"gray":   {"灰", "gray", "grey"},
		"brown":  {"棕", "brown"},
		"pink":   {"粉", "pink"},
		"purple": {"紫", "purple"},
	}

	// 查找匹配的颜色
	for color, keywords := range colorKeywords {
		for _, keyword := range keywords {
			if strings.Contains(valueLower, keyword) {
				// 在有效值中查找对应的颜色
				for _, validValue := range validValues {
					validValueLower := strings.ToLower(validValue.Value)
					if strings.Contains(validValueLower, color) ||
						strings.Contains(validValueLower, keyword) {
						f.logger.Debugf("🎯 颜色匹配: '%s' → '%s'", value, validValue.Value)
						return &validValue
					}
				}
			}
		}
	}

	return nil
}

// selectBestDefaultValue 选择最佳的默认值
func (f *PropertyValueFixer) selectBestDefaultValue(templateProp types.TemplateRespGoodsProperty) types.PropertyValue {
	if len(templateProp.Values) == 0 {
		f.logger.Error("❌ 属性没有可选值列表")
		return types.PropertyValue{}
	}

	// 优先选择的中性关键词（按优先级排序）
	neutralKeywords := []string{
		// 英文中性词
		"Other", "N/A", "None", "Not Applicable", "No", "Without",
		"Mixed", "General", "Universal", "Standard", "Default",
		"Unspecified", "Not specified", "Various", "Multiple",

		// 中文中性词
		"其他", "其它", "不适用", "无需", "混合", "通用", "标准",
		"未指定", "不指定", "多种", "各种", "无", "否",
	}

	// 按优先级查找中性选项
	for _, keyword := range neutralKeywords {
		for _, validValue := range templateProp.Values {
			if strings.Contains(validValue.Value, keyword) {
				f.logger.Debugf("🎯 选择中性默认值: '%s' (VID=%d)",
					validValue.Value, validValue.VID)
				return validValue
			}
		}
	}

	// 如果没找到中性选项，选择第一个
	defaultValue := templateProp.Values[0]
	f.logger.Debugf("🎯 选择第一个默认值: '%s' (VID=%d)",
		defaultValue.Value, defaultValue.VID)
	return defaultValue
}

// isMaterialProperty 判断是否为材质属性
func (f *PropertyValueFixer) isMaterialProperty(propertyName string) bool {
	materialNames := []string{"材质", "Material", "material", "材料"}
	propertyNameLower := strings.ToLower(propertyName)

	for _, name := range materialNames {
		if strings.Contains(propertyNameLower, strings.ToLower(name)) {
			return true
		}
	}
	return false
}

// isColorProperty 判断是否为颜色属性
func (f *PropertyValueFixer) isColorProperty(propertyName string) bool {
	colorNames := []string{"颜色", "Color", "color", "色彩"}
	propertyNameLower := strings.ToLower(propertyName)

	for _, name := range colorNames {
		if strings.Contains(propertyNameLower, strings.ToLower(name)) {
			return true
		}
	}
	return false
}

// FixAllInvalidProperties 批量修复所有无效属性
func (f *PropertyValueFixer) FixAllInvalidProperties(
	properties []types.PropertyItem,
	templateProps []types.TemplateRespGoodsProperty,
) []types.PropertyItem {

	f.logger.Info("🔧 开始批量修复无效属性值")

	// 创建模板属性映射
	templateMap := make(map[int]types.TemplateRespGoodsProperty)
	for _, tmpl := range templateProps {
		templateMap[tmpl.PID] = tmpl
	}

	fixedProperties := make([]types.PropertyItem, 0, len(properties))
	fixedCount := 0

	// 修复每个属性
	for _, prop := range properties {
		templateProp, exists := templateMap[prop.Pid]
		if !exists {
			// 模板中不存在的属性，跳过
			continue
		}

		// 只修复选择类型属性
		if templateProp.PropertyValueType != 1 {
			fixedProperties = append(fixedProperties, prop)
			continue
		}

		// 修复选择类型属性
		fixedProp := f.FixInvalidSelectionValue(prop, templateProp)
		if fixedProp != nil {
			fixedProperties = append(fixedProperties, *fixedProp)
			if fixedProp.Value != prop.Value || fixedProp.Vid != prop.Vid {
				fixedCount++
			}
		} else if templateProp.Required {
			// 🚨 必填属性不能跳过，强制提供默认值
			f.logger.Warnf("🚨 必填属性修复失败，强制使用默认值: %s (PID=%d)", templateProp.Name, templateProp.PID)
			defaultProp := f.forceCreateValidProperty(prop, templateProp)
			if defaultProp != nil {
				fixedProperties = append(fixedProperties, *defaultProp)
				fixedCount++
			}
		}
	}

	f.logger.Infof("✅ 属性修复完成: 总数=%d, 修复=%d", len(properties), fixedCount)
	return fixedProperties
}

// forceCreateValidProperty 强制为必填属性创建有效值
func (f *PropertyValueFixer) forceCreateValidProperty(
	prop types.PropertyItem,
	templateProp types.TemplateRespGoodsProperty,
) *types.PropertyItem {
	f.logger.Warnf("🚨 强制为必填属性创建有效值: %s (PID=%d)", templateProp.Name, templateProp.PID)

	if len(templateProp.Values) == 0 {
		f.logger.Errorf("❌ 必填属性没有可选值列表: %s", templateProp.Name)
		return nil
	}

	// 选择最佳默认值
	defaultValue := f.selectBestDefaultValue(templateProp)

	validProp := prop
	validProp.Vid = defaultValue.VID
	validProp.Value = defaultValue.Value

	f.logger.Infof("✅ 必填属性强制修复完成: %s → '%s' (VID=%d)",
		templateProp.Name, defaultValue.Value, defaultValue.VID)

	return &validProp
}
