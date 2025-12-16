// Package amazon 提供Amazon动态产品模板
package amazon

import (
	"context"
	"task-processor/internal/amazon/schema"
	"task-processor/platforms/amazon/api"

	"github.com/sirupsen/logrus"
)

// DynamicTemplate 动态产品模板（支持任意品类）
type DynamicTemplate struct {
	schemaManager    *SchemaManager
	attributeBuilder *AttributeBuilder
	logger           *logrus.Entry
}

// NewDynamicTemplate 创建动态模板
func NewDynamicTemplate(apiClient *api.Client) *DynamicTemplate {
	return &DynamicTemplate{
		schemaManager:    NewSchemaManager(apiClient),
		attributeBuilder: NewAttributeBuilder(),
		logger:           logrus.WithField("component", "DynamicTemplate"),
	}
}

// BuildListingRequest 动态构建商品请求
func (t *DynamicTemplate) BuildListingRequest(
	ctx context.Context,
	productType string,
	marketplaceID string,
	data *ProductData,
) (*api.ListingRequest, error) {
	// 获取产品类型Schema
	productSchema, err := t.schemaManager.GetProductTypeSchema(ctx, productType)
	if err != nil {
		t.logger.WithError(err).Warn("获取Schema失败，使用通用模板")
		return t.buildGenericRequest(productType, marketplaceID, data), nil
	}

	// 解析必需属性
	requiredAttrs := t.schemaManager.GetRequiredAttributes(productSchema)
	t.logger.WithField("required_count", len(requiredAttrs)).Info("解析必需属性")

	// 构建属性
	builder := schema.NewBuilder(marketplaceID, "en_US")
	attributes := t.attributeBuilder.BuildAttributes(ctx, builder, requiredAttrs, data, productSchema)

	return &api.ListingRequest{
		SKU:          data.SKU,
		ProductType:  productType,
		Requirements: "LISTING",
		Attributes:   attributes,
	}, nil
}

// buildGenericRequest 构建通用请求（Schema获取失败时使用）
func (t *DynamicTemplate) buildGenericRequest(
	productType string,
	marketplaceID string,
	data *ProductData,
) *api.ListingRequest {
	builder := schema.NewBuilder(marketplaceID, "en_US")

	// 使用属性构建器构建基础属性
	attrs := t.attributeBuilder.BuildAttributes(context.Background(), builder, []schema.AttributeInfo{}, data, nil)

	return &api.ListingRequest{
		SKU:          data.SKU,
		ProductType:  productType,
		Requirements: "LISTING",
		Attributes:   attrs,
	}
}
