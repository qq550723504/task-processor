// Package context 提供TEMU平台属性映射相关的类型定义
package context

import (
	"task-processor/internal/model"
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

func ConvertAmazonProductData(amazonProduct *model.Product) AmazonProductData {
	if amazonProduct == nil {
		return AmazonProductData{}
	}

	productDetails := make([]ProductDetailData, 0, len(amazonProduct.ProductDetails))
	for _, detail := range amazonProduct.ProductDetails {
		productDetails = append(productDetails, ProductDetailData{
			Type:  detail.Type,
			Value: detail.Value,
		})
	}

	return AmazonProductData{
		Title:             amazonProduct.Title,
		Brand:             amazonProduct.Brand,
		Description:       amazonProduct.Description,
		Features:          amazonProduct.Features,
		ProductDetails:    productDetails,
		ProductDimensions: amazonProduct.ProductDimensions,
		ItemWeight:        amazonProduct.ItemWeight,
		ModelNumber:       amazonProduct.ModelNumber,
		Department:        amazonProduct.Department,
		Manufacturer:      amazonProduct.Manufacturer,
		Categories:        amazonProduct.Categories,
	}
}
