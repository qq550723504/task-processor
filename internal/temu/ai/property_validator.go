// Package ai 提供TEMU平台的AI属性验证功能
package ai

import (
	"fmt"
	"strings"

	models "task-processor/internal/temu/api/product"
	temutemplate "task-processor/internal/temu/api/template"
)

// validatePropertyConstraints 验证属性约束
func (m *AIPropertyMapper) validatePropertyConstraints(templateProps []temutemplate.TemplateRespGoodsProperty, ext *models.ExtensionInfo) error {
	m.logger.Info("🔍 开始验证属性约束")

	// 统计每个RefPID的使用次数
	refPIDCount := make(map[int]int)
	for _, prop := range ext.GoodsProperty.GoodsProperties {
		refPIDCount[prop.RefPid]++
	}

	// 检查单选约束
	for _, templateProp := range templateProps {
		if templateProp.PropertyValueType == 1 && templateProp.ChooseMaxNum == 1 {
			if count := refPIDCount[templateProp.RefPID]; count > 1 {
				return fmt.Errorf("属性 %s (RefPID=%d) 是单选类型，但出现了%d次",
					templateProp.Name, templateProp.RefPID, count)
			}
		}
	}

	m.logger.Info("✅ 属性约束验证通过")
	return nil
}

// validatePropertyValues 验证属性值的有效性
func (m *AIPropertyMapper) validatePropertyValues(templateProps []temutemplate.TemplateRespGoodsProperty, ext *models.ExtensionInfo) error {
	m.logger.Info("🔍 开始验证属性值有效性")

	// 构建模板属性映射
	templateMap := make(map[int]temutemplate.TemplateRespGoodsProperty)
	for _, templateProp := range templateProps {
		templateMap[templateProp.PID] = templateProp
	}

	// 验证每个填充的属性
	for _, prop := range ext.GoodsProperty.GoodsProperties {
		templateProp, exists := templateMap[prop.Pid]
		if !exists {
			m.logger.Warnf("⚠️ 属性PID=%d不存在于模板中", prop.Pid)
			continue
		}

		// 验证选择类型属性的VID
		if templateProp.PropertyValueType == 1 && prop.Vid == 0 {
			return fmt.Errorf("选择类型属性 %s (PID=%d) 的VID不能为0",
				templateProp.Name, prop.Pid)
		}

		// 验证数值范围
		if templateProp.MinValue != "" || templateProp.MaxValue != "" {
			// 这里可以添加数值范围验证逻辑
			m.logger.Debugf("属性 %s 值范围验证: %s", templateProp.Name, prop.Value)
		}
	}

	m.logger.Info("✅ 属性值验证通过")
	return nil
}

// isValidVID 检查VID是否在模板的有效值列表中
func (m *AIPropertyMapper) isValidVID(vid int, values []temutemplate.PropertyValue) bool {
	for _, value := range values {
		if value.VID == vid {
			return true
		}
	}
	return false
}

// isValidVIDForTemplate 检查VID是否在指定模板的有效值列表中
// 注意：这个函数与isValidVID功能相同，保留是为了兼容性
func (m *AIPropertyMapper) isValidVIDForTemplate(vid int, values []temutemplate.PropertyValue) bool {
	return m.isValidVID(vid, values)
}

// findCorrectVIDByValue 通过值匹配找到正确的VID
func (m *AIPropertyMapper) findCorrectVIDByValue(propValue string, values []temutemplate.PropertyValue) (int, string) {
	propValue = strings.ToLower(propValue)

	// 精确匹配
	for _, value := range values {
		if strings.ToLower(value.Value) == propValue {
			return value.VID, value.Value
		}
	}

	// 模糊匹配
	for _, value := range values {
		if strings.Contains(strings.ToLower(value.Value), propValue) ||
			strings.Contains(propValue, strings.ToLower(value.Value)) {
			return value.VID, value.Value
		}
	}

	// 如果都没有匹配，返回第一个值作为默认值
	if len(values) > 0 {
		return values[0].VID, values[0].Value
	}

	return 0, ""
}
