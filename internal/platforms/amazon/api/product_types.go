// Package api 提供Amazon SP-API产品类型定义功能
package api

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
)

// ProductTypeDefinition 产品类型定义
type ProductTypeDefinition struct {
	Name                 string                 `json:"name"`
	DisplayName          string                 `json:"displayName"`
	MarketplaceID        string                 `json:"marketplaceId"`
	MarketplaceIDs       []string               `json:"marketplaceIds"`
	ProductType          string                 `json:"productType"`
	ProductTypeVersion   map[string]any `json:"productTypeVersion"`
	Locale               string                 `json:"locale"`
	Requirements         string                 `json:"requirements"`
	RequirementsEnforced string                 `json:"requirementsEnforced"`
	MetaSchema           *ProductTypeSchema     `json:"metaSchema,omitempty"`
	Schema               *ProductTypeSchema     `json:"schema,omitempty"`
	PropertyGroups       map[string]any `json:"propertyGroups,omitempty"`
}

// ProductTypeSchema 产品类型Schema
type ProductTypeSchema struct {
	Link       any            `json:"link,omitempty"` // 可能是string或object
	Properties map[string]any `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
	Type       string                 `json:"type,omitempty"`
	Checksum   string                 `json:"checksum,omitempty"`
}

// ProductTypeSearchResult 产品类型搜索结果
type ProductTypeSearchResult struct {
	ProductTypes []ProductTypeDefinition `json:"productTypes"`
}

// GetProductTypeDefinitions 获取产品类型定义列表
func (c *Client) GetProductTypeDefinitions(ctx context.Context, keywords []string) ([]ProductTypeDefinition, error) {
	c.logger.Info("获取产品类型定义列表")

	// 构建查询参数
	path := fmt.Sprintf("/definitions/2020-09-01/productTypes?marketplaceIds=%s", c.marketplaceID)

	if len(keywords) > 0 {
		path += "&keywords=" + keywords[0] // 只使用第一个关键词
	}

	// 发送请求
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("获取产品类型定义失败: %w", err)
	}

	// 检查速率限制
	if err := c.handleRateLimit(resp); err != nil {
		return nil, err
	}

	// 解析响应
	var result ProductTypeSearchResult
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	c.logger.WithFields(logrus.Fields{
		"count": len(result.ProductTypes),
	}).Info("产品类型定义获取成功")

	return result.ProductTypes, nil
}

// GetProductTypeDefinition 获取特定产品类型的详细定义
func (c *Client) GetProductTypeDefinition(ctx context.Context, productType string) (*ProductTypeDefinition, error) {
	c.logger.WithFields(logrus.Fields{
		"productType": productType,
	}).Info("获取产品类型详细定义")

	// 构建请求路径
	path := fmt.Sprintf("/definitions/2020-09-01/productTypes/%s?marketplaceIds=%s&requirements=LISTING&requirementsEnforced=ENFORCED&locale=DEFAULT",
		productType, c.marketplaceID)

	// 发送请求
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("获取产品类型定义失败: %w", err)
	}

	// 检查速率限制
	if err := c.handleRateLimit(resp); err != nil {
		return nil, err
	}

	// 解析响应
	var result ProductTypeDefinition
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	c.logger.WithFields(logrus.Fields{
		"productType": productType,
		"displayName": result.DisplayName,
	}).Info("产品类型定义获取成功")

	return &result, nil
}

// SearchProductTypes 搜索产品类型
func (c *Client) SearchProductTypes(ctx context.Context, keywords []string) ([]ProductTypeDefinition, error) {
	c.logger.WithFields(logrus.Fields{
		"keywords": keywords,
	}).Info("搜索产品类型")

	return c.GetProductTypeDefinitions(ctx, keywords)
}

// AnalyzeProductTypeSchema 分析产品类型schema并打印详细信息
func (c *Client) AnalyzeProductTypeSchema(ctx context.Context, productType string) error {
	definition, err := c.GetProductTypeDefinition(ctx, productType)
	if err != nil {
		return err
	}

	c.logger.Info("📋 ===== 产品类型Schema分析 =====")
	c.logger.Infof("🏷️  产品类型: %s", productType)
	c.logger.Infof("📝 显示名称: %s", definition.DisplayName)
	c.logger.Infof("🌍 市场ID: %s", definition.MarketplaceID)

	// 分析属性组信息
	if len(definition.PropertyGroups) > 0 {
		c.logger.Info("📂 属性组:")
		for groupName, groupDef := range definition.PropertyGroups {
			if groupMap, ok := groupDef.(map[string]any); ok {
				title := groupMap["title"]
				description := groupMap["description"]
				c.logger.Infof("  📁 %s: %s - %s", groupName, title, description)

				// 显示属性组中的属性
				if propertyNames, ok := groupMap["propertyNames"].([]any); ok {
					c.logger.Infof("    属性数量: %d", len(propertyNames))
					if groupName == "product_identity" || groupName == "product_details" {
						c.logger.Infof("    包含属性:")
						for _, prop := range propertyNames {
							if propStr, ok := prop.(string); ok {
								c.logger.Infof("      - %s", propStr)
							}
						}
					}
				}
			}
		}
	}

	// 检查schema链接
	if definition.Schema != nil {
		if schemaMap, ok := definition.Schema.Link.(map[string]any); ok {
			if resource, ok := schemaMap["resource"].(string); ok {
				c.logger.Infof("📋 Schema资源链接: %s", resource)

				// 尝试下载并解析schema
				schemaContent, err := c.downloadSchema(ctx, resource)
				if err != nil {
					c.logger.Warnf("⚠️  无法下载schema: %v", err)
				} else {
					c.analyzeSchemaContent(schemaContent)
				}
			}
		}
	}

	c.logger.Info("📋 ===== Schema分析结束 =====")
	return nil
}

// DownloadSchema 下载schema内容（公开方法）
func (c *Client) DownloadSchema(ctx context.Context, url string) (map[string]any, error) {
	return c.downloadSchema(ctx, url)
}

// downloadSchema 下载schema内容
func (c *Client) downloadSchema(ctx context.Context, url string) (map[string]any, error) {
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("下载schema失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("下载schema失败，状态码: %d", resp.StatusCode)
	}

	var schemaContent map[string]any
	if err := c.parseResponse(resp, &schemaContent); err != nil {
		return nil, fmt.Errorf("解析schema失败: %w", err)
	}

	return schemaContent, nil
}

// analyzeSchemaContent 分析schema内容
func (c *Client) analyzeSchemaContent(schema map[string]any) {
	c.logger.Info("📋 Schema内容分析:")

	// 获取必需属性
	var requiredAttrs []string
	if required, ok := schema["required"].([]any); ok {
		for _, attr := range required {
			if attrStr, ok := attr.(string); ok {
				requiredAttrs = append(requiredAttrs, attrStr)
			}
		}
	}

	c.logger.Infof("✅ 必需属性 (%d个):", len(requiredAttrs))
	for _, attr := range requiredAttrs {
		c.logger.Infof("  - %s", attr)
	}

	// 分析所有属性
	if properties, ok := schema["properties"].(map[string]any); ok {
		c.logger.Infof("📝 所有属性 (%d个):", len(properties))

		requiredMap := make(map[string]bool)
		for _, attr := range requiredAttrs {
			requiredMap[attr] = true
		}

		// 重点关注unit_count相关属性
		unitCountAttrs := []string{"unit_count", "unit_count_type", "number_of_items", "item_package_quantity"}

		c.logger.Info("🔍 重点关注的数量相关属性:")
		for _, attrName := range unitCountAttrs {
			if attrDef, exists := properties[attrName]; exists {
				isRequired := requiredMap[attrName]
				requiredMark := ""
				if isRequired {
					requiredMark = " [必需]"
				}

				c.logger.Infof("  ✅ %s%s", attrName, requiredMark)
				if attrDefMap, ok := attrDef.(map[string]any); ok {
					if desc, ok := attrDefMap["description"].(string); ok {
						c.logger.Infof("      描述: %s", desc)
					}

					// 详细分析unit_count字段的结构
					if attrName == "unit_count" {
						c.logger.Info("      🔍 unit_count字段详细结构:")
						c.analyzeUnitCountStructure(attrDefMap, "        ")
					}
				}
			} else {
				c.logger.Infof("  ❌ %s (不存在)", attrName)
			}
		}
	}
}

// analyzeUnitCountStructure 详细分析unit_count字段结构
func (c *Client) analyzeUnitCountStructure(unitCountDef map[string]any, indent string) {
	// 分析items结构
	if items, ok := unitCountDef["items"].(map[string]any); ok {
		c.logger.Infof("%sitems结构:", indent)

		// 分析properties
		if properties, ok := items["properties"].(map[string]any); ok {
			c.logger.Infof("%s  properties:", indent)
			for propName, propDef := range properties {
				c.logger.Infof("%s    %s:", indent, propName)
				if propDefMap, ok := propDef.(map[string]any); ok {
					// 显示类型
					if propType, ok := propDefMap["type"].(string); ok {
						c.logger.Infof("%s      type: %s", indent, propType)
					}

					// 显示描述
					if desc, ok := propDefMap["description"].(string); ok {
						c.logger.Infof("%s      description: %s", indent, desc)
					}

					// 显示枚举值
					if enum, ok := propDefMap["enum"].([]any); ok {
						c.logger.Infof("%s      enum: %v", indent, enum)
					}

					// 如果是type字段，进一步分析其properties
					if propName == "type" {
						if typeProps, ok := propDefMap["properties"].(map[string]any); ok {
							c.logger.Infof("%s      type字段的properties:", indent)
							for typePropName, typePropDef := range typeProps {
								c.logger.Infof("%s        %s:", indent, typePropName)
								if typePropDefMap, ok := typePropDef.(map[string]any); ok {
									if typeType, ok := typePropDefMap["type"].(string); ok {
										c.logger.Infof("%s          type: %s", indent, typeType)
									}
									if typeEnum, ok := typePropDefMap["enum"].([]any); ok {
										c.logger.Infof("%s          enum: %v", indent, typeEnum)
									}
								}
							}
						}

						// 检查type字段的required
						if typeRequired, ok := propDefMap["required"].([]any); ok {
							c.logger.Infof("%s      type字段required: %v", indent, typeRequired)
						}
					}

					// 显示最小值/最大值
					if minimum, ok := propDefMap["minimum"].(float64); ok {
						c.logger.Infof("%s      minimum: %.0f", indent, minimum)
					}
					if maximum, ok := propDefMap["maximum"].(float64); ok {
						c.logger.Infof("%s      maximum: %.0f", indent, maximum)
					}
				}
			}
		}

		// 分析required字段
		if required, ok := items["required"].([]any); ok {
			c.logger.Infof("%s  required: %v", indent, required)
		}

		// 分析additionalProperties
		if additionalProps, ok := items["additionalProperties"]; ok {
			c.logger.Infof("%s  additionalProperties: %v", indent, additionalProps)
		}
	}
}
