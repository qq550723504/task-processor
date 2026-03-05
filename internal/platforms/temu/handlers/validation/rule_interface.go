// Package validation 提供验证规则接口定义
package validation

import (
	"task-processor/internal/platforms/temu/api/models"
	"task-processor/internal/platforms/temu/handlers/common"
	"task-processor/internal/platforms/temu/types"
)

// ValidationRule 验证规则接口
type ValidationRule interface {
	// GetRuleName 获取规则名称
	GetRuleName() string

	// Matches 判断规则是否匹配该属性特征
	Matches(feature common.PropertyFeature) bool

	// Validate 验证属性值
	Validate(props []*models.PropertyItem, templateProp types.TemplateRespGoodsProperty) RuleValidationResult

	// Fix 修复属性值
	Fix(props []*models.PropertyItem, templateProp types.TemplateRespGoodsProperty) error
}

// RuleValidationResult 规则验证结果
type RuleValidationResult struct {
	IsValid       bool        // 是否有效
	ErrorMessage  string      // 错误消息
	CurrentValue  interface{} // 当前值
	ExpectedValue interface{} // 期望值
	CanAutoFix    bool        // 是否可以自动修复
}
