// Package service 提供Amazon属性构建功能
package service

import (
	"context"
	"sort"
	"strings"
	"task-processor/internal/platforms/amazon/internal/model"

	"github.com/sirupsen/logrus"
)

// AttributeBuilder 属性构建器
type AttributeBuilder struct {
	identifierService *ProductIdentifierService
	variationHandler  *VariationHandler
	logger            *logrus.Entry
}

// NewAttributeBuilder 创建属性构建器
func NewAttributeBuilder() *AttributeBuilder {
	return &AttributeBuilder{
		identifierService: NewProductIdentifierService(),
		variationHandler:  NewVariationHandler(),
		logger:            logrus.WithField("component", "AttributeBuilder"),
	}
}

// BuildAttributes 构建属性
func (ab *AttributeBuilder) BuildAttributes(
	ctx context.Context,
	builder *SchemaBuilder,
	requiredAttrs []model.AttributeInfo,
	data *model.ProductData,
	productSchema *model.ProductTypeSchema,
) map[string]any {
	attrs := make(map[string]any)
	marketplaceID := builder.MarketplaceID()
	productType := data.ProductType

	// 基础必需属性
	ab.addBasicAttributes(attrs, builder, data, marketplaceID)

	// 使用产品标识符服务处理标识符
	ab.addIdentifierAttributes(attrs, data, marketplaceID, productType)

	// 添加产品详细信息
	ab.addDetailAttributes(attrs, data, marketplaceID)

	// 处理必需属性
	ab.addRequiredAttributes(attrs, builder, requiredAttrs, data, marketplaceID)

	// 处理图片
	ab.addImageAttributes(ctx, attrs, data, marketplaceID, productSchema)

	// 处理变体信息
	ab.variationHandler.AddVariationAttributes(attrs, data, marketplaceID)

	// 添加用户自定义属性
	ab.addCustomAttributes(attrs, data, marketplaceID)

	// 确保常见必需属性都有值
	ab.ensureCommonAttributes(attrs, marketplaceID, data, productSchema)

	return attrs
}

// addBasicAttributes 添加基础属性
func (ab *AttributeBuilder) addBasicAttributes(attrs map[string]any, builder *SchemaBuilder, data *model.ProductData, marketplaceID string) {
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
	attrs["purchasable_offer"] = ab.buildPurchasableOfferWithTax(builder, data)
	attrs["fulfillment_availability"] = builder.BuildFulfillmentAvailability(data.Quantity)
}

// addIdentifierAttributes 添加标识符属性
func (ab *AttributeBuilder) addIdentifierAttributes(attrs map[string]any, data *model.ProductData, marketplaceID string, productType string) {
	identifierAttrs := ab.identifierService.BuildIdentifierAttributes(
		data.IdentifierConfig,
		data.SKU,
		marketplaceID,
		isAutomotiveCategory(productType),
		productType,
	)

	// 合并标识符属性
	for key, value := range identifierAttrs {
		attrs[key] = value
	}
}

