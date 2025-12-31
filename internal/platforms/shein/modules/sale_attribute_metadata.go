// Package modules 提供SHEIN平台销售属性的元数据构建功能
package modules

import (
	"fmt"
	"task-processor/internal/domain/model"
	"task-processor/internal/platforms/shein/api/attribute"

	"github.com/sirupsen/logrus"
)

// SaleAttributeMetadataBuilder 销售属性元数据构建器
type SaleAttributeMetadataBuilder struct {
	valueFilter *SaleAttributeValueFilter
}

// NewSaleAttributeMetadataBuilder 创建元数据构建器实例
func NewSaleAttributeMetadataBuilder() *SaleAttributeMetadataBuilder {
	return &SaleAttributeMetadataBuilder{
		valueFilter: NewSaleAttributeValueFilter(),
	}
}

// BuildAttributeMetadata 构建属性元数据
func (b *SaleAttributeMetadataBuilder) BuildAttributeMetadata(ctx *TaskContext, importanceCalc *AttributeImportanceCalculator) []AttributeMetadata {
	var attributeMetadata []AttributeMetadata
	isSingleVariant := ctx.Variants == nil || len(*ctx.Variants) == 0

	for _, saleAttr := range ctx.BuildAttributeData.SaleAttributeData {
		metadata := AttributeMetadata{
			AttrID:    saleAttr.AttrID,
			AttrValue: append([]GenerateAttributeValue{}, saleAttr.AttrValue...),
			Required:  saleAttr.Required,
			Type:      saleAttr.Type,
		}

		// 从属性模板中查找对应的属性信息
		if ctx.AttributeTemplates != nil && len(ctx.AttributeTemplates.Data) > 0 {
			for _, attribute := range ctx.AttributeTemplates.Data[0].AttributeInfos {
				if attribute.AttributeID == saleAttr.AttrID {
					metadata.Importance = CalculateImportanceForSaleAttribute(importanceCalc, &attribute)
					metadata.AttributeName = attribute.AttributeName
					metadata.AttributeNameEn = attribute.AttributeNameEn
					metadata.VariantName = b.findMappedName(saleAttr.AttrID, ctx.AttributeTemplates)
					break
				}
			}
		}

		if metadata.VariantName == "" {
			metadata.VariantName = fmt.Sprintf("attr_%d", saleAttr.AttrID)
		}

		// 单变体产品优化
		if isSingleVariant && len(metadata.AttrValue) > 3 {
			logrus.Debugf("单变体产品：属性 %s (ID:%d) 的候选值从 %d 个简化为 3 个",
				metadata.AttributeNameEn, metadata.AttrID, len(metadata.AttrValue))
			metadata.AttrValue = metadata.AttrValue[:3]
		}

		// 多变体产品优化：根据实际变体值过滤候选列表
		if !isSingleVariant && ctx.AmazonProduct != nil {
			metadata.AttrValue = b.filterAttributeValuesByActualUsage(
				metadata.AttrValue,
				ctx.AmazonProduct.VariationsValues,
				metadata.AttributeNameEn,
			)
		}

		attributeMetadata = append(attributeMetadata, metadata)
	}

	return attributeMetadata
}

// findMappedName 查找映射的属性名称
func (b *SaleAttributeMetadataBuilder) findMappedName(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) string {
	if attributeTemplates == nil || len(attributeTemplates.Data) == 0 {
		return ""
	}
	for _, attribute := range attributeTemplates.Data[0].AttributeInfos {
		if attribute.AttributeID == attrID {
			if attribute.AttributeNameEn != "" {
				return attribute.AttributeNameEn
			}
			if attribute.AttributeName != "" {
				return attribute.AttributeName
			}
			break
		}
	}
	return ""
}

// BuildAttributeNameMappings 构建属性名称映射
func (b *SaleAttributeMetadataBuilder) BuildAttributeNameMappings(
	attributeData BuildAttributeInfo,
	attributeTemplates *attribute.AttributeTemplateInfo,
) map[int]string {
	mappings := make(map[int]string)
	for _, saleAttr := range attributeData.SaleAttributeData {
		if mappedName := b.findMappedName(saleAttr.AttrID, attributeTemplates); mappedName != "" {
			mappings[saleAttr.AttrID] = mappedName
		} else {
			mappings[saleAttr.AttrID] = fmt.Sprintf("attr_%d", saleAttr.AttrID)
		}
	}
	return mappings
}

// filterAttributeValuesByActualUsage 根据实际变体值过滤属性候选列表
func (b *SaleAttributeMetadataBuilder) filterAttributeValuesByActualUsage(
	candidateValues []GenerateAttributeValue,
	variationsValues []model.VariationValue,
	attributeName string,
) []GenerateAttributeValue {
	// 从变体数据中提取实际使用的属性值
	actualValues := b.valueFilter.ExtractActualValuesFromVariations(
		variationsValues,
		attributeName,
	)

	// 使用筛选器过滤候选值
	filteredValues := b.valueFilter.FilterAttributeValuesByUsage(
		candidateValues,
		actualValues,
		attributeName,
	)

	return filteredValues
}
