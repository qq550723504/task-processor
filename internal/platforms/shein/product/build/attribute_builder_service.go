package build

import (
	"task-processor/internal/platforms/shein/api/attribute"
	"task-processor/internal/platforms/shein"
	"task-processor/internal/platforms/shein/validation"
)

// AttributeBuilder 属性构建器
type AttributeBuilder struct {
	validator *validation.AttributeValidator
}

// NewAttributeBuilder 创建新的属性构建器
func NewAttributeBuilder(validator *validation.AttributeValidator) *AttributeBuilder {
	return &AttributeBuilder{
		validator: validator,
	}
}

// BuildAttributeValues 构建属性值列表
func (b *AttributeBuilder) BuildAttributeValues(valueInfoList []attribute.AttributeValue) []shein.GenerateAttributeValue {
	values := make([]shein.GenerateAttributeValue, 0, len(valueInfoList))
	for _, v := range valueInfoList {
		values = append(values, shein.GenerateAttributeValue{
			ID:    v.AttributeValueID,
			Value: v.AttributeValueEn,
		})
	}
	return values
}

// BuildGenerateAttribute 构建生成属性
func (b *AttributeBuilder) BuildGenerateAttribute(attr attribute.AttributeInfo) shein.GenerateAttribute {
	required := b.validator.IsAttributeRequired(attr)

	return shein.GenerateAttribute{
		AttrID:    attr.AttributeID,
		AttrValue: b.BuildAttributeValues(attr.AttributeValueInfoList),
		Required:  required,
		Type:      attr.AttributeMode,
	}
}

// BuildSaleGenerateAttribute 构建销售属性
func (b *AttributeBuilder) BuildSaleGenerateAttribute(attr attribute.AttributeInfo) shein.GenerateAttribute {
	saleRequired := b.validator.IsSaleSpecRequired(attr)

	return shein.GenerateAttribute{
		AttrID:    attr.AttributeID,
		AttrValue: b.BuildAttributeValues(attr.AttributeValueInfoList),
		Required:  saleRequired,
		Type:      attr.AttributeMode,
	}
}



