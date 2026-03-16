// Package rules 提供验证规则接口定义
package rules

import (
	models "task-processor/internal/platforms/temu/api/product"
	temutemplate "task-processor/internal/platforms/temu/api/template"
	"task-processor/internal/platforms/temu/handlers/handlerbase"
)

// ValidationRule 验证规则接口
type ValidationRule interface {
	// GetRuleName 获取规则名称
	GetRuleName() string

	// Matches 判断规则是否匹配该属性特征
	Matches(feature handlerbase.PropertyFeature) bool

	// Validate 验证属性值
	Validate(props []*models.PropertyItem, templateProp temutemplate.TemplateRespGoodsProperty) RuleValidationResult

	// Fix 修复属性值
	Fix(props []*models.PropertyItem, templateProp temutemplate.TemplateRespGoodsProperty) error
}

// RuleValidationResult 规则验证结果
type RuleValidationResult struct {
	IsValid       bool   // 是否有效
	ErrorMessage  string // 错误消息
	CurrentValue  any    // 当前值
	ExpectedValue any    // 期望值
	CanAutoFix    bool   // 是否可以自动修复
}
