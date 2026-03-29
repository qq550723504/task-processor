// Package listing 提供Amazon动态产品模板
package listing

import (
	"context"
	"task-processor/internal/amazon/api"
	"task-processor/internal/amazon/model"
	"task-processor/internal/amazon/schema"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// DynamicTemplate 动态产品模板（支持任意品类）
type DynamicTemplate struct {
	schemaManager    *schema.SchemaManager
	attributeBuilder *AttributeBuilder
	logger           *logrus.Entry
}

// NewDynamicTemplate 创建动态模板
func NewDynamicTemplate(apiClient *api.Client, marketplaceID, languageTag string) *DynamicTemplate {
	return &DynamicTemplate{
		schemaManager:    schema.NewSchemaManager(apiClient, marketplaceID, languageTag),
		attributeBuilder: NewAttributeBuilder(),
		logger:           logger.GetGlobalLogger("DynamicTemplate"),
	}
}

// BuildListingRequest 动态构建商品请求
func (t *DynamicTemplate) BuildListingRequest(
	ctx context.Context,
	productType string,
	marketplaceID string,
	data *model.ProductData,
) (*api.ListingRequest, error) {
	// 获取产品类型Schema
	productSchema, err := t.schemaManager.GetProductTypeSchema(ctx, productType)
	if err != nil {
		t.logger.WithError(err).Warn("获取Schema失败，使用通用模板")
		return t.buildGenericRequest(ctx, productType, marketplaceID, data), nil
	}

	// 解析必需属性
	requiredAttrs, err := t.schemaManager.ParseProductTypeAttributes(ctx, productType)
	if err != nil {
		t.logger.WithError(err).Warn("解析必需属性失败，使用通用模板")
		return t.buildGenericRequest(ctx, productType, marketplaceID, data), nil
	}

	t.logger.WithField("required_count", len(requiredAttrs)).Info("解析必需属性")

	// 构建属性
	builder := schema.NewSchemaBuilder(marketplaceID, "en_US")
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
	ctx context.Context,
	productType string,
	marketplaceID string,
	data *model.ProductData,
) *api.ListingRequest {
	builder := schema.NewSchemaBuilder(marketplaceID, "en_US")

	// 使用属性构建器构建基础属性
	attrs := t.attributeBuilder.BuildAttributes(ctx, builder, []model.AttributeInfo{}, data, nil)

	return &api.ListingRequest{
		SKU:          data.SKU,
		ProductType:  productType,
		Requirements: "LISTING",
		Attributes:   attrs,
	}
}

// ValidateProductData 验证产品数据
func (t *DynamicTemplate) ValidateProductData(ctx context.Context, productType string, data *model.ProductData) error {
	// 基础数据验证
	if err := data.Validate(); err != nil {
		return err
	}

	// Schema验证
	return t.schemaManager.ValidateProductData(ctx, productType, data.Attributes)
}

// GetAttributeTemplate 获取属性模板
func (t *DynamicTemplate) GetAttributeTemplate(ctx context.Context, productType string) (map[string]any, error) {
	return t.schemaManager.GetAttributeTemplate(ctx, productType)
}

// GetRequiredAttributes 获取必需属性
func (t *DynamicTemplate) GetRequiredAttributes(ctx context.Context, productType string) ([]model.AttributeInfo, error) {
	return t.schemaManager.GetRequiredAttributes(ctx, productType)
}

// AnalyzeComplexity 分析产品类型复杂度
func (t *DynamicTemplate) AnalyzeComplexity(ctx context.Context, productType string) (map[string]any, error) {
	return t.schemaManager.AnalyzeProductTypeComplexity(ctx, productType)
}
