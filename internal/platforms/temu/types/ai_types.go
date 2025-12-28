// Package types 提供TEMU平台AI相关的类型定义
package types

// VariantMappingRequest AI变体映射请求
type VariantMappingRequest struct {
	ProductTitle       string                          `json:"product_title"`
	Variants           []AmazonVariantForAI            `json:"variants"`
	TemuSpecProperties []TemplateRespGoodsSpecProperty `json:"temu_spec_properties"`
}

// AmazonVariantForAI AI处理用的Amazon变体数据
type AmazonVariantForAI struct {
	Name              string            `json:"name"`
	Asin              string            `json:"asin"`
	Price             float64           `json:"price"`
	Image             string            `json:"image"`
	Attributes        map[string]any    `json:"attributes"`
	ProductDimensions string            `json:"product_dimensions,omitempty"` // 产品尺寸
	ItemWeight        string            `json:"item_weight,omitempty"`        // 产品重量
	Description       string            `json:"description,omitempty"`        // 产品描述
	Features          []string          `json:"features,omitempty"`           // 产品特性
	ProductDetails    map[string]string `json:"product_details,omitempty"`    // 产品详情
}

// AISkuMappingResponse AI SKU映射响应
type AISkuMappingResponse struct {
	SkuList []AIGeneratedSku `json:"sku_list"`
}

// AIGeneratedSku AI生成的SKU结构
type AIGeneratedSku struct {
	UniqueID          string            `json:"unique_id"`
	Asin              string            `json:"asin"`
	Spec              []SpecInfo        `json:"spec"`
	ColorSpecID       string            `json:"color_spec_id"`
	SpecID            string            `json:"spec_id"`
	VariantAttributes map[string]string `json:"variant_attributes"`
	// 物流信息
	Weight string `json:"weight"` // 重量，单位：克
	Length string `json:"length"` // 长度，单位：毫米
	Width  string `json:"width"`  // 宽度，单位：毫米
	Height string `json:"height"` // 高度，单位：毫米
	// 多件装信息
	SkuClassification  int    `json:"sku_classification"`    // SKU类型：1-单品，2-组合装，3-混合装
	NumberOfPieces     int    `json:"number_of_pieces"`      // 可单独售卖的产品数量
	PieceUnitCode      int    `json:"piece_unit_code"`       // 单位规格ID：1-件，2-双，3-包
	NetContentNumber   string `json:"net_content_number"`    // 净含量数值（用于计算单价）
	NetContentUnitCode int    `json:"net_content_unit_code"` // 净含量单位代码
	IndividuallyPacked int    `json:"individually_packed"`   // 是否独立包装：1-是，0-否
}
