// Package listing 提供Amazon基础属性构建功能
package listing

import (
	"strings"
	"task-processor/internal/amazon/model"
	"task-processor/internal/amazon/schema"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// BasicAttributeBuilder 基础属性构建器
type BasicAttributeBuilder struct {
	logger *logrus.Entry
}

// NewBasicAttributeBuilder 创建基础属性构建器
func NewBasicAttributeBuilder() *BasicAttributeBuilder {
	return &BasicAttributeBuilder{
		logger: logger.GetGlobalLogger("BasicAttributeBuilder"),
	}
}

// AddBasicAttributes 添加基础属性
func (bab *BasicAttributeBuilder) AddBasicAttributes(attrs map[string]any, builder *schema.SchemaBuilder, data *model.ProductData, marketplaceID string) {
	attrs["item_name"] = builder.BuildAttribute(
		model.AttributeInfo{Format: model.FormatWithLang},
		data.Title,
	)

	attrs["brand"] = builder.BuildAttribute(
		model.AttributeInfo{Format: model.FormatWithLang},
		data.Brand,
	)

	attrs["condition_type"] = builder.BuildAttribute(
		model.AttributeInfo{Format: model.FormatSimple},
		data.Condition,
	)

	// 价格和库存
	attrs["purchasable_offer"] = bab.buildPurchasableOfferWithTax(builder, data)
	attrs["fulfillment_availability"] = builder.BuildFulfillmentAvailability(data.Quantity)
}

// AddDetailAttributes 添加产品详细信息属性
func (bab *BasicAttributeBuilder) AddDetailAttributes(attrs map[string]any, data *model.ProductData, marketplaceID string) {
	// 描述
	if data.Description != "" {
		attrs["product_description"] = []map[string]any{
			{"value": data.Description, "language_tag": "en_US", "marketplace_id": marketplaceID},
		}
	}

	// 要点
	if len(data.BulletPoints) > 0 {
		var bulletPoints []map[string]any
		for _, point := range data.BulletPoints {
			bulletPoints = append(bulletPoints, map[string]any{
				"value":          point,
				"language_tag":   "en_US",
				"marketplace_id": marketplaceID,
			})
		}
		attrs["bullet_point"] = bulletPoints
	}

	// 型号信息
	modelName := data.Title
	if data.ModelNumber != "" {
		modelName = data.ModelNumber
	}
	attrs["model_name"] = []map[string]any{
		{"value": modelName, "marketplace_id": marketplaceID},
	}
	attrs["model_number"] = []map[string]any{
		{"value": data.SKU, "marketplace_id": marketplaceID},
	}

	// 制造商信息
	if data.Manufacturer != "" {
		attrs["manufacturer"] = []map[string]any{
			{"value": data.Manufacturer, "language_tag": "en_US", "marketplace_id": marketplaceID},
		}
	}

	// 搜索关键词
	if len(data.SearchTerms) > 0 {
		attrs["generic_keyword"] = []map[string]any{
			{"value": strings.Join(data.SearchTerms, " "), "language_tag": "en_US", "marketplace_id": marketplaceID},
		}
	}

	// 目标受众
	if data.TargetAudience != "" {
		attrs["target_audience"] = []map[string]any{
			{"value": data.TargetAudience, "marketplace_id": marketplaceID},
		}
	}

	// 原产国
	if data.CountryOfOrigin != "" {
		attrs["country_of_origin"] = []map[string]any{
			{"value": data.CountryOfOrigin, "marketplace_id": marketplaceID},
		}
	} else {
		// 默认设置为中国
		attrs["country_of_origin"] = []map[string]any{
			{"value": "CN", "marketplace_id": marketplaceID},
		}
	}

	// 材质信息
	if len(data.Materials) > 0 {
		attrs["material"] = []map[string]any{
			{"value": strings.Join(data.Materials, ", "), "marketplace_id": marketplaceID},
		}
	}

	// 安全警告
	if len(data.SafetyWarnings) > 0 {
		attrs["safety_warning"] = []map[string]any{
			{"value": strings.Join(data.SafetyWarnings, "; "), "language_tag": "en_US", "marketplace_id": marketplaceID},
		}
	}
}

// buildPurchasableOfferWithTax 构建包含税务信息的价格报价
// 根据Amazon官方文档，purchasable_offer需要包含marketplace_id
func (bab *BasicAttributeBuilder) buildPurchasableOfferWithTax(builder *schema.SchemaBuilder, data *model.ProductData) any {
	marketplaceID := builder.MarketplaceID()

	offer := map[string]any{
		"audience":       "ALL",
		"currency":       data.Currency,
		"marketplace_id": marketplaceID,
		"our_price": []map[string]any{
			{
				"schedule": []map[string]any{
					{"value_with_tax": data.Price},
				},
			},
		},
	}

	// 记录价格信息（Amazon API只支持含税价格）
	bab.logger.WithFields(logrus.Fields{
		"price_with_tax": data.Price,
		"currency":       data.Currency,
	}).Info("设置商品价格（Amazon API仅支持含税价格）")

	// 如果用户提供了不含税价格信息，记录警告
	if data.PriceExcludingTax > 0 || data.TaxRate > 0 {
		bab.logger.WithFields(logrus.Fields{
			"price_excluding_tax": data.PriceExcludingTax,
			"tax_rate":            data.TaxRate,
		}).Warn("Amazon Listings API不支持直接设置不含税价格，不含税价格需要通过税务报告查看")
	}

	return []map[string]any{offer}
}