// addDetailAttributes 添加产品详细信息属性
func (ab *AttributeBuilder) addDetailAttributes(attrs map[string]any, data *model.ProductData, marketplaceID string) {
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

// addRequiredAttributes 处理必需属性
func (ab *AttributeBuilder) addRequiredAttributes(attrs map[string]any, builder *SchemaBuilder, requiredAttrs []model.AttributeInfo, data *model.ProductData, marketplaceID string) {
	for _, attr := range requiredAttrs {
		if _, exists := attrs[attr.Name]; exists {
			continue // 已处理
		}

		// 跳过APPAREL类型不适用的属性
		if attr.Name == "target_audience" {
			continue
		}

		// 对closure属性使用特殊处理
		if attr.Name == "closure" {
			// 检查用户是否已设置closure属性
			if _, exists := data.Attributes["closure"]; !exists {
				// 使用默认值提供器构建closure属性
				provider := NewDefaultValueProvider(marketplaceID, "en_US", data.Brand)
				if defaultVal := provider.GetDefaultValue(attr.Name); defaultVal != nil {
					attrs[attr.Name] = defaultVal
				}
			}
			continue
		}

		// 从用户数据获取
		if value, ok := data.Attributes[attr.Name]; ok {
			attrs[attr.Name] = builder.BuildAttribute(attr, value)
			continue
		}

		// 使用默认值
		defaultValue := ab.getDefaultValue(attr, marketplaceID)
		if defaultValue != nil {
			attrs[attr.Name] = defaultValue
		}
	}
}

// addImageAttributes 添加图片属性
func (ab *AttributeBuilder) addImageAttributes(ctx context.Context, attrs map[string]any, data *model.ProductData, marketplaceID string, productSchema *model.ProductTypeSchema) {
	// 处理主图 - Amazon支持直接使用外部图片URL
	if data.MainImageURL != "" {
		attrs["main_product_image_locator"] = []map[string]any{
			{
				"media_location": data.MainImageURL,
				"marketplace_id": marketplaceID,
			},
		}
		ab.logger.WithField("main_image_url", data.MainImageURL).Info("设置主图URL")
	}

	// 处理附加图片 - 根据Schema判断支持的图片属性
	if len(data.AdditionalImages) > 0 {
		ab.addAdditionalImagesBySchema(attrs, data.AdditionalImages, marketplaceID, productSchema)
	}
}

// addAdditionalImagesBySchema 根据Schema添加附加图片
func (ab *AttributeBuilder) addAdditionalImagesBySchema(attrs map[string]any, imageURLs []string, marketplaceID string, productSchema *model.ProductTypeSchema) {
	if productSchema == nil {
		ab.logger.Warn("无Schema信息，跳过附加图片设置")
		return
	}

	// 检查Schema中支持的图片属性
	supportedImageAttrs := ab.getSupportedImageAttributes(productSchema)
	if len(supportedImageAttrs) == 0 {
		ab.logger.Info("该产品类型不支持附加图片属性")
		return
	}

	// 构建图片数据
	var imageData []map[string]any
	maxImages := 9 // Amazon最多支持9张附加图片

	for i, imageURL := range imageURLs {
		if i >= maxImages {
			ab.logger.WithField("total_images", len(imageURLs)).
				Warn("附加图片数量超过Amazon限制，只使用前9张")
			break
		}

		if imageURL != "" {
			imageData = append(imageData, map[string]any{
				"media_location": imageURL,
				"marketplace_id": marketplaceID,
			})
		}
	}

	if len(imageData) == 0 {
		return
	}

	// 如果有带数字后缀的属性（如 other_product_image_locator_1），分别设置每张图片
	if len(supportedImageAttrs) > 1 && strings.Contains(supportedImageAttrs[0], "_") {
		ab.distributeImagesToNumberedAttrs(attrs, imageData, supportedImageAttrs, marketplaceID)
	} else if len(supportedImageAttrs) > 0 {
		// 使用第一个支持的图片属性
		selectedAttr := supportedImageAttrs[0]
		attrs[selectedAttr] = imageData

		ab.logger.WithFields(logrus.Fields{
			"attribute": selectedAttr,
			"count":     len(imageData),
		}).Info("设置附加图片URL")
	}
}

// getSupportedImageAttributes 获取Schema中支持的图片属性
func (ab *AttributeBuilder) getSupportedImageAttributes(productSchema *model.ProductTypeSchema) []string {
	var supportedAttrs []string
	var allImageAttrs []string

	// 扫描Schema中所有包含"image"的属性
	for attrName := range productSchema.Properties {
		if strings.Contains(strings.ToLower(attrName), "image") {
			allImageAttrs = append(allImageAttrs, attrName)
		}
	}

	// 记录所有找到的图片相关属性
	ab.logger.WithFields(logrus.Fields{
		"all_image_attrs": allImageAttrs,
	}).Info("Schema中所有图片相关属性")

	// 从所有图片属性中筛选出附加图片属性（排除主图）
	for _, attrName := range allImageAttrs {
		// 排除主图属性
		if strings.Contains(attrName, "main_") {
			continue
		}
		// 优先选择 other_product_image_locator 系列
		if strings.HasPrefix(attrName, "other_product_image_locator") {
			supportedAttrs = append(supportedAttrs, attrName)
		}
	}

	// 如果没有找到 other_product_image_locator 系列，尝试其他图片属性
	if len(supportedAttrs) == 0 {
		for _, attrName := range allImageAttrs {
			if strings.Contains(attrName, "main_") {
				continue
			}
			if strings.Contains(attrName, "swatch_") ||
				strings.Contains(attrName, "other_") ||
				strings.Contains(attrName, "additional_") {
				supportedAttrs = append(supportedAttrs, attrName)
			}
		}
	}

	if len(supportedAttrs) > 0 {
		ab.logger.WithFields(logrus.Fields{
			"supported_attrs": supportedAttrs,
		}).Info("找到支持的附加图片属性")
	} else {
		ab.logger.Info("该产品类型仅支持主图，不支持附加图片")
	}

	return supportedAttrs
}

// distributeImagesToNumberedAttrs 将图片分配到带数字后缀的属性中
func (ab *AttributeBuilder) distributeImagesToNumberedAttrs(attrs map[string]any, imageData []map[string]any, supportedAttrs []string, marketplaceID string) {
	// 按数字后缀排序属性
	sort.Slice(supportedAttrs, func(i, j int) bool {
		return supportedAttrs[i] < supportedAttrs[j]
	})

	// 为每张图片分配一个属性
	assignedCount := 0
	for i, imageItem := range imageData {
		if i >= len(supportedAttrs) {
			ab.logger.WithField("total_images", len(imageData)).
				WithField("max_attrs", len(supportedAttrs)).
				Warn("图片数量超过可用属性数量，部分图片将被忽略")
			break
		}

		attrName := supportedAttrs[i]
		attrs[attrName] = []map[string]any{imageItem}
		assignedCount++
	}

	ab.logger.WithFields(logrus.Fields{
		"assigned_count": assignedCount,
		"total_images":   len(imageData),
		"attributes":     supportedAttrs[:assignedCount],
	}).Info("设置附加图片到带数字后缀的属性")
}

// addCustomAttributes 添加用户自定义属性
func (ab *AttributeBuilder) addCustomAttributes(attrs map[string]any, data *model.ProductData, marketplaceID string) {
	for attrName, attrValue := range data.Attributes {
		if _, exists := attrs[attrName]; !exists {
			// 清理属性值中的特殊字符
			cleanValue := ab.sanitizeAttributeValue(attrValue)

			// 构建标准格式的属性值
			attrs[attrName] = []map[string]any{
				{"value": cleanValue, "marketplace_id": marketplaceID},
			}
		}
	}
}

// sanitizeAttributeValue 清理属性值中可能导致API错误的特殊字符
func (ab *AttributeBuilder) sanitizeAttributeValue(value any) any {
	if str, ok := value.(string); ok {
		// 替换可能导致问题的特殊字符
		cleaned := strings.ReplaceAll(str, "<", "less than ")
		cleaned = strings.ReplaceAll(cleaned, ">", "greater than ")
		cleaned = strings.ReplaceAll(cleaned, "&", "and")

		// 如果值被修改了，记录日志
		if cleaned != str {
			ab.logger.WithFields(logrus.Fields{
				"original": str,
				"cleaned":  cleaned,
			}).Info("清理属性值中的特殊字符")
		}

		return cleaned
	}
	return value
}

// ensureCommonAttributes 确保常见必需属性都有值（完全通用，不限制品类）
func (ab *AttributeBuilder) ensureCommonAttributes(attrs map[string]any, marketplaceID string, data *model.ProductData, productSchema *model.ProductTypeSchema) {
	// 如果没有Schema，不添加额外属性
	if productSchema == nil {
		return
	}

	// 获取Schema中定义的所有属性名
	schemaAttrs := make(map[string]bool)
	for name := range productSchema.Properties {
		schemaAttrs[name] = true
	}

	// 只添加最基本的通用属性，其他属性让用户通过Attributes字段自定义
	basicDefaults := map[string]func() any{
		"country_of_origin": func() any {
			if data.CountryOfOrigin != "" {
				return []map[string]any{{"value": data.CountryOfOrigin, "marketplace_id": marketplaceID}}
			}
			return []map[string]any{{"value": "CN", "marketplace_id": marketplaceID}}
		},
		"import_designation": func() any {
			return []map[string]any{{"value": "imported", "marketplace_id": marketplaceID}}
		},
	}

	// 只添加Schema中存在且尚未设置的基本属性
	for name, valueFn := range basicDefaults {
		if schemaAttrs[name] {
			if _, exists := attrs[name]; !exists {
				attrs[name] = valueFn()
			}
		}
	}
}

// getDefaultValue 获取属性默认值（完全基于Schema，不硬编码）
func (ab *AttributeBuilder) getDefaultValue(attr model.AttributeInfo, marketplaceID string) any {
	// 如果有枚举值，使用第一个
	if len(attr.EnumValues) > 0 {
		return []map[string]any{
			{"value": attr.EnumValues[0], "marketplace_id": marketplaceID},
		}
	}

	// 如果有示例值，使用第一个
	if len(attr.Examples) > 0 {
		return []map[string]any{
			{"value": attr.Examples[0], "marketplace_id": marketplaceID},
		}
	}

	// 其他情况返回nil，让Amazon API告诉我们缺少什么
	return nil
}

// buildPurchasableOfferWithTax 构建包含税务信息的价格报价
// 根据Amazon官方文档，purchasable_offer需要包含marketplace_id
func (ab *AttributeBuilder) buildPurchasableOfferWithTax(builder *SchemaBuilder, data *model.ProductData) any {
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
	ab.logger.WithFields(logrus.Fields{
		"price_with_tax": data.Price,
		"currency":       data.Currency,
	}).Info("设置商品价格（Amazon API仅支持含税价格）")

	// 如果用户提供了不含税价格信息，记录警告
	if data.PriceExcludingTax > 0 || data.TaxRate > 0 {
		ab.logger.WithFields(logrus.Fields{
			"price_excluding_tax": data.PriceExcludingTax,
			"tax_rate":            data.TaxRate,
		}).Warn("Amazon Listings API不支持直接设置不含税价格，不含税价格需要通过税务报告查看")
	}

	return []map[string]any{offer}
}

// isAutomotiveCategory 判断是否为汽配类目（保留用于产品标识符处理）
func isAutomotiveCategory(productType string) bool {
	automotiveTypes := map[string]bool{
		"AUTO_ACCESSORY": true,
		"AUTO_PART":      true,
		"AUTOMOTIVE":     true,
	}
	return automotiveTypes[productType]
}
