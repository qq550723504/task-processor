// Package amazon 提供基于Amazon Schema的属性构建功能
package amazon

// SchemaBasedAttributeBuilder 基于Schema的属性构建器
type SchemaBasedAttributeBuilder struct {
	marketplaceID string
}

// NewSchemaBasedAttributeBuilder 创建基于Schema的属性构建器
func NewSchemaBasedAttributeBuilder(marketplaceID string) *SchemaBasedAttributeBuilder {
	return &SchemaBasedAttributeBuilder{
		marketplaceID: marketplaceID,
	}
}

// BuildLuggageAttributes 构建LUGGAGE产品类型的完整属性
func (sb *SchemaBasedAttributeBuilder) BuildLuggageAttributes(asin, sku string, price float64) map[string]any {
	return map[string]any{
		// Product Identity 组 - 必需
		"item_name": []map[string]any{
			{
				"value":          "Test Luggage Product",
				"language_tag":   "en_US",
				"marketplace_id": sb.marketplaceID,
			},
		},
		"brand": []map[string]any{
			{
				"value":          "TestBrand",
				"language_tag":   "en_US",
				"marketplace_id": sb.marketplaceID,
			},
		},
		"merchant_suggested_asin": []map[string]any{
			{
				"value":          asin,
				"marketplace_id": sb.marketplaceID,
			},
		},

		// Offer 组 - 必需
		"condition_type": []map[string]any{
			{
				"value":          "new_new",
				"marketplace_id": sb.marketplaceID,
			},
		},
		"purchasable_offer": []map[string]any{
			{
				"audience":       "ALL",
				"currency":       "USD",
				"marketplace_id": sb.marketplaceID,
				"our_price": []map[string]any{
					{
						"schedule": []map[string]any{
							{"value_with_tax": price},
						},
					},
				},
			},
		},
		"fulfillment_availability": []map[string]any{
			{
				"fulfillment_channel_code": "DEFAULT",
				"quantity":                 10,
			},
		},

		// Safety & Compliance 组 - 通常必需
		"country_of_origin": []map[string]any{
			{
				"value":          "CN",
				"marketplace_id": sb.marketplaceID,
			},
		},
	}
}

// BuildMinimalValidAttributes 构建最小有效属性集
func (sb *SchemaBasedAttributeBuilder) BuildMinimalValidAttributes(productType, asin string, price float64) map[string]any {
	switch productType {
	case "LUGGAGE", "SUITCASE":
		return sb.BuildLuggageAttributes(asin, "", price)
	default:
		// 通用最小属性集
		return map[string]any{
			"item_name": []map[string]any{
				{
					"value":          "Test Product",
					"language_tag":   "en_US",
					"marketplace_id": sb.marketplaceID,
				},
			},
			"brand": []map[string]any{
				{
					"value":          "TestBrand",
					"language_tag":   "en_US",
					"marketplace_id": sb.marketplaceID,
				},
			},
			"condition_type": []map[string]any{
				{
					"value":          "new_new",
					"marketplace_id": sb.marketplaceID,
				},
			},
			"merchant_suggested_asin": []map[string]any{
				{
					"value":          asin,
					"marketplace_id": sb.marketplaceID,
				},
			},
		}
	}
}
