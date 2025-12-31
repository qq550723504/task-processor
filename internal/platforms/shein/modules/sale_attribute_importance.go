// Package modules 提供SHEIN平台销售属性的重要性计算功能
package modules

import (
	"task-processor/internal/platforms/shein/api/attribute"
)

// NewAttributeImportanceCalculatorForSaleAttribute 创建销售属性专用的重要性计算器实例
func NewAttributeImportanceCalculatorForSaleAttribute() *AttributeImportanceCalculator {
	return &AttributeImportanceCalculator{
		rules: &ImportanceRules{
			RemarkListScore: 10,
			RequiredScore:   20,
			SampleScore:     5,
			ActiveScore:     15,
			DisplayScore:    8,
		},
	}
}

// CalculateImportanceForSaleAttribute 为销售属性计算重要性（保持原有业务逻辑）
func CalculateImportanceForSaleAttribute(calc *AttributeImportanceCalculator, attribute *attribute.AttributeInfo) int {
	importance := 0
	if len(attribute.AttributeRemarkList) > 0 {
		importance += calc.rules.RemarkListScore
	}
	if attribute.AttributeLabel == 1 {
		importance += calc.rules.RequiredScore
	}
	if attribute.IsSample == 1 {
		importance += calc.rules.SampleScore
	}
	if attribute.AttributeStatus == 3 {
		importance += calc.rules.ActiveScore
	}
	if attribute.AttributeIsShow == 1 {
		importance += calc.rules.DisplayScore
	}
	return importance
}

// CalculateImportance 计算属性重要性
func (calc *AttributeImportanceCalculator) CalculateImportance(attribute *attribute.AttributeInfo) int {
	importance := 0
	if len(attribute.AttributeRemarkList) > 0 {
		importance += calc.rules.RemarkListScore
	}
	if attribute.AttributeLabel == 1 {
		importance += calc.rules.RequiredScore
	}
	if attribute.IsSample == 1 {
		importance += calc.rules.SampleScore
	}
	if attribute.AttributeStatus == 3 {
		importance += calc.rules.ActiveScore
	}
	if attribute.AttributeIsShow == 1 {
		importance += calc.rules.DisplayScore
	}
	return importance
}
