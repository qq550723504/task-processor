// Package context 提供TEMU平台属性映射相关的类型定义
package context

import (
	temutemplate "task-processor/internal/temu/api/template"
)

// PropertyMappingData AI属性映射数据结构
type PropertyMappingData struct {
	AmazonProduct  AmazonProductData                        `json:"amazon_product"`
	TemuProperties []temutemplate.TemplateRespGoodsProperty `json:"temu_properties"`
}

// AmazonProductData Amazon产品数据（简化版）
type AmazonProductData struct {
	Title             string              `json:"title"`
	Brand             string              `json:"brand"`
	Description       string              `json:"description"`
	Features          []string            `json:"features"`
	ProductDetails    []ProductDetailData `json:"product_details"`
	ProductDimensions string              `json:"product_dimensions"`
	ItemWeight        string              `json:"item_weight"`
	ModelNumber       string              `json:"model_number"`
	Department        string              `json:"department"`
	Manufacturer      string              `json:"manufacturer"`
	Categories        []string            `json:"categories"`
}

// ProductDetailData 产品详情数据
type ProductDetailData struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
