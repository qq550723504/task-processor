// Package variations 提供 Amazon 产品变体信息提取功能
package variations

// VariationsData 保存变体数据和ASIN映射信息
type VariationsData struct {
	VariationsValues map[string][]string          `json:"variations_values"`
	ASINMapping      map[string]map[string]string `json:"asin_mapping"`
	PriceMapping     map[string]any       `json:"price_mapping"`
}

// VariationValue 变体值
type VariationValue struct {
	VariantName string   `json:"variant_name"`
	Values      []string `json:"values"`
}

// Variation 变体信息
type Variation struct {
	Name string `json:"name"`
	Asin string `json:"asin"`
	//Price      float64                `json:"price"`
	Currency   string                 `json:"currency"`
	Attributes map[string]any `json:"attributes"`
}

// ProductDetail 产品详情
type ProductDetail struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Product 产品接口（避免循环依赖）
type Product interface {
	GetVariationsValues() []VariationValue
	SetVariationsValues([]VariationValue)
	GetVariations() []Variation
	SetVariations([]Variation)
	GetProductDetails() []ProductDetail
	GetFinalPrice() float64
}
