// Package service 提供Amazon Schema管理功能
package service

import (
	"context"
	"fmt"
	"sync"
	"task-processor/internal/platforms/amazon/api"
	"task-processor/internal/platforms/amazon/core/model"

	"github.com/sirupsen/logrus"
)

// SchemaManager Schema管理器
type SchemaManager struct {
	apiClient     *api.Client
	schemaFetcher *SchemaFetcher
	schemaParser  *SchemaParser
	schemaBuilder *SchemaBuilder
	logger        *logrus.Entry
	schemaCache   sync.Map
}

// NewSchemaManager 创建Schema管理器
func NewSchemaManager(apiClient *api.Client, marketplaceID, languageTag string) *SchemaManager {
	return &SchemaManager{
		apiClient:     apiClient,
		schemaFetcher: NewSchemaFetcher(),
		schemaParser:  NewSchemaParser(),
		schemaBuilder: NewSchemaBuilder(marketplaceID, languageTag),
		logger:        logrus.WithField("service", "SchemaManager"),
	}
}

// GetProductTypeSchema 获取产品类型Schema
func (m *SchemaManager) GetProductTypeSchema(ctx context.Context, productType string) (*model.ProductTypeSchema, error) {
	m.logger.WithField("product_type", productType).Info("获取产品类型Schema")

	// 检查缓存
	if cached, ok := m.schemaCache.Load(productType); ok {
		if schema, ok := cached.(*model.ProductTypeSchema); ok {
			m.logger.Info("使用缓存的Schema")
			return schema, nil
		}
	}

	// 通过API获取产品类型定义
	definition, err := m.apiClient.GetProductTypeDefinition(ctx, productType)
	if err != nil {
		return nil, fmt.Errorf("获取产品类型定义失败: %w", err)
	}

	// 如果有Schema链接，下载Schema
	var schema *model.ProductTypeSchema
	if definition.Schema != nil && definition.Schema.Link != nil {
		schemaURL := m.extractSchemaURL(definition.Schema.Link)
		if schemaURL != "" {
			schema, err = m.schemaFetcher.FetchSchema(ctx, schemaURL)
			if err != nil {
				return nil, fmt.Errorf("获取Schema失败: %w", err)
			}
		}
	}

	// 如果没有Schema链接，使用内置的Schema
	if schema == nil {
		schema = m.buildDefaultSchema(definition)
	}

	// 验证Schema
	if err := m.schemaFetcher.ValidateSchema(schema); err != nil {
		return nil, fmt.Errorf("Schema验证失败: %w", err)
	}

	// 缓存Schema
	m.schemaCache.Store(productType, schema)

	m.logger.WithField("properties_count", len(schema.Properties)).Info("Schema获取成功")
	return schema, nil
}

// ParseProductTypeAttributes 解析产品类型属性
func (m *SchemaManager) ParseProductTypeAttributes(ctx context.Context, productType string) ([]model.AttributeInfo, error) {
	schema, err := m.GetProductTypeSchema(ctx, productType)
	if err != nil {
		return nil, err
	}

	return m.schemaParser.ParseAttributes(schema), nil
}

// BuildProductAttributes 构建产品属性
func (m *SchemaManager) BuildProductAttributes(ctx context.Context, productType string, values map[string]any) (map[string][]map[string]any, error) {
	schema, err := m.GetProductTypeSchema(ctx, productType)
	if err != nil {
		return nil, err
	}

	template := m.schemaBuilder.GenerateAttributeTemplate(schema)

	// 转换为正确的返回类型
	result := make(map[string][]map[string]any)
	for key, value := range template {
		if arrayValue, ok := value.([]map[string]any); ok {
			result[key] = arrayValue
		}
	}

	return result, nil
}

