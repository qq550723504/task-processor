// Package service 提供Amazon产品类型Schema解析功能
package service

import (
	"fmt"
	"strings"
	"task-processor/internal/platforms/amazon/internal/model"

	"github.com/sirupsen/logrus"
)

// SchemaParser Schema解析器
type SchemaParser struct {
	logger *logrus.Entry
}

// NewSchemaParser 创建解析器
func NewSchemaParser() *SchemaParser {
	return &SchemaParser{
		logger: logrus.WithField("service", "SchemaParser"),
	}
}

// ParseAttributes 解析所有属性信息
func (p *SchemaParser) ParseAttributes(schema *model.ProductTypeSchema) []model.AttributeInfo {
	p.logger.WithField("properties_count", len(schema.Properties)).Info("开始解析Schema属性")

	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	attrs := make([]model.AttributeInfo, 0, len(schema.Properties))
	for name, prop := range schema.Properties {
		attr := p.parseProperty(name, prop, requiredSet[name])
		attrs = append(attrs, attr)
	}

	p.logger.WithField("parsed_count", len(attrs)).Info("Schema属性解析完成")
	return attrs
}

// parseProperty 解析单个属性
func (p *SchemaParser) parseProperty(name string, prop model.PropertyDef, required bool) model.AttributeInfo {
	attr := model.AttributeInfo{
		Name:        name,
		Required:    required,
		Description: prop.Description,
		Type:        prop.Type,
		EnumValues:  prop.Enum,
		Examples:    prop.Examples,
		Format:      p.determineFormat(prop),
	}

	// 解析子属性
	if prop.Items != nil && len(prop.Items.Properties) > 0 {
		attr.SubAttrs = p.parseSubProperties(prop.Items)
	}

	return attr
}

// parseSubProperties 解析子属性
func (p *SchemaParser) parseSubProperties(items *model.ItemsDef) []model.SubAttributeInfo {
	requiredSet := make(map[string]bool)
	for _, r := range items.Required {
		requiredSet[r] = true
	}

	subAttrs := make([]model.SubAttributeInfo, 0, len(items.Properties))
	for name, propAny := range items.Properties {
		if propMap, ok := propAny.(map[string]any); ok {
			subAttr := p.parseSubProperty(name, propMap, requiredSet[name])
			subAttrs = append(subAttrs, subAttr)
		}
	}

	return subAttrs
}

// parseSubProperty 解析单个子属性
func (p *SchemaParser) parseSubProperty(name string, propMap map[string]any, required bool) model.SubAttributeInfo {
	subAttr := model.SubAttributeInfo{
		Name:     name,
		Required: required,
	}

	// 解析类型
	if typeVal, ok := propMap["type"].(string); ok {
		subAttr.Type = typeVal
	}

	// 解析枚举值
	if enumVal, ok := propMap["enum"].([]any); ok {
		subAttr.EnumValues = p.convertToStringSlice(enumVal)
	}

	// 检查是否是引用
	if refVal, ok := propMap["$ref"].(string); ok {
		subAttr.IsRef = true
		subAttr.RefName = p.extractRefName(refVal)
	}

	return subAttr
}

// determineFormat 确定属性格式
func (p *SchemaParser) determineFormat(prop model.PropertyDef) model.AttributeFormat {
	// 检查是否有items且items有properties
	if prop.Items != nil && len(prop.Items.Properties) > 0 {
		// 检查是否有language_tag字段
		if p.hasLanguageTag(prop.Items.Properties) {
			return model.FormatWithLang
		}

		// 检查是否有嵌套结构
		if p.hasNestedStructure(prop.Items.Properties) {
			return model.FormatNested
		}

		// 检查是否是复杂结构
		if len(prop.Items.Properties) > 3 {
			return model.FormatComplex
		}
	}

	return model.FormatSimple
}

// hasLanguageTag 检查是否有language_tag字段
func (p *SchemaParser) hasLanguageTag(properties map[string]any) bool {
	_, hasLangTag := properties["language_tag"]
	return hasLangTag
}

// hasNestedStructure 检查是否有嵌套结构
func (p *SchemaParser) hasNestedStructure(properties map[string]any) bool {
	for _, propAny := range properties {
		if propMap, ok := propAny.(map[string]any); ok {
			if typeVal, ok := propMap["type"].(string); ok && typeVal == "array" {
				return true
			}
			if _, hasItems := propMap["items"]; hasItems {
				return true
			}
		}
	}
	return false
}

// convertToStringSlice 转换为字符串切片
func (p *SchemaParser) convertToStringSlice(slice []any) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if str, ok := item.(string); ok {
			result = append(result, str)
		}
	}
	return result
}

// extractRefName 从引用路径中提取名称
func (p *SchemaParser) extractRefName(ref string) string {
	// 例如: "#/$defs/LanguageTag" -> "LanguageTag"
	parts := strings.Split(ref, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ref
}

// GetRequiredAttributes 获取必需属性列表
func (p *SchemaParser) GetRequiredAttributes(attrs []model.AttributeInfo) []model.AttributeInfo {
	var required []model.AttributeInfo
	for _, attr := range attrs {
		if attr.Required {
			required = append(required, attr)
		}
	}
	return required
}

// GetOptionalAttributes 获取可选属性列表
func (p *SchemaParser) GetOptionalAttributes(attrs []model.AttributeInfo) []model.AttributeInfo {
	var optional []model.AttributeInfo
	for _, attr := range attrs {
		if !attr.Required {
			optional = append(optional, attr)
		}
	}
	return optional
}

// GetAttributesByFormat 根据格式获取属性列表
func (p *SchemaParser) GetAttributesByFormat(attrs []model.AttributeInfo, format model.AttributeFormat) []model.AttributeInfo {
	var filtered []model.AttributeInfo
	for _, attr := range attrs {
		if attr.Format == format {
			filtered = append(filtered, attr)
		}
	}
	return filtered
}

// ValidateAttributeValue 验证属性值
func (p *SchemaParser) ValidateAttributeValue(attr model.AttributeInfo, value any) error {
	// 检查必需属性
	if attr.Required && (value == nil || value == "") {
		return fmt.Errorf("属性 %s 是必需的", attr.Name)
	}

	// 检查枚举值
	if len(attr.EnumValues) > 0 {
		if strVal, ok := value.(string); ok {
			for _, enumVal := range attr.EnumValues {
				if enumVal == strVal {
					return nil
				}
			}
			return fmt.Errorf("属性 %s 的值 %s 不在允许的枚举值中: %v", attr.Name, strVal, attr.EnumValues)
		}
	}

	return nil
}
