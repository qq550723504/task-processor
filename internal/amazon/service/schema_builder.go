// Package service 提供Amazon产品属性动态构建功能
package service

import (
	"task-processor/internal/amazon/model"
)

// SchemaBuilder 属性构建器
type SchemaBuilder struct {
	marketplaceID string
	languageTag   string
}

// NewSchemaBuilder 创建属性构建器
func NewSchemaBuilder(marketplaceID, languageTag string) *SchemaBuilder {
	return &SchemaBuilder{
		marketplaceID: marketplaceID,
		languageTag:   languageTag,
	}
}

// MarketplaceID 获取市场ID
func (b *SchemaBuilder) MarketplaceID() string {
	return b.marketplaceID
}

// BuildAttribute 根据属性信息构建属性值
func (b *SchemaBuilder) BuildAttribute(attr model.AttributeInfo, value any) []map[string]any {
	switch attr.Format {
	case model.FormatSimple:
		return b.buildSimple(value)
	case model.FormatWithLang:
		return b.buildWithLang(value)
	case model.FormatNested:
		return b.buildNested(attr, value)
	default:
		return b.buildSimple(value)
	}
}

// buildSimple 构建简单格式属性
func (b *SchemaBuilder) buildSimple(value any) []map[string]any {
	return []map[string]any{
		{
			"value":          value,
			"marketplace_id": b.marketplaceID,
		},
	}
}

// buildWithLang 构建带语言标签的属性
func (b *SchemaBuilder) buildWithLang(value any) []map[string]any {
	return []map[string]any{
		{
			"value":          value,
			"language_tag":   b.languageTag,
			"marketplace_id": b.marketplaceID,
		},
	}
}

// buildNested 构建嵌套格式属性
func (b *SchemaBuilder) buildNested(attr model.AttributeInfo, value any) []map[string]any {
	// 找到嵌套的子属性名
	nestedAttrName := ""
	for _, sub := range attr.SubAttrs {
		if sub.Type == "array" && sub.Name != "marketplace_id" {
			nestedAttrName = sub.Name
			break
		}
	}

	if nestedAttrName == "" {
		return b.buildSimple(value)
	}

	return []map[string]any{
		{
			"marketplace_id": b.marketplaceID,
			nestedAttrName: []map[string]any{
				{
					"language_tag": b.languageTag,
					"value":        value,
				},
			},
		},
	}
}

// BuildBulletPoints 构建要点描述
func (b *SchemaBuilder) BuildBulletPoints(points []string) []map[string]any {
	result := make([]map[string]any, 0, len(points))
	for _, point := range points {
		result = append(result, map[string]any{
			"value":          point,
			"language_tag":   b.languageTag,
			"marketplace_id": b.marketplaceID,
		})
	}
	return result
}

// BuildProductIdentifiers 构建产品标识符
func (b *SchemaBuilder) BuildProductIdentifiers(upc, ean string) []map[string]any {
	return []map[string]any{
		{
			"type":           "UPC",
			"value":          upc,
			"marketplace_id": b.marketplaceID,
		},
		{
			"type":           "EAN",
			"value":          ean,
			"marketplace_id": b.marketplaceID,
		},
	}
}

// BuildPurchasableOffer 构建价格信息
// 根据Amazon官方文档，purchasable_offer需要包含marketplace_id
func (b *SchemaBuilder) BuildPurchasableOffer(currency string, price float64) []map[string]any {
	return []map[string]any{
		{
			"audience":       "ALL",
			"currency":       currency,
			"marketplace_id": b.marketplaceID,
			"our_price": []map[string]any{
				{
					"schedule": []map[string]any{
						{"value_with_tax": price},
					},
				},
			},
		},
	}
}

// BuildFulfillmentAvailability 构建库存信息
// 注意：根据Amazon官方文档，fulfillment_availability不需要marketplace_id
func (b *SchemaBuilder) BuildFulfillmentAvailability(quantity int) []map[string]any {
	return []map[string]any{
		{
			"fulfillment_channel_code": "DEFAULT",
			"quantity":                 quantity,
		},
	}
}

// GenerateAttributeTemplate 生成属性模板
func (b *SchemaBuilder) GenerateAttributeTemplate(schema *model.ProductTypeSchema) map[string]any {
	template := make(map[string]any)

	for attrName, propDef := range schema.Properties {
		// 为每个属性生成模板结构
		template[attrName] = []map[string]any{
			{
				"value":          "",
				"marketplace_id": b.marketplaceID,
			},
		}

		// 如果属性需要语言标签，添加language_tag
		if b.needsLanguageTag(propDef) {
			template[attrName] = []map[string]any{
				{
					"value":          "",
					"language_tag":   b.languageTag,
					"marketplace_id": b.marketplaceID,
				},
			}
		}
	}

	return template
}

// GetDefaultValues 获取默认值
func (b *SchemaBuilder) GetDefaultValues(attrs []model.AttributeInfo) map[string]any {
	defaults := make(map[string]any)

	for _, attr := range attrs {
		if attr.HasEnumValues() {
			// 使用第一个枚举值作为默认值
			defaults[attr.Name] = b.BuildAttribute(attr, attr.GetFirstEnumValue())
		} else if attr.GetFirstExample() != nil {
			// 使用第一个示例值作为默认值
			defaults[attr.Name] = b.BuildAttribute(attr, attr.GetFirstExample())
		}
	}

	return defaults
}

// needsLanguageTag 检查属性是否需要语言标签
func (b *SchemaBuilder) needsLanguageTag(propDef model.PropertyDef) bool {
	// 检查属性结构中是否包含language_tag字段
	if propDef.Items != nil && propDef.Items.Properties != nil {
		if _, hasLangTag := propDef.Items.Properties["language_tag"]; hasLangTag {
			return true
		}
	}
	return false
}

