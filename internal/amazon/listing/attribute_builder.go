// Package listing 提供Amazon属性构建功能
package listing

import (
	"context"
	"task-processor/internal/amazon/model"
	"task-processor/internal/amazon/schema"

	"github.com/sirupsen/logrus"
)

// AttributeBuilder 属性构建器
type AttributeBuilder struct {
	identifierService *ProductIdentifierService
	variationHandler  *VariationHandler
	basicBuilder      *BasicAttributeBuilder
	imageBuilder      *ImageAttributeBuilder
	requiredBuilder   *RequiredAttributeBuilder
	customBuilder     *CustomAttributeBuilder
	logger            *logrus.Entry
}

// NewAttributeBuilder 创建属性构建器
func NewAttributeBuilder() *AttributeBuilder {
	return &AttributeBuilder{
		identifierService: NewProductIdentifierService(),
		variationHandler:  NewVariationHandler(),
		basicBuilder:      NewBasicAttributeBuilder(),
		imageBuilder:      NewImageAttributeBuilder(),
		requiredBuilder:   NewRequiredAttributeBuilder(),
		customBuilder:     NewCustomAttributeBuilder(),
		logger:            logrus.WithField("component", "AttributeBuilder"),
	}
}

// BuildAttributes 构建属性
func (ab *AttributeBuilder) BuildAttributes(
	ctx context.Context,
	builder *schema.SchemaBuilder,
	requiredAttrs []model.AttributeInfo,
	data *model.ProductData,
	productSchema *model.ProductTypeSchema,
) map[string]any {
	attrs := make(map[string]any)
	marketplaceID := builder.MarketplaceID()
	productType := data.ProductType

	// 基础必需属性
	ab.basicBuilder.AddBasicAttributes(attrs, builder, data, marketplaceID)

	// 使用产品标识符服务处理标识符
	ab.addIdentifierAttributes(attrs, data, marketplaceID, productType)

	// 添加产品详细信息
	ab.basicBuilder.AddDetailAttributes(attrs, data, marketplaceID)

	// 处理必需属性
	ab.requiredBuilder.AddRequiredAttributes(attrs, builder, requiredAttrs, data, marketplaceID)

	// 处理图片
	ab.imageBuilder.AddImageAttributes(ctx, attrs, data, marketplaceID, productSchema)

	// 处理变体信息
	ab.variationHandler.AddVariationAttributes(attrs, data, marketplaceID)

	// 添加用户自定义属性
	ab.customBuilder.AddCustomAttributes(attrs, data, marketplaceID)

	// 确保常见必需属性都有值
	ab.requiredBuilder.EnsureCommonAttributes(attrs, marketplaceID, data, productSchema)

	return attrs
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

// isAutomotiveCategory 判断是否为汽配类目（保留用于产品标识符处理）
func isAutomotiveCategory(productType string) bool {
	automotiveTypes := map[string]bool{
		"AUTO_ACCESSORY": true,
		"AUTO_PART":      true,
		"AUTOMOTIVE":     true,
	}
	return automotiveTypes[productType]
}
