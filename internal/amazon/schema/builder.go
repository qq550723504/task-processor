// Package schema 提供Amazon产品属性动态构建功能
package schema

// Builder 属性构建器
type Builder struct {
	marketplaceID string
	languageTag   string
}

// NewBuilder 创建属性构建器
func NewBuilder(marketplaceID, languageTag string) *Builder {
	return &Builder{
		marketplaceID: marketplaceID,
		languageTag:   languageTag,
	}
}

// MarketplaceID 获取市场ID
func (b *Builder) MarketplaceID() string {
	return b.marketplaceID
}

// BuildAttribute 根据属性信息构建属性值
func (b *Builder) BuildAttribute(attr AttributeInfo, value any) []map[string]any {
	switch attr.Format {
	case FormatSimple:
		return b.buildSimple(value)
	case FormatWithLang:
		return b.buildWithLang(value)
	case FormatNested:
		return b.buildNested(attr, value)
	default:
		return b.buildSimple(value)
	}
}

// buildSimple 构建简单格式属性
func (b *Builder) buildSimple(value any) []map[string]any {
	return []map[string]any{
		{
			"value":          value,
			"marketplace_id": b.marketplaceID,
		},
	}
}

// buildWithLang 构建带语言标签的属性
func (b *Builder) buildWithLang(value any) []map[string]any {
	return []map[string]any{
		{
			"value":          value,
			"language_tag":   b.languageTag,
			"marketplace_id": b.marketplaceID,
		},
	}
}

// buildNested 构建嵌套格式属性
func (b *Builder) buildNested(attr AttributeInfo, value any) []map[string]any {
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
func (b *Builder) BuildBulletPoints(points []string) []map[string]any {
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
func (b *Builder) BuildProductIdentifiers(upc, ean string) []map[string]any {
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
func (b *Builder) BuildPurchasableOffer(currency string, price float64) []map[string]any {
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
func (b *Builder) BuildFulfillmentAvailability(quantity int) []map[string]any {
	return []map[string]any{
		{
			"fulfillment_channel_code": "DEFAULT",
			"quantity":                 quantity,
		},
	}
}
