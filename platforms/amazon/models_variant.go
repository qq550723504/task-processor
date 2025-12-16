package amazon

// VariantProduct 变体产品
type VariantProduct struct {
	ParentSKU      string                 `json:"parent_sku"`
	VariationTheme string                 `json:"variation_theme"` // 如: "SizeColor", "Size", "Color"
	ParentData     map[string]interface{} `json:"parent_data"`     // 父产品数据
	Children       []VariantChild         `json:"children"`        // 子变体列表
}

// VariantChild 子变体
type VariantChild struct {
	SKU           string                 `json:"sku"`
	Attributes    map[string]interface{} `json:"attributes"`     // 变体属性
	VariationData map[string]string      `json:"variation_data"` // 变体值（如：color=Red, size=M）
	Price         float64                `json:"price"`
	Quantity      int                    `json:"quantity"`
	Images        []string               `json:"images"`
	MainImageURL  string                 `json:"main_image_url"`
}

// VariationTheme 变体主题定义
type VariationTheme struct {
	Name       string   `json:"name"`       // 如: "SizeColor"
	Attributes []string `json:"attributes"` // 如: ["size", "color"]
}
