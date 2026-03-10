// Package model 提供Amazon产品类型Schema数据结构定义
package model

// ProductTypeSchema 产品类型Schema
type ProductTypeSchema struct {
	Properties map[string]PropertyDef `json:"properties"`
	Required   []string               `json:"required"`
	Defs       map[string]any         `json:"$defs"`
}

// PropertyDef 属性定义
type PropertyDef struct {
	Description string         `json:"description"`
	Title       string         `json:"title"`
	Examples    []any          `json:"examples"`
	Items       *ItemsDef      `json:"items"`
	Type        string         `json:"type"`
	Enum        []string       `json:"enum"`
	EnumNames   []string       `json:"enumNames"`
	AnyOf       []any          `json:"anyOf"`
	Properties  map[string]any `json:"properties"`
}

// ItemsDef 数组项定义
type ItemsDef struct {
	Properties map[string]any `json:"properties"`
	Required   []string       `json:"required"`
}

// SubPropertyDef 子属性定义
type SubPropertyDef struct {
	Description string     `json:"description"`
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	Ref         string     `json:"$ref"`
	Items       *ItemsDef  `json:"items"`
	Enum        []string   `json:"enum"`
	EnumNames   []string   `json:"enumNames"`
	AnyOf       []AnyOfDef `json:"anyOf"`
	Examples    []any      `json:"examples"`
}

// AnyOfDef anyOf定义
type AnyOfDef struct {
	Type      string   `json:"type"`
	Enum      []string `json:"enum"`
	EnumNames []string `json:"enumNames"`
}

// AttributeInfo 属性信息（解析后）
type AttributeInfo struct {
	Name        string             // 属性名
	Required    bool               // 是否必需
	Description string             // 描述
	Type        string             // 类型
	Format      AttributeFormat    // 格式类型
	EnumValues  []string           // 枚举值
	SubAttrs    []SubAttributeInfo // 子属性
	Examples    []any              // 示例值
}

// SubAttributeInfo 子属性信息
type SubAttributeInfo struct {
	Name       string   // 子属性名
	Required   bool     // 是否必需
	Type       string   // 类型
	EnumValues []string // 枚举值
	IsRef      bool     // 是否引用
	RefName    string   // 引用名称
}

// AttributeFormat 属性格式类型
type AttributeFormat int

const (
	FormatSimple   AttributeFormat = iota // 简单格式: [{value, marketplace_id}]
	FormatWithLang                        // 带语言标签: [{value, language_tag, marketplace_id}]
	FormatNested                          // 嵌套格式: [{marketplace_id, sub_attr: [{value, ...}]}]
	FormatComplex                         // 复杂格式: 需要特殊处理
)

// String 返回格式类型的字符串表示
func (f AttributeFormat) String() string {
	switch f {
	case FormatSimple:
		return "simple"
	case FormatWithLang:
		return "with_lang"
	case FormatNested:
		return "nested"
	case FormatComplex:
		return "complex"
	default:
		return "unknown"
	}
}

// IsRequired 检查属性是否为必需
func (ai *AttributeInfo) IsRequired() bool {
	return ai.Required
}

// HasEnumValues 检查属性是否有枚举值
func (ai *AttributeInfo) HasEnumValues() bool {
	return len(ai.EnumValues) > 0
}

// HasSubAttributes 检查属性是否有子属性
func (ai *AttributeInfo) HasSubAttributes() bool {
	return len(ai.SubAttrs) > 0
}

// GetFirstEnumValue 获取第一个枚举值
func (ai *AttributeInfo) GetFirstEnumValue() string {
	if len(ai.EnumValues) > 0 {
		return ai.EnumValues[0]
	}
	return ""
}

// GetFirstExample 获取第一个示例值
func (ai *AttributeInfo) GetFirstExample() any {
	if len(ai.Examples) > 0 {
		return ai.Examples[0]
	}
	return nil
}
