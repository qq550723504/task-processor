package modules

import (
	"task-processor/internal/platforms/shein/api/attribute"
)

// AttributeBuilder 属性构建器
type AttributeBuilder struct {
	validator *AttributeValidator
}

// NewAttributeBuilder 创建新的属性构建器
func NewAttributeBuilder(validator *AttributeValidator) *AttributeBuilder {
	return &AttributeBuilder{
		validator: validator,
	}
}

// BuildAttributeValues 构建属性值列表
func (b *AttributeBuilder) BuildAttributeValues(valueInfoList []attribute.AttributeValue) []GenerateAttributeValue {
	values := make([]GenerateAttributeValue, 0, len(valueInfoList))
	for _, v := range valueInfoList {
		values = append(values, GenerateAttributeValue{
			ID:    v.AttributeValueID,
			Value: v.AttributeValueEn,
		})
	}
	return values
}

// BuildGenerateAttribute 构建生成属性
func (b *AttributeBuilder) BuildGenerateAttribute(attr attribute.AttributeInfo) GenerateAttribute {
	required := b.validator.IsAttributeRequired(attr)

	return GenerateAttribute{
		AttrID:    attr.AttributeID,
		AttrValue: b.BuildAttributeValues(attr.AttributeValueInfoList),
		Required:  required,
		Type:      attr.AttributeMode,
	}
}

// BuildSaleGenerateAttribute 构建销售属性
func (b *AttributeBuilder) BuildSaleGenerateAttribute(attr attribute.AttributeInfo) GenerateAttribute {
	saleRequired := b.validator.IsSaleSpecRequired(attr)

	return GenerateAttribute{
		AttrID:    attr.AttributeID,
		AttrValue: b.BuildAttributeValues(attr.AttributeValueInfoList),
		Required:  saleRequired,
		Type:      attr.AttributeMode,
	}
}
