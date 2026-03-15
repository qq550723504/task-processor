package validation

import (
	"strings"
	"task-processor/internal/platforms/shein/api/attribute"
)

// AttributeValidator 属性验证器
type AttributeValidator struct{}

// NewAttributeValidator 创建新的属性验证器
func NewAttributeValidator() *AttributeValidator {
	return &AttributeValidator{}
}

// IsAttributeRequired 基于模板数据判断属性是否必填
func (v *AttributeValidator) IsAttributeRequired(attr attribute.AttributeInfo) bool {
	// 判断必填的优先级逻辑（基于实际模板数据）
	switch {
	case len(attr.AttributeRemarkList) > 0:
		// 有备注列表的属性通常是必填的
		return true
	case attr.AttributeLabel == 1:
		// AttributeLabel为1表示必填
		return true
	case attr.AttributeStatus == 3:
		// 状态为3的属性通常是必填的
		return true
	case attr.AttributeIsShow == 1:
		// 显示标记为1且有其他必填特征
		return false
	default:
		return false
	}
}

// IsSaleSpecRequired 判断销售属性是否必填（基于属性名称的智能判断）
func (v *AttributeValidator) IsSaleSpecRequired(attr attribute.AttributeInfo) bool {
	// 基础条件：必须有属性值列表
	if len(attr.AttributeValueInfoList) == 0 {
		return false
	}

	// 基于属性名称进行智能判断，只有核心变化属性才标记为必填
	attrNameLower := strings.ToLower(attr.AttributeNameEn)

	// 只有Size和Color这类核心变化属性才标记为必填
	isCoreAttribute := strings.Contains(attrNameLower, "size") ||
		strings.Contains(attrNameLower, "color") ||
		strings.Contains(attrNameLower, "colour")

	// 必须是核心属性且有多个候选值
	hasMultipleValues := len(attr.AttributeValueInfoList) >= 2

	return isCoreAttribute && hasMultipleValues
}
