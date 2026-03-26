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

func NewEmptyAISkuMappingResponse() *AISkuMappingResponse {
	return &AISkuMappingResponse{}
}

func NewAISkuMappingResponse(skus []AIGeneratedSku) *AISkuMappingResponse {
	response := NewEmptyAISkuMappingResponse()
	response.ReplaceSKUs(skus)
	return response
}

func (r *AISkuMappingResponse) SkuCount() int {
	if r == nil {
		return 0
	}
	return len(r.SkuList)
}

func (r *AISkuMappingResponse) FirstSKU() (*AIGeneratedSku, bool) {
	if r == nil || len(r.SkuList) == 0 {
		return nil, false
	}
	return &r.SkuList[0], true
}

func (r *AISkuMappingResponse) SKUAt(index int) (*AIGeneratedSku, bool) {
	if r == nil || index < 0 || index >= len(r.SkuList) {
		return nil, false
	}
	return &r.SkuList[index], true
}

func (r *AISkuMappingResponse) ForEachSKU(fn func(*AIGeneratedSku)) {
	if r == nil || fn == nil {
		return
	}
	for i := range r.SkuList {
		fn(&r.SkuList[i])
	}
}

func (r *AISkuMappingResponse) ForEachSKUIndexed(fn func(int, *AIGeneratedSku)) {
	if r == nil || fn == nil {
		return
	}
	for i := range r.SkuList {
		fn(i, &r.SkuList[i])
	}
}

func (r *AISkuMappingResponse) ReplaceSKUs(skus []AIGeneratedSku) {
	if r == nil {
		return
	}
	r.SkuList = skus
}

func (r *AISkuMappingResponse) AppendSKU(sku AIGeneratedSku) {
	if r == nil {
		return
	}
	r.SkuList = append(r.SkuList, sku)
}

func (r *AISkuMappingResponse) AppendSKUs(skus []AIGeneratedSku) {
	if r == nil || len(skus) == 0 {
		return
	}
	r.SkuList = append(r.SkuList, skus...)
}

func (r *AISkuMappingResponse) AppendResponse(other *AISkuMappingResponse) {
	if r == nil || other == nil || other.SkuCount() == 0 {
		return
	}
	r.AppendSKUs(other.SkuList)
}

func (r *AISkuMappingResponse) FirstSpecDimensions() []string {
	firstSKU, ok := r.FirstSKU()
	if !ok {
		return []string{}
	}

	specDimensions := make(map[string]bool)
	ordered := make([]string, 0, len(firstSKU.Spec))
	for _, specInfo := range firstSKU.Spec {
		if specDimensions[specInfo.ParentSpecID] {
			continue
		}
		specDimensions[specInfo.ParentSpecID] = true
		ordered = append(ordered, specInfo.ParentSpecID)
	}

	return ordered
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
