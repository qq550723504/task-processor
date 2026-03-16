// Package property 提供必填属性保障机制
package property

import (
	models "task-processor/internal/platforms/temu/api/product"
	temutemplate "task-processor/internal/platforms/temu/api/template"

	"github.com/sirupsen/logrus"
)

// RequiredPropertyGuardian 必填属性守护者 - 确保所有必填属性都被正确填充
type RequiredPropertyGuardian struct {
	logger *logrus.Entry
	filler *DefaultPropertyFiller
}

// NewRequiredPropertyGuardian 创建必填属性守护者
func NewRequiredPropertyGuardian(logger *logrus.Entry) *RequiredPropertyGuardian {
	return &RequiredPropertyGuardian{
		logger: logger,
		filler: NewDefaultPropertyFiller(logger),
	}
}

// GuardAllRequiredProperties 保障所有必填属性（包括条件依赖的）
func (g *RequiredPropertyGuardian) GuardAllRequiredProperties(
	templateProps []temutemplate.TemplateRespGoodsProperty,
	ext *models.ExtensionInfo,
) error {
	g.logger.Info("🛡️ 开始必填属性保障检查")

	// 1. 基础必填属性检查
	missingBasic := g.checkBasicRequiredProperties(templateProps, ext)

	// 2. 条件依赖必填属性检查（通用逻辑）
	missingConditional := g.checkConditionalRequiredProperties(templateProps, ext)

	totalMissing := len(missingBasic) + len(missingConditional)

	if totalMissing > 0 {
		g.logger.Warnf("⚠️ 发现 %d 个缺失的必填属性，开始填充", totalMissing)

		// 填充所有缺失的必填属性
		allMissing := append(missingBasic, missingConditional...)
		g.fillMissingProperties(allMissing, ext)

		g.logger.Infof("✅ 必填属性保障完成，共填充 %d 个属性", totalMissing)
	} else {
		g.logger.Info("✅ 所有必填属性检查通过")
	}

	return nil
}

// checkBasicRequiredProperties 检查基础必填属性
func (g *RequiredPropertyGuardian) checkBasicRequiredProperties(
	templateProps []temutemplate.TemplateRespGoodsProperty,
	ext *models.ExtensionInfo,
) []temutemplate.TemplateRespGoodsProperty {
	filledPIDs := make(map[int]bool)
	for _, prop := range ext.GoodsProperty.GoodsProperties {
		filledPIDs[prop.Pid] = true
	}

	var missing []temutemplate.TemplateRespGoodsProperty
	for _, templateProp := range templateProps {
		if templateProp.Required && !filledPIDs[templateProp.PID] {
			missing = append(missing, templateProp)
			g.logger.Warnf("  - 缺失基础必填属性: %s (PID=%d)", templateProp.Name, templateProp.PID)
		}
	}

	return missing
}

// checkConditionalRequiredProperties 检查条件依赖的必填属性（通用逻辑）
func (g *RequiredPropertyGuardian) checkConditionalRequiredProperties(
	templateProps []temutemplate.TemplateRespGoodsProperty,
	ext *models.ExtensionInfo,
) []temutemplate.TemplateRespGoodsProperty {
	var missing []temutemplate.TemplateRespGoodsProperty

	// 构建已填充属性的VID集合
	filledVIDs := make(map[int]bool)
	for _, prop := range ext.GoodsProperty.GoodsProperties {
		filledVIDs[prop.Vid] = true
	}

	g.logger.Infof("🔍 已填充的VID集合: %v", filledVIDs)

	// 检查所有有条件依赖的必填属性
	for _, templateProp := range templateProps {
		// 跳过已填充的属性
		if g.isPropertyFilledByTemplatePID(ext, templateProp.TemplatePID) {
			continue
		}

		// 检查是否是条件依赖的必填属性
		if templateProp.Required && g.hasParentCondition(templateProp) {
			g.logger.Infof("🔍 检查条件必填属性: %s (template_pid=%d)", templateProp.Name, templateProp.TemplatePID)
			// 检查父条件是否满足
			if g.isParentConditionMet(templateProp, filledVIDs) {
				missing = append(missing, templateProp)
				g.logger.Warnf("  - 缺失条件必填属性: %s (PID=%d, template_pid=%d) - 父条件已满足", templateProp.Name, templateProp.PID, templateProp.TemplatePID)
			} else {
				g.logger.Infof("  - 条件必填属性 %s 的父条件未满足，跳过", templateProp.Name)
			}
		}
	}

	return missing
}

// isPropertyFilledByTemplatePID 根据TemplatePID检查属性是否已填充
func (g *RequiredPropertyGuardian) isPropertyFilledByTemplatePID(ext *models.ExtensionInfo, templatePID int) bool {
	for _, prop := range ext.GoodsProperty.GoodsProperties {
		if prop.TemplatePid == templatePID {
			return true
		}
	}
	return false
}

// hasParentCondition 检查属性是否有父条件依赖
func (g *RequiredPropertyGuardian) hasParentCondition(prop temutemplate.TemplateRespGoodsProperty) bool {
	// 检查是否有 parent_template_pid 或其他条件依赖标识
	return prop.ParentTemplatePID > 0 || len(prop.TemplatePropertyValueParentList) > 0
}

// isParentConditionMet 检查父条件是否满足
func (g *RequiredPropertyGuardian) isParentConditionMet(prop temutemplate.TemplateRespGoodsProperty, filledVIDs map[int]bool) bool {
	// 检查 TemplatePropertyValueParentList 中的父条件
	for _, parentList := range prop.TemplatePropertyValueParentList {
		for _, parentVID := range parentList.ParentVIDs {
			if filledVIDs[parentVID] {
				return true // 父条件满足
			}
		}
	}
	return false
}

// isPropertyFilled 检查属性是否已填充
func (g *RequiredPropertyGuardian) isPropertyFilled(ext *models.ExtensionInfo, pid int) bool {
	for _, prop := range ext.GoodsProperty.GoodsProperties {
		if prop.Pid == pid {
			return true
		}
	}
	return false
}

// fillMissingProperties 填充缺失的属性
func (g *RequiredPropertyGuardian) fillMissingProperties(
	missingProps []temutemplate.TemplateRespGoodsProperty,
	ext *models.ExtensionInfo,
) {
	for _, prop := range missingProps {
		g.filler.FillSingleRequiredProperty(prop, ext)
	}
}

// GetDefaultFiller 获取内部的DefaultPropertyFiller实例
func (g *RequiredPropertyGuardian) GetDefaultFiller() *DefaultPropertyFiller {
	return g.filler
}
