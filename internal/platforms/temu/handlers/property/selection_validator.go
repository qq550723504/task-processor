// Package property 提供属性选择约束验证功能
package property

import (
	models "task-processor/internal/platforms/temu/api/product"
	temutemplate "task-processor/internal/platforms/temu/api/template"

	"github.com/sirupsen/logrus"
)

// PropertySelectionValidator 属性选择约束验证器
type PropertySelectionValidator struct {
	logger *logrus.Entry
}

// NewPropertySelectionValidator 创建新的属性选择约束验证器
func NewPropertySelectionValidator(logger *logrus.Entry) *PropertySelectionValidator {
	return &PropertySelectionValidator{
		logger: logger,
	}
}

// ValidateSelectionConstraints 验证并修复选择约束
// 确保单选属性只有一个值，多选属性不超过最大选择数
func (v *PropertySelectionValidator) ValidateSelectionConstraints(
	properties []models.PropertyItem,
	templateProps []temutemplate.TemplateRespGoodsProperty,
) []models.PropertyItem {

	v.logger.Info("🔍 开始验证属性选择约束")

	// 创建模板属性映射，便于快速查找
	templateMap := make(map[int]temutemplate.TemplateRespGoodsProperty)
	for _, tmpl := range templateProps {
		templateMap[tmpl.PID] = tmpl
	}

	// 按PID分组属性
	pidGroups := make(map[int][]models.PropertyItem)
	for _, prop := range properties {
		pidGroups[prop.Pid] = append(pidGroups[prop.Pid], prop)
	}

	var result []models.PropertyItem
	violationCount := 0

	// 检查每个PID组
	for pid, propGroup := range pidGroups {
		templateProp, exists := templateMap[pid]
		if !exists {
			// 模板中不存在的属性，直接保留
			result = append(result, propGroup...)
			continue
		}

		// 只处理选择类型属性
		if templateProp.PropertyValueType != 1 {
			result = append(result, propGroup...)
			continue
		}

		// 检查选择约束
		maxChoose := templateProp.ChooseMaxNum
		if maxChoose <= 0 {
			// 没有限制，保留所有
			result = append(result, propGroup...)
			continue
		}

		if len(propGroup) > maxChoose {
			v.logger.Warnf("⚠️ 属性 %s (PID=%d) 违反选择约束: 实际%d个 > 最大%d个",
				templateProp.Name, pid, len(propGroup), maxChoose)

			// 应用约束修复策略
			fixedProps := v.applySelectionConstraint(propGroup, templateProp)
			result = append(result, fixedProps...)
			violationCount++
		} else {
			result = append(result, propGroup...)
		}
	}

	if violationCount > 0 {
		v.logger.Warnf("🔧 选择约束验证完成，修复了 %d 个违规属性组", violationCount)
	} else {
		v.logger.Info("✅ 所有属性都符合选择约束")
	}

	return result
}

// applySelectionConstraint 应用选择约束修复策略
func (v *PropertySelectionValidator) applySelectionConstraint(
	propGroup []models.PropertyItem,
	templateProp temutemplate.TemplateRespGoodsProperty,
) []models.PropertyItem {

	maxChoose := templateProp.ChooseMaxNum

	if maxChoose == 1 {
		// 单选：选择最合适的一个
		return v.selectBestSingleChoice(propGroup, templateProp)
	} else {
		// 多选：选择前N个最合适的
		return v.selectBestMultipleChoices(propGroup, templateProp, maxChoose)
	}
}

// selectBestSingleChoice 为单选属性选择最佳选项
func (v *PropertySelectionValidator) selectBestSingleChoice(
	propGroup []models.PropertyItem,
	templateProp temutemplate.TemplateRespGoodsProperty,
) []models.PropertyItem {

	v.logger.Debugf("🎯 为单选属性 %s (PID=%d) 选择最佳选项", templateProp.Name, templateProp.PID)

	// 策略1: 优先选择有效的VID
	for _, prop := range propGroup {
		if v.isValidVID(prop.Vid, templateProp.Values) {
			v.logger.Debugf("   选择: %s (VID=%d) - 有效VID", prop.Value, prop.Vid)
			return []models.PropertyItem{prop}
		}
	}

	// 策略2: 选择第一个候选值匹配的
	for _, prop := range propGroup {
		if v.isValueInCandidates(prop.Value, templateProp.Values) {
			v.logger.Debugf("   选择: %s (VID=%d) - 候选值匹配", prop.Value, prop.Vid)
			return []models.PropertyItem{prop}
		}
	}

	// 策略3: 选择第一个
	if len(propGroup) > 0 {
		selected := propGroup[0]
		v.logger.Debugf("   选择: %s (VID=%d) - 默认第一个", selected.Value, selected.Vid)
		return []models.PropertyItem{selected}
	}

	return []models.PropertyItem{}
}

