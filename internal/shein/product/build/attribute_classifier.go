package build

import (
	"task-processor/internal/core/logger"
	"task-processor/internal/shein"
	"task-processor/internal/shein/api/attribute"
	"task-processor/internal/shein/product/attribute/sale"

)

// AttributeClassifier 属性分类器
type AttributeClassifier struct {
	builder *AttributeBuilder
	filter  *sale.SaleAttributeSmartFilter
}

// NewAttributeClassifier 创建新的属性分类器
func NewAttributeClassifier(builder *AttributeBuilder) *AttributeClassifier {
	return &AttributeClassifier{
		builder: builder,
		filter:  sale.NewSaleAttributeSmartFilter(),
	}
}

// ClassifyAndBuildAttribute 分类并构建属性
func (c *AttributeClassifier) ClassifyAndBuildAttribute(attr attribute.AttributeInfo, attributeInfo *shein.BuildAttributeInfo) {
	// 检查级联依赖关系
	if attr.CascadeAttributeID != 0 {
		logger.GetGlobalLogger("shein/product").Infof("属性ID %d 存在级联依赖关系，依赖于: %d", attr.AttributeID, attr.CascadeAttributeID)
	}

	// 根据属性类型分类
	switch attr.AttributeType {
	case 4: // 产品属性
		generateAttr := c.builder.BuildGenerateAttribute(attr)
		attributeInfo.AttributeData = append(attributeInfo.AttributeData, generateAttr)
	case 1: // 销售规格
		generateAttr := c.builder.BuildSaleGenerateAttribute(attr)
		attributeInfo.SaleAttributeData = append(attributeInfo.SaleAttributeData, generateAttr)
	case 3: // 成分属性
		// 成分属性作为产品属性处理
		generateAttr := c.builder.BuildGenerateAttribute(attr)
		attributeInfo.AttributeData = append(attributeInfo.AttributeData, generateAttr)
		// case 2: // 尺寸属性（长度、宽度、高度等）
		// 尺寸属性作为产品属性处理，目前暂未启用
	}
}
