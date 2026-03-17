// Package sku 提供TEMU平台的AI SKU映射工具辅助功能
package sku

import (
	"strings"

	"task-processor/internal/model"
	"task-processor/internal/pkg/jsonx"
	temucontext "task-processor/internal/temu/context"
)

// getProductDimensions 获取产品尺寸（优先使用直接字段，如果为空则从ProductDetails中提取）
func (vp *SkuVariantProcessor) getProductDimensions(product *model.Product) string {
	return vp.getProductDetailField(product, product.ProductDimensions, "dimensions")
}

// getItemWeight 获取产品重量（优先使用直接字段，如果为空则从ProductDetails中提取）
func (vp *SkuVariantProcessor) getItemWeight(product *model.Product) string {
	return vp.getProductDetailField(product, product.ItemWeight, "weight")
}

// getProductDetailField 通用字段提取：优先直接字段，否则从ProductDetails中按关键词查找
func (vp *SkuVariantProcessor) getProductDetailField(product *model.Product, directValue, keyword string) string {
	if directValue != "" {
		return directValue
	}
	for _, detail := range product.ProductDetails {
		if strings.Contains(strings.ToLower(detail.Type), keyword) && detail.Value != "" {
			return detail.Value
		}
	}
	return ""
}

// marshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
func (vp *SkuVariantProcessor) marshalWithoutHTMLEscape(v any) ([]byte, error) {
	return jsonx.MarshalIndentWithoutHTMLEscape(v, "", "  ")
}

// buildAIVariant 构建AI变体数据
func (vp *SkuVariantProcessor) buildAIVariant(
	product *model.Product,
	attributes map[string]any,
	productDimensions string,
	itemWeight string,
	productDetailsMap map[string]string,
) temucontext.AmazonVariantForAI {
	aiVariant := temucontext.AmazonVariantForAI{
		Name:       product.Title,
		Asin:       product.Asin,
		Price:      product.FinalPrice,
		Image:      product.ImageURL,
		Attributes: attributes,
	}

	// 只在有值时设置可选字段
	if productDimensions != "" {
		aiVariant.ProductDimensions = productDimensions
	}
	if itemWeight != "" {
		aiVariant.ItemWeight = itemWeight
	}
	// 只有在productDimensions或itemWeight为空时才赋值这些字段
	if productDimensions == "" || itemWeight == "" {
		if product.Description != "" {
			aiVariant.Description = product.Description
		}
		if len(product.Features) > 0 {
			aiVariant.Features = product.Features
		}
		if len(productDetailsMap) > 0 {
			aiVariant.ProductDetails = productDetailsMap
		}
	}

	return aiVariant
}
