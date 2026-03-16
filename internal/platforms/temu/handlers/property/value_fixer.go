// Package property 提供TEMU平台的属性值修复功能
package property

import (
	"strings"

	models "task-processor/internal/platforms/temu/api/product"
	temutemplate "task-processor/internal/platforms/temu/api/template"

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
	prop models.PropertyItem,
	templateProp temutemplate.TemplateRespGoodsProperty,
) *models.PropertyItem {

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
	prop models.PropertyItem,
	templateProp temutemplate.TemplateRespGoodsProperty,
) *temutemplate.PropertyValue {

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

	// 3. 通用关键词匹配（基于属性值内容，而非属性名称）
	return f.matchByValueContent(prop.Value, templateProp.Values)
}

// matchByValueContent 基于值内容进行通用匹配
func (f *PropertyValueFixer) matchByValueContent(value string, validValues []temutemplate.PropertyValue) *temutemplate.PropertyValue {
	valueLower := strings.ToLower(value)

	// 通用关键词匹配 - 基于值的实际内容而非属性名称
	for _, validValue := range validValues {
		validValueLower := strings.ToLower(validValue.Value)

		// 检查是否有共同的关键词
		if f.hasCommonKeywords(valueLower, validValueLower) {
			f.logger.Debugf("🎯 关键词匹配: '%s' → '%s'", value, validValue.Value)
			return &validValue
		}
	}

	return nil
}

// hasCommonKeywords 检查两个字符串是否有共同的关键词
func (f *PropertyValueFixer) hasCommonKeywords(value1, value2 string) bool {
	// 提取关键词（长度大于2的单词）
	words1 := f.extractKeywords(value1)
	words2 := f.extractKeywords(value2)

	// 检查是否有共同关键词
	for _, word1 := range words1 {
		for _, word2 := range words2 {
			if strings.Contains(word1, word2) || strings.Contains(word2, word1) {
				return true
			}
		}
	}

	return false
}

// extractKeywords 提取关键词
func (f *PropertyValueFixer) extractKeywords(text string) []string {
	words := strings.Fields(text)
	keywords := make([]string, 0, len(words))

	for _, word := range words {
		// 只保留长度大于2的词作为关键词
		if len(word) > 2 {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// selectBestDefaultValue 选择最佳的默认值
func (f *PropertyValueFixer) selectBestDefaultValue(templateProp temutemplate.TemplateRespGoodsProperty) temutemplate.PropertyValue {
	if len(templateProp.Values) == 0 {
		f.logger.Error("❌ 属性没有可选值列表")
		return temutemplate.PropertyValue{}
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

// FixAllInvalidProperties 批量修复所有无效属性
func (f *PropertyValueFixer) FixAllInvalidProperties(
	properties []models.PropertyItem,
	templateProps []temutemplate.TemplateRespGoodsProperty,
) []models.PropertyItem {

	f.logger.Info("🔧 开始批量修复无效属性值")
	f.logger.Infof("📊 输入属性数量: %d, 模板属性数量: %d", len(properties), len(templateProps))

	// 创建模板属性映射
	templateMap := make(map[int]temutemplate.TemplateRespGoodsProperty)
	for _, tmpl := range templateProps {
		templateMap[tmpl.PID] = tmpl
	}

	fixedProperties := make([]models.PropertyItem, 0, len(properties))
	fixedCount := 0

	// 修复每个属性
	for _, prop := range properties {
		templateProp, exists := templateMap[prop.Pid]
		if !exists {
			// 模板中不存在的属性，跳过
			f.logger.Warnf("⚠️ 属性PID=%d在模板中不存在，跳过修复", prop.Pid)
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
				f.logger.Infof("🔧 属性修复: PID=%d, '%s'(VID=%d) → '%s'(VID=%d)",
					prop.Pid, prop.Value, prop.Vid, fixedProp.Value, fixedProp.Vid)
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
	prop models.PropertyItem,
	templateProp temutemplate.TemplateRespGoodsProperty,
) *models.PropertyItem {
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
