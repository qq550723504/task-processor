// Package service 提供Amazon必需属性构建功能
package service

import (
	"task-processor/internal/platforms/amazon/internal/model"

	"github.com/sirupsen/logrus"
)

// RequiredAttributeBuilder 必需属性构建器
type RequiredAttributeBuilder struct {
	logger *logrus.Entry
}

// NewRequiredAttributeBuilder 创建必需属性构建器
func NewRequiredAttributeBuilder() *RequiredAttributeBuilder {
	return &RequiredAttributeBuilder{
		logger: logrus.WithField("component", "RequiredAttributeBuilder"),
	}
}

// AddRequiredAttributes 处理必需属性
func (rab *RequiredAttributeBuilder) AddRequiredAttributes(attrs map[string]any, builder *SchemaBuilder, requiredAttrs []model.AttributeInfo, data *model.ProductData, marketplaceID string) {
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
		defaultValue := rab.getDefaultValue(attr, marketplaceID)
		if defaultValue != nil {
			attrs[attr.Name] = defaultValue
		}
	}
}

// EnsureCommonAttributes 确保常见必需属性都有值（完全通用，不限制品类）
func (rab *RequiredAttributeBuilder) EnsureCommonAttributes(attrs map[string]any, marketplaceID string, data *model.ProductData, productSchema *model.ProductTypeSchema) {
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
func (rab *RequiredAttributeBuilder) getDefaultValue(attr model.AttributeInfo, marketplaceID string) any {
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
