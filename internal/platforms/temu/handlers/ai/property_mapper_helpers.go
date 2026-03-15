// Package handlers 提供AI属性映射的辅助功能
package ai

import (
	models "task-processor/internal/platforms/temu/api/product"
	"task-processor/internal/platforms/temu/types"
)

// isRequiredProperty 判断属性是否为必填属性
func (m *AIPropertyMapper) isRequiredProperty(pid int64, templateProps []types.TemplateRespGoodsProperty) bool {
	for _, prop := range templateProps {
		if int64(prop.PID) == pid && prop.Required {
			return true
		}
	}
	return false
}

// verifyAndFillMissingRequired 验证并填充缺失的必填属性
func (m *AIPropertyMapper) verifyAndFillMissingRequired(templateProps []types.TemplateRespGoodsProperty, ext *models.ExtensionInfo) int {
	// 构建已填充属性的PID集合
	filledPIDs := make(map[int64]bool)
	for _, prop := range ext.GoodsProperty.GoodsProperties {
		filledPIDs[int64(prop.Pid)] = true
	}

	// 检查缺失的必填属性
	missingCount := 0
	var missingProps []types.TemplateRespGoodsProperty

	for _, templateProp := range templateProps {
		templatePID := int64(templateProp.PID)
		if templateProp.Required && !filledPIDs[templatePID] {
			missingProps = append(missingProps, templateProp)
			missingCount++
		}
	}

	// 如果有缺失的必填属性，用默认值填充
	if missingCount > 0 {
		m.logger.Warnf("⚠️ 发现%d个缺失的必填属性，开始填充默认值", missingCount)
		for _, missingProp := range missingProps {
			m.logger.Warnf("  - 缺失属性: %s (PID=%d, Type=%d)",
				missingProp.Name, missingProp.PID, missingProp.PropertyValueType)
		}

		// 使用默认填充器填充缺失的属性
		m.defaultFiller.FillRequiredPropertiesWithDefaults(templateProps, ext)
	}

	return missingCount
}

// verifyRequiredProperties 验证必填属性完整性（保留原有方法以兼容）
func (m *AIPropertyMapper) verifyRequiredProperties(templateProps []types.TemplateRespGoodsProperty, ext *models.ExtensionInfo) {
	m.verifyAndFillMissingRequired(templateProps, ext)
}