// selectBestMultipleChoices 为多选属性选择最佳选项组合
func (v *PropertySelectionValidator) selectBestMultipleChoices(
	propGroup []models.PropertyItem,
	templateProp temutemplate.TemplateRespGoodsProperty,
	maxChoose int,
) []models.PropertyItem {

	v.logger.Debugf("🎯 为多选属性 %s (PID=%d) 选择最多%d个选项",
		templateProp.Name, templateProp.PID, maxChoose)

	// 按优先级排序
	sortedProps := v.sortPropertiesByPriority(propGroup, templateProp)

	// 取前N个
	if len(sortedProps) > maxChoose {
		result := sortedProps[:maxChoose]
		v.logger.Debugf("   选择了前%d个最佳选项", len(result))
		return result
	}

	return sortedProps
}

// sortPropertiesByPriority 按优先级排序属性
func (v *PropertySelectionValidator) sortPropertiesByPriority(
	propGroup []models.PropertyItem,
	templateProp temutemplate.TemplateRespGoodsProperty,
) []models.PropertyItem {

	// 创建带优先级的属性列表
	type prioritizedProp struct {
		prop     models.PropertyItem
		priority int
	}

	var prioritized []prioritizedProp

	for _, prop := range propGroup {
		priority := v.calculatePriority(prop, templateProp)
		prioritized = append(prioritized, prioritizedProp{prop: prop, priority: priority})
	}

	// 按优先级排序（高优先级在前）
	for i := 0; i < len(prioritized)-1; i++ {
		for j := i + 1; j < len(prioritized); j++ {
			if prioritized[i].priority < prioritized[j].priority {
				prioritized[i], prioritized[j] = prioritized[j], prioritized[i]
			}
		}
	}

	// 提取排序后的属性
	result := make([]models.PropertyItem, len(prioritized))
	for i, p := range prioritized {
		result[i] = p.prop
	}

	return result
}

// calculatePriority 计算属性的优先级
func (v *PropertySelectionValidator) calculatePriority(
	prop models.PropertyItem,
	templateProp temutemplate.TemplateRespGoodsProperty,
) int {
	priority := 0

	// 有效VID加分
	if v.isValidVID(prop.Vid, templateProp.Values) {
		priority += 100
	}

	// 候选值匹配加分
	if v.isValueInCandidates(prop.Value, templateProp.Values) {
		priority += 50
	}

	// 非空值加分
	if prop.Value != "" {
		priority += 10
	}

	return priority
}

// isValidVID 检查VID是否在候选列表中
func (v *PropertySelectionValidator) isValidVID(vid int, candidates []temutemplate.PropertyValue) bool {
	for _, candidate := range candidates {
		if candidate.VID == vid {
			return true
		}
	}
	return false
}

// isValueInCandidates 检查值是否在候选列表中
func (v *PropertySelectionValidator) isValueInCandidates(value string, candidates []temutemplate.PropertyValue) bool {
	for _, candidate := range candidates {
		if candidate.Value == value {
			return true
		}
	}
	return false
}

// GetSelectionConstraintSummary 获取选择约束摘要信息
func (v *PropertySelectionValidator) GetSelectionConstraintSummary(
	templateProps []temutemplate.TemplateRespGoodsProperty,
) map[string]any {

	summary := map[string]any{
		"total_properties":     len(templateProps),
		"selection_properties": 0,
		"single_choice":        0,
		"multiple_choice":      0,
		"unlimited_choice":     0,
	}

	for _, prop := range templateProps {
		if prop.PropertyValueType == 1 { // 选择类型
			summary["selection_properties"] = summary["selection_properties"].(int) + 1

			if prop.ChooseMaxNum == 1 {
				summary["single_choice"] = summary["single_choice"].(int) + 1
			} else if prop.ChooseMaxNum > 1 {
				summary["multiple_choice"] = summary["multiple_choice"].(int) + 1
			} else {
				summary["unlimited_choice"] = summary["unlimited_choice"].(int) + 1
			}
		}
	}

	return summary
}
