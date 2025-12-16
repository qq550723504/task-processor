// Package amazon 提供Amazon Schema管理功能
package amazon

import (
	"context"
	"fmt"
	"task-processor/internal/amazon/schema"
	"task-processor/platforms/amazon/api"

	"github.com/sirupsen/logrus"
)

// SchemaManager Schema管理器
type SchemaManager struct {
	apiClient     *api.Client
	schemaFetcher *schema.Fetcher
	schemaParser  *schema.Parser
	logger        *logrus.Entry
	schemaCache   map[string]*schema.ProductTypeSchema
}

// NewSchemaManager 创建Schema管理器
func NewSchemaManager(apiClient *api.Client) *SchemaManager {
	return &SchemaManager{
		apiClient:     apiClient,
		schemaFetcher: schema.NewFetcher(),
		schemaParser:  schema.NewParser(),
		logger:        logrus.WithField("component", "SchemaManager"),
		schemaCache:   make(map[string]*schema.ProductTypeSchema),
	}
}

// GetProductTypeSchema 获取产品类型Schema
func (sm *SchemaManager) GetProductTypeSchema(ctx context.Context, productType string) (*schema.ProductTypeSchema, error) {
	// 检查缓存
	if cached, ok := sm.schemaCache[productType]; ok {
		return cached, nil
	}

	// 从API获取产品类型定义
	definition, err := sm.apiClient.GetProductTypeDefinition(ctx, productType)
	if err != nil {
		return nil, fmt.Errorf("获取产品类型定义失败: %w", err)
	}

	// 检查Schema是否存在
	if definition.Schema == nil {
		return nil, fmt.Errorf("产品类型 %s 没有Schema定义", productType)
	}

	// 获取Schema URL（如果有的话）
	var schemaURL string
	if definition.Schema.Link != nil {
		if linkMap, ok := definition.Schema.Link.(map[string]any); ok {
			if resource, ok := linkMap["resource"].(string); ok {
				schemaURL = resource
			}
		}
	}

	if schemaURL == "" {
		return nil, fmt.Errorf("产品类型 %s 没有Schema链接", productType)
	}

	// 下载并解析Schema
	productSchema, err := sm.schemaFetcher.FetchSchema(ctx, schemaURL)
	if err != nil {
		return nil, fmt.Errorf("下载Schema失败: %w", err)
	}

	// 缓存
	sm.schemaCache[productType] = productSchema

	return productSchema, nil
}

// GetRequiredAttributes 获取必需属性
func (sm *SchemaManager) GetRequiredAttributes(productSchema *schema.ProductTypeSchema) []schema.AttributeInfo {
	return sm.schemaParser.GetRequiredAttributes(productSchema)
}
