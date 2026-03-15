package property

import (
	models "task-processor/internal/platforms/temu/api/product"
	temutemplate "task-processor/internal/platforms/temu/api/template"

	"github.com/sirupsen/logrus"
)

// ConditionalPropertyValidator 条件属性验证器
type ConditionalPropertyValidator struct {
	logger *logrus.Entry
}

// NewConditionalPropertyValidator 创建条件属性验证器
func NewConditionalPropertyValidator(logger *logrus.Entry) *ConditionalPropertyValidator {
	return &ConditionalPropertyValidator{
		logger: logger,
	}
}

// ValidateAndCleanConditionalProperties 验证和清理条件属性依赖关系
func (v *ConditionalPropertyValidator) ValidateAndCleanConditionalProperties(properties *[]models.PropertyItem, templateProps []temutemplate.TemplateRespGoodsProperty) {
	v.logger.Info("🔍 开始验证条件属性依赖关系")

	// 创建模板属性映射
	templateMap := make(map[int]temutemplate.TemplateRespGoodsProperty)
	for _, templateProp := range templateProps {
		templateMap[templateProp.TemplatePID] = templateProp
	}

	// 创建当前属性值映射（按TemplatePID）
	currentValues := make(map[int]models.PropertyItem)
	for _, prop := range *properties {
		if prop.TemplatePid != 0 {
			currentValues[prop.TemplatePid] = prop
		}
	}

	// 检查每个条件属性
	validProperties := make([]models.PropertyItem, 0, len(*properties))
	removedCount := 0

	for i, prop := range *properties {
		shouldKeep := true

		if templateProp, exists := templateMap[prop.TemplatePid]; exists {
			// 检查是否是条件属性
			if len(templateProp.TemplatePropertyValueParentList) > 0 {
				v.logger.Infof("🔍 检查条件属性: %s (template_pid=%d)", templateProp.Name, templateProp.TemplatePID)

				// 验证父属性条件
				isValid := false
				for _, parentList := range templateProp.TemplatePropertyValueParentList {
					if v.checkParentCondition(parentList, currentValues, templateMap) {
						isValid = true
						break
					}
				}

				if !isValid {
					v.logger.Warnf("⚠️ 条件属性 %s (template_pid=%d) 的父属性条件不满足，将被移除",
						templateProp.Name, templateProp.TemplatePID)
					shouldKeep = false
					removedCount++
				} else {
					v.logger.Infof("✅ 条件属性 %s (template_pid=%d) 验证通过",
						templateProp.Name, templateProp.TemplatePID)
				}
			}
		}

		if shouldKeep {
			validProperties = append(validProperties, prop)
		} else {
			v.logger.Warnf("🗑️ 移除无效条件属性[%d]: PID=%d, TemplatePID=%d, Value=%s",
				i, prop.Pid, prop.TemplatePid, prop.Value)
		}
	}

	// 更新原始slice
	*properties = validProperties

	if removedCount > 0 {
		v.logger.Infof("🔧 条件属性验证完成，移除了 %d 个无效属性", removedCount)
	} else {
		v.logger.Info("✅ 条件属性验证完成，所有属性都有效")
	}
}

// checkParentCondition 检查父属性条件是否满足
func (v *ConditionalPropertyValidator) checkParentCondition(parentList temutemplate.TemplatePropertyValueParent,
	currentValues map[int]models.PropertyItem, templateMap map[int]temutemplate.TemplateRespGoodsProperty) bool {

	// 找到父属性的TemplatePID
	var parentTemplatePID int
	for templatePID, templateProp := range templateMap {
		for _, parentVID := range parentList.ParentVIDs {
			for _, value := range templateProp.Values {
				if value.VID == parentVID {
					parentTemplatePID = templatePID
					break
				}
			}
			if parentTemplatePID != 0 {
				break
			}
		}
		if parentTemplatePID != 0 {
			break
		}
	}

	if parentTemplatePID == 0 {
		v.logger.Warnf("⚠️ 未找到父属性的TemplatePID")
		return false
	}

	// 检查当前是否有对应的父属性值
	if parentProp, exists := currentValues[parentTemplatePID]; exists {
		// 检查父属性的VID是否在允许的范围内
		for _, parentVID := range parentList.ParentVIDs {
			if parentProp.Vid == parentVID {
				v.logger.Infof("✅ 父属性条件满足: TemplatePID=%d, VID=%d", parentTemplatePID, parentVID)
				return true
			}
		}
		v.logger.Warnf("⚠️ 父属性VID不匹配: 期望=%v, 实际=%d", parentList.ParentVIDs, parentProp.Vid)
	} else {
		v.logger.Warnf("⚠️ 缺少父属性: TemplatePID=%d", parentTemplatePID)
	}

	return false
}
