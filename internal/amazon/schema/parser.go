// Package schema 提供Amazon产品类型Schema解析功能
package schema

import (
	"strings"
)

// Parser Schema解析器
type Parser struct{}

// NewParser 创建解析器
func NewParser() *Parser {
	return &Parser{}
}

// ParseAttributes 解析所有属性信息
func (p *Parser) ParseAttributes(schema *ProductTypeSchema) []AttributeInfo {
	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	attrs := make([]AttributeInfo, 0, len(schema.Properties))
	for name, prop := range schema.Properties {
		attr := p.parseProperty(name, prop, requiredSet[name])
		attrs = append(attrs, attr)
	}

	return attrs
}

// GetRequiredAttributes 获取必需属性
func (p *Parser) GetRequiredAttributes(schema *ProductTypeSchema) []AttributeInfo {
	all := p.ParseAttributes(schema)
	required := make([]AttributeInfo, 0)
	for _, attr := range all {
		if attr.Required {
			required = append(required, attr)
		}
	}
	return required
}

// parseProperty 解析单个属性
func (p *Parser) parseProperty(name string, prop PropertyDef, required bool) AttributeInfo {
	attr := AttributeInfo{
		Name:        name,
		Required:    required,
		Description: prop.Description,
		Examples:    prop.Examples,
	}

	// 解析枚举值
	if len(prop.Enum) > 0 {
		attr.EnumValues = prop.Enum
	}

	// 解析items中的子属性
	if prop.Items != nil {
		attr.SubAttrs = p.parseSubProperties(prop.Items)
		attr.Format = p.detectFormat(attr.SubAttrs)
	} else {
		attr.Format = FormatSimple
	}

	return attr
}

// parseSubProperties 解析子属性
func (p *Parser) parseSubProperties(items *ItemsDef) []SubAttributeInfo {
	subAttrs := make([]SubAttributeInfo, 0, len(items.Properties))

	requiredSet := make(map[string]bool)
	for _, r := range items.Required {
		requiredSet[r] = true
	}

	for name, prop := range items.Properties {
		sub := SubAttributeInfo{
			Name:     name,
			Required: requiredSet[name],
		}

		// 解析子属性详情
		if propMap, ok := prop.(map[string]any); ok {
			if ref, ok := propMap["$ref"].(string); ok {
				sub.IsRef = true
				sub.RefName = strings.TrimPrefix(ref, "#/$defs/")
			}
			if t, ok := propMap["type"].(string); ok {
				sub.Type = t
			}
			// 解析枚举
			if items, ok := propMap["items"].(map[string]any); ok {
				if props, ok := items["properties"].(map[string]any); ok {
					if valueProp, ok := props["value"].(map[string]any); ok {
						if anyOf, ok := valueProp["anyOf"].([]any); ok {
							for _, a := range anyOf {
								if aMap, ok := a.(map[string]any); ok {
									if enum, ok := aMap["enum"].([]any); ok {
										for _, e := range enum {
											if s, ok := e.(string); ok {
												sub.EnumValues = append(sub.EnumValues, s)
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}

		subAttrs = append(subAttrs, sub)
	}

	return subAttrs
}

// detectFormat 检测属性格式
func (p *Parser) detectFormat(subAttrs []SubAttributeInfo) AttributeFormat {
	hasValue := false
	hasLangTag := false
	hasNestedArray := false

	for _, sub := range subAttrs {
		switch sub.Name {
		case "value":
			hasValue = true
		case "language_tag":
			hasLangTag = true
		default:
			if sub.Type == "array" {
				hasNestedArray = true
			}
		}
	}

	if hasNestedArray {
		return FormatNested
	}
	if hasValue && hasLangTag {
		return FormatWithLang
	}
	if hasValue {
		return FormatSimple
	}
	return FormatComplex
}