// ValidateProductData 验证产品数据
func (m *SchemaManager) ValidateProductData(ctx context.Context, productType string, data map[string]any) error {
	attrs, err := m.ParseProductTypeAttributes(ctx, productType)
	if err != nil {
		return fmt.Errorf("解析属性失败: %w", err)
	}

	var errors []error
	for _, attr := range attrs {
		if value, exists := data[attr.Name]; exists {
			if err := m.schemaParser.ValidateAttributeValue(attr, value); err != nil {
				errors = append(errors, err)
			}
		} else if attr.Required {
			errors = append(errors, fmt.Errorf("缺少必需属性: %s", attr.Name))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("数据验证失败: %v", errors)
	}

	return nil
}

// GetAttributeTemplate 获取属性模板
func (m *SchemaManager) GetAttributeTemplate(ctx context.Context, productType string) (map[string]any, error) {
	schema, err := m.GetProductTypeSchema(ctx, productType)
	if err != nil {
		return nil, err
	}

	return m.schemaBuilder.GenerateAttributeTemplate(schema), nil
}

// GetDefaultValues 获取默认值
func (m *SchemaManager) GetDefaultValues(ctx context.Context, productType string) (map[string]any, error) {
	attrs, err := m.ParseProductTypeAttributes(ctx, productType)
	if err != nil {
		return nil, err
	}

	return m.schemaBuilder.GetDefaultValues(attrs), nil
}

// GetRequiredAttributes 获取必需属性
func (m *SchemaManager) GetRequiredAttributes(ctx context.Context, productType string) ([]model.AttributeInfo, error) {
	attrs, err := m.ParseProductTypeAttributes(ctx, productType)
	if err != nil {
		return nil, err
	}

	return m.schemaParser.GetRequiredAttributes(attrs), nil
}

// AnalyzeProductTypeComplexity 分析产品类型复杂度
func (m *SchemaManager) AnalyzeProductTypeComplexity(ctx context.Context, productType string) (map[string]any, error) {
	attrs, err := m.ParseProductTypeAttributes(ctx, productType)
	if err != nil {
		return nil, err
	}

	analysis := map[string]any{
		"total_attributes":    len(attrs),
		"required_count":      len(m.schemaParser.GetRequiredAttributes(attrs)),
		"optional_count":      len(m.schemaParser.GetOptionalAttributes(attrs)),
		"simple_format":       len(m.schemaParser.GetAttributesByFormat(attrs, model.FormatSimple)),
		"with_lang_format":    len(m.schemaParser.GetAttributesByFormat(attrs, model.FormatWithLang)),
		"nested_format":       len(m.schemaParser.GetAttributesByFormat(attrs, model.FormatNested)),
		"complex_format":      len(m.schemaParser.GetAttributesByFormat(attrs, model.FormatComplex)),
		"has_enum_attributes": m.countEnumAttributes(attrs),
		"complexity_score":    m.calculateComplexityScore(attrs),
	}

	return analysis, nil
}

// extractSchemaURL 从Schema链接中提取URL
func (m *SchemaManager) extractSchemaURL(link any) string {
	if linkMap, ok := link.(map[string]any); ok {
		if resource, ok := linkMap["resource"].(string); ok {
			return resource
		}
	}
	if linkStr, ok := link.(string); ok {
		return linkStr
	}
	return ""
}

// buildDefaultSchema 构建默认Schema
func (m *SchemaManager) buildDefaultSchema(_ *api.ProductTypeDefinition) *model.ProductTypeSchema {
	// 基于产品类型定义构建一个基本的Schema
	schema := &model.ProductTypeSchema{
		Properties: make(map[string]model.PropertyDef),
		Required:   []string{},
	}

	// 添加基本属性
	basicAttributes := []string{
		"item_name", "brand", "manufacturer", "product_description",
		"bullet_point", "item_type_name", "product_type",
	}

	for _, attr := range basicAttributes {
		schema.Properties[attr] = model.PropertyDef{
			Type:        "array",
			Description: fmt.Sprintf("%s attribute", attr),
			Items: &model.ItemsDef{
				Properties: map[string]any{
					"value": map[string]any{
						"type": "string",
					},
					"marketplace_id": map[string]any{
						"type": "string",
					},
				},
				Required: []string{"value", "marketplace_id"},
			},
		}
	}

	// 设置必需属性
	schema.Required = []string{"item_name", "brand", "product_description"}

	return schema
}

// countEnumAttributes 统计枚举属性数量
func (m *SchemaManager) countEnumAttributes(attrs []model.AttributeInfo) int {
	count := 0
	for _, attr := range attrs {
		if len(attr.EnumValues) > 0 {
			count++
		}
	}
	return count
}

// calculateComplexityScore 计算复杂度分数
func (m *SchemaManager) calculateComplexityScore(attrs []model.AttributeInfo) int {
	score := 0
	for _, attr := range attrs {
		// 基础分数
		score += 1

		// 必需属性加分
		if attr.Required {
			score += 2
		}

		// 格式复杂度加分
		switch attr.Format {
		case model.FormatWithLang:
			score += 1
		case model.FormatNested:
			score += 2
		case model.FormatComplex:
			score += 3
		}

		// 子属性加分
		score += len(attr.SubAttrs)

		// 枚举值加分
		if len(attr.EnumValues) > 0 {
			score += 1
		}
	}

	return score
}

// ClearCache 清理缓存
func (m *SchemaManager) ClearCache() {
	m.schemaCache = sync.Map{}
	m.schemaFetcher.ClearCache()
	m.logger.Info("Schema管理器缓存已清理")
}

// GetCacheStats 获取缓存统计
func (m *SchemaManager) GetCacheStats() map[string]any {
	schemaCount := 0
	m.schemaCache.Range(func(key, value any) bool {
		schemaCount++
		return true
	})

	fetcherStats := m.schemaFetcher.GetCacheStats()

	return map[string]any{
		"schema_cache":  schemaCount,
		"fetcher_cache": fetcherStats,
	}
}
