// Package context 提供TEMU平台AI相关的类型定义
package context

import (
	temutemplate "task-processor/internal/temu/api/template"
)

// SpecInfo 规格信息（用于AI生成的SKU映射）
type SpecInfo struct {
	SpecID         string `json:"spec_id"`
	SpecName       string `json:"spec_name"`
	ParentSpecID   string `json:"parent_spec_id"`
	ParentSpecName string `json:"parent_spec_name,omitempty"`
	ParentID       string `json:"parent_id,omitempty"`
}

// VariantMappingRequest AI变体映射请求
type VariantMappingRequest struct {
	ProductTitle       string                                       `json:"product_title"`
	Variants           []AmazonVariantForAI                         `json:"variants"`
	TemuSpecProperties []temutemplate.TemplateRespGoodsSpecProperty `json:"temu_spec_properties"`
}

// AmazonVariantForAI AI处理用的Amazon变体数据
type AmazonVariantForAI struct {
	Name              string            `json:"name"`
	Asin              string            `json:"asin"`
	Price             float64           `json:"price"`
	Image             string            `json:"image"`
	Attributes        map[string]any    `json:"attributes"`
	ProductDimensions string            `json:"product_dimensions,omitempty"`
	ItemWeight        string            `json:"item_weight,omitempty"`
	Description       string            `json:"description,omitempty"`
	Features          []string          `json:"features,omitempty"`
	ProductDetails    map[string]string `json:"product_details,omitempty"`
}

// AISkuMappingResponse AI SKU映射响应
type AISkuMappingResponse struct {
	SkuList []AIGeneratedSku `json:"sku_list"`
}

// AIGeneratedSku AI生成的SKU结构
type AIGeneratedSku struct {
	UniqueID           string            `json:"unique_id"`
	Asin               string            `json:"asin"`
	Spec               []SpecInfo        `json:"spec"`
	ColorSpecID        string            `json:"color_spec_id"`
	SpecID             string            `json:"spec_id"`
	VariantAttributes  map[string]string `json:"variant_attributes"`
	Weight             string            `json:"weight"`
	Length             string            `json:"length"`
	Width              string            `json:"width"`
	Height             string            `json:"height"`
	SkuClassification  int               `json:"sku_classification"`
	NumberOfPieces     int               `json:"number_of_pieces"`
	PieceUnitCode      int               `json:"piece_unit_code"`
	NetContentNumber   string            `json:"net_content_number"`
	NetContentUnitCode int               `json:"net_content_unit_code"`
	IndividuallyPacked int               `json:"individually_packed"`
}
