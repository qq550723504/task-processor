// Package handlers 提供TEMU平台的AI属性验证功能
package handlers

import (
	"fmt"

	"task-processor/internal/platforms/temu/types"
)

// validatePropertyConstraints 验证属性约束
func (m *AIPropertyMapper) validatePropertyConstraints(templateProps []types.TemplateRespGoodsProperty, ext *types.ExtensionInfo) error {
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
func (m *AIPropertyMapper) validatePropertyValues(templateProps []types.TemplateRespGoodsProperty, ext *types.ExtensionInfo) error {
	m.logger.Info("🔍 开始验证属性值有效性")

	// 构建模板属性映射
	templateMap := make(map[int]types.TemplateRespGoodsProperty)
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
