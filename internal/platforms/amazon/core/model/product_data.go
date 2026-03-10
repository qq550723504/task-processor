// Package model 提供Amazon产品数据结构定义
package model

import "fmt"

// ProductData 产品数据
type ProductData struct {
	SKU          string
	Title        string
	Brand        string
	Price        float64
	Currency     string
	Condition    string
	Quantity     int
	Description  string
	BulletPoints []string
	ProductType  string // 产品类型
	// 外部产品标识符配置
	IdentifierConfig *ProductIdentifierConfig
	// 价格相关
	PriceExcludingTax float64 // 不含税价格
	TaxRate           float64 // 税率（百分比）
	// 图片相关
	MainImageURL     string   // 主图URL
	AdditionalImages []string // 附加图片URLs
	// SEO优化相关
	SearchTerms    []string // 搜索关键词
	TargetAudience string   // 目标受众
	// 产品详细信息
	Manufacturer string            // 制造商
	ModelNumber  string            // 型号
	Dimensions   map[string]string // 尺寸信息
	Weight       string            // 重量
	Materials    []string          // 材质
	// 变体信息（用于创建父子产品关系）
	VariationData *VariationData // 变体数据
	// 合规性信息
	SafetyWarnings  []string // 安全警告
	AgeRestriction  string   // 年龄限制
	CountryOfOrigin string   // 原产国
	// 动态属性
	Attributes map[string]any
}

// ProductIdentifierConfig 产品标识符配置
type ProductIdentifierConfig struct {
	UPC         string `json:"upc,omitempty"`          // UPC码
	EAN         string `json:"ean,omitempty"`          // EAN码
	GTIN        string `json:"gtin,omitempty"`         // GTIN码
	ISBN        string `json:"isbn,omitempty"`         // ISBN码（书籍）
	PartNumber  string `json:"part_number,omitempty"`  // 零件号（汽配）
	ModelNumber string `json:"model_number,omitempty"` // 型号
}

// VariationData 变体数据
type VariationData struct {
	ParentSKU       string            `json:"parent_sku"`       // 父产品SKU
	IsParent        bool              `json:"is_parent"`        // 是否为父产品
	VariationTheme  string            `json:"variation_theme"`  // 变体主题（如 "ColorSize", "Color", "Size"）
	VariationValues map[string]string `json:"variation_values"` // 当前产品的变体值（如 {"color": "Red", "size": "Large"}）
	AllVariations   []VariationChild  `json:"all_variations"`   // 所有子变体（仅父产品使用）
}

// VariationChild 子变体信息
type VariationChild struct {
	SKU             string            `json:"sku"`              // 子产品SKU
	Price           float64           `json:"price"`            // 子产品价格
	Quantity        int               `json:"quantity"`         // 子产品库存
	MainImageURL    string            `json:"main_image_url"`   // 子产品主图
	VariationValues map[string]string `json:"variation_values"` // 子产品变体值
}

// Validate 验证产品数据
func (pd *ProductData) Validate() error {
	if pd.SKU == "" {
		return NewValidationError("sku", pd.SKU, "SKU不能为空")
	}

	if pd.Title == "" {
		return NewValidationError("title", pd.Title, "产品标题不能为空")
	}

	if pd.Brand == "" {
		return NewValidationError("brand", pd.Brand, "品牌不能为空")
	}

	if pd.Price <= 0 {
		return NewValidationError("price", fmt.Sprintf("%.2f", pd.Price), "价格必须大于0")
	}

	if pd.Currency == "" {
		return NewValidationError("currency", pd.Currency, "货币不能为空")
	}

	if pd.Quantity < 0 {
		return NewValidationError("quantity", fmt.Sprintf("%d", pd.Quantity), "库存不能为负数")
	}

	if pd.ProductType == "" {
		return NewValidationError("product_type", pd.ProductType, "产品类型不能为空")
	}

	return nil
}

// HasVariation 检查是否有变体信息
func (pd *ProductData) HasVariation() bool {
	return pd.VariationData != nil && pd.VariationData.VariationTheme != ""
}

// GetMainImage 获取主图URL
func (pd *ProductData) GetMainImage() string {
	return pd.MainImageURL
}

// GetAllImages 获取所有图片URL
func (pd *ProductData) GetAllImages() []string {
	images := make([]string, 0, len(pd.AdditionalImages)+1)

	if pd.MainImageURL != "" {
		images = append(images, pd.MainImageURL)
	}

	images = append(images, pd.AdditionalImages...)
	return images
}

// SetAttribute 设置自定义属性
func (pd *ProductData) SetAttribute(name string, value any) {
	if pd.Attributes == nil {
		pd.Attributes = make(map[string]any)
	}
	pd.Attributes[name] = value
}

// GetAttribute 获取自定义属性
func (pd *ProductData) GetAttribute(name string) (any, bool) {
	if pd.Attributes == nil {
		return nil, false
	}
	value, exists := pd.Attributes[name]
	return value, exists
}

// IsAutomotiveCategory 判断是否为汽配类目
func (pd *ProductData) IsAutomotiveCategory() bool {
	automotiveTypes := map[string]bool{
		"AUTO_ACCESSORY": true,
		"AUTO_PART":      true,
		"AUTOMOTIVE":     true,
	}
	return automotiveTypes[pd.ProductType]
}

// HasAnyIdentifier 检查是否有任何标识符
func (c *ProductIdentifierConfig) HasAnyIdentifier() bool {
	if c == nil {
		return false
	}
	return c.UPC != "" || c.EAN != "" || c.GTIN != "" || c.ISBN != "" || c.PartNumber != "" || c.ModelNumber != ""
}

// GetPrimaryIdentifier 获取主要标识符
func (c *ProductIdentifierConfig) GetPrimaryIdentifier() (string, string) {
	if c == nil {
		return "", ""
	}

	if c.UPC != "" {
		return "UPC", c.UPC
	}
	if c.EAN != "" {
		return "EAN", c.EAN
	}
	if c.GTIN != "" {
		return "GTIN", c.GTIN
	}
	if c.ISBN != "" {
		return "ISBN", c.ISBN
	}
	if c.PartNumber != "" {
		return "PART_NUMBER", c.PartNumber
	}
	if c.ModelNumber != "" {
		return "MODEL_NUMBER", c.ModelNumber
	}

	return "", ""
}
