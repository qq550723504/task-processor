package modules

import (
	"task-processor/internal/common/shein/api/attribute"
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

// IsSaleSpecRequired 判断销售属性是否必填
func (v *AttributeValidator) IsSaleSpecRequired(attr attribute.AttributeInfo) bool {
	// 销售属性的必填判断逻辑
	switch {
	case len(attr.AttributeRemarkList) > 0:
		return true
	case attr.AttributeLabel == 1:
		return true
	case attr.IsSample == 1:
		// IsSample为1表示是示例属性，通常必填
		return true
	default:
		return false
	}
}
