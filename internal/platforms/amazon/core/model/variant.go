// Package model 提供Amazon变体产品数据模型
package model

import (
	"fmt"
)

// VariantProduct 变体产品
type VariantProduct struct {
	ParentSKU      string         `json:"parent_sku"`
	VariationTheme string         `json:"variation_theme"` // 如: "SizeColor", "Size", "Color"
	ParentData     map[string]any `json:"parent_data"`     // 父产品数据
	Children       []VariantChild `json:"children"`        // 子变体列表
}

// VariantChild 子变体
type VariantChild struct {
	SKU           string            `json:"sku"`
	ASIN          string            `json:"asin,omitempty"`
	Attributes    map[string]any    `json:"attributes"`     // 变体属性
	VariationData map[string]string `json:"variation_data"` // 变体值（如：color=Red, size=M）
	Price         float64           `json:"price"`
	Quantity      int               `json:"quantity"`
	Images        []string          `json:"images"`
	MainImageURL  string            `json:"main_image_url"`
	Status        string            `json:"status"`
}

// VariationTheme 变体主题定义
type VariationTheme struct {
	Name       string   `json:"name"`       // 如: "SizeColor"
	Attributes []string `json:"attributes"` // 如: ["size", "color"]
}

// VariationAttribute 变体属性定义
type VariationAttribute struct {
	Name        string   `json:"name"`         // 属性名，如: "color", "size"
	DisplayName string   `json:"display_name"` // 显示名称，如: "颜色", "尺寸"
	Values      []string `json:"values"`       // 可选值列表
	Required    bool     `json:"required"`     // 是否必需
	DataType    string   `json:"data_type"`    // 数据类型: string, number, boolean
}

// 常用变体主题常量
const (
	VariationThemeSize      = "Size"
	VariationThemeColor     = "Color"
	VariationThemeSizeColor = "SizeColor"
	VariationThemeStyle     = "Style"
	VariationThemeFlavor    = "Flavor"
	VariationThemeScent     = "Scent"
)

// 常用变体属性常量
const (
	VariationAttrSize   = "size"
	VariationAttrColor  = "color"
	VariationAttrStyle  = "style"
	VariationAttrFlavor = "flavor"
	VariationAttrScent  = "scent"
)

// VariantCreateRequest 变体创建请求
type VariantCreateRequest struct {
	ParentSKU      string                `json:"parent_sku"`
	VariationTheme string                `json:"variation_theme"`
	ParentData     map[string]any        `json:"parent_data"`
	Children       []VariantChildRequest `json:"children"`
}

// VariantChildRequest 子变体创建请求
type VariantChildRequest struct {
	SKU           string            `json:"sku"`
	Attributes    map[string]any    `json:"attributes"`
	VariationData map[string]string `json:"variation_data"`
	Price         float64           `json:"price"`
	Quantity      int               `json:"quantity"`
	Images        []string          `json:"images"`
}

// VariantUpdateRequest 变体更新请求
type VariantUpdateRequest struct {
	ParentSKU string               `json:"parent_sku"`
	Children  []VariantChildUpdate `json:"children"`
}

// VariantChildUpdate 子变体更新
type VariantChildUpdate struct {
	SKU      string  `json:"sku"`
	Price    float64 `json:"price,omitempty"`
	Quantity int     `json:"quantity,omitempty"`
	Status   string  `json:"status,omitempty"`
}

// GetVariationThemes 获取支持的变体主题
func GetVariationThemes() []VariationTheme {
	return []VariationTheme{
		{
			Name:       VariationThemeSize,
			Attributes: []string{VariationAttrSize},
		},
		{
			Name:       VariationThemeColor,
			Attributes: []string{VariationAttrColor},
		},
		{
			Name:       VariationThemeSizeColor,
			Attributes: []string{VariationAttrSize, VariationAttrColor},
		},
		{
			Name:       VariationThemeStyle,
			Attributes: []string{VariationAttrStyle},
		},
		{
			Name:       VariationThemeFlavor,
			Attributes: []string{VariationAttrFlavor},
		},
		{
			Name:       VariationThemeScent,
			Attributes: []string{VariationAttrScent},
		},
	}
}

// IsValidVariationTheme 检查变体主题是否有效
func IsValidVariationTheme(theme string) bool {
	themes := GetVariationThemes()
	for _, t := range themes {
		if t.Name == theme {
			return true
		}
	}
	return false
}

// GetAttributesForTheme 获取变体主题对应的属性
func GetAttributesForTheme(theme string) []string {
	themes := GetVariationThemes()
	for _, t := range themes {
		if t.Name == theme {
			return t.Attributes
		}
	}
	return []string{}
}

// ValidateVariantChild 验证子变体数据
func (v *VariantChild) Validate() error {
	if v.SKU == "" {
		return fmt.Errorf("子变体SKU不能为空")
	}

	if v.Price <= 0 {
		return fmt.Errorf("子变体价格必须大于0")
	}

	if v.Quantity < 0 {
		return fmt.Errorf("子变体库存不能为负数")
	}

	if len(v.VariationData) == 0 {
		return fmt.Errorf("子变体必须包含变体数据")
	}

	return nil
}

// ValidateVariantProduct 验证变体产品数据
func (v *VariantProduct) Validate() error {
	if v.ParentSKU == "" {
		return fmt.Errorf("父产品SKU不能为空")
	}

	if v.VariationTheme == "" {
		return fmt.Errorf("变体主题不能为空")
	}

	if !IsValidVariationTheme(v.VariationTheme) {
		return fmt.Errorf("无效的变体主题: %s", v.VariationTheme)
	}

	if len(v.Children) == 0 {
		return fmt.Errorf("变体产品必须包含至少一个子变体")
	}

	// 验证每个子变体
	for i, child := range v.Children {
		if err := child.Validate(); err != nil {
			return fmt.Errorf("子变体 %d 验证失败: %w", i+1, err)
		}
	}

	// 验证变体属性一致性
	expectedAttrs := GetAttributesForTheme(v.VariationTheme)
	for i, child := range v.Children {
		for _, attr := range expectedAttrs {
			if _, exists := child.VariationData[attr]; !exists {
				return fmt.Errorf("子变体 %d 缺少必需的变体属性: %s", i+1, attr)
			}
		}
	}

	return nil
}
