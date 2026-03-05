// Package sku 提供TEMU平台的AI SKU映射工具辅助功能
package sku

import (
	"bytes"
	"encoding/json"
	"strings"

	"task-processor/internal/domain/model"
	"task-processor/internal/platforms/temu/types"
)

// getProductDimensions 获取产品尺寸（优先使用直接字段，如果为空则从ProductDetails中提取）
func (vp *SkuVariantProcessor) getProductDimensions(product *model.Product) string {
	// 优先使用直接字段
	if product.ProductDimensions != "" {
		return product.ProductDimensions
	}

	// 从ProductDetails中提取
	for _, detail := range product.ProductDetails {
		if strings.Contains(strings.ToLower(detail.Type), "dimensions") && detail.Value != "" {
			return detail.Value
		}
	}

	return ""
}

// getItemWeight 获取产品重量（优先使用直接字段，如果为空则从ProductDetails中提取）
func (vp *SkuVariantProcessor) getItemWeight(product *model.Product) string {
	// 优先使用直接字段
	if product.ItemWeight != "" {
		return product.ItemWeight
	}

	// 从ProductDetails中提取
	for _, detail := range product.ProductDetails {
		if strings.Contains(strings.ToLower(detail.Type), "weight") && detail.Value != "" {
			return detail.Value
		}
	}

	return ""
}

// marshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
func (vp *SkuVariantProcessor) marshalWithoutHTMLEscape(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(v)
	if err != nil {
		return nil, err
	}
	// 移除最后的换行符
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result, nil
}

// buildAIVariant 构建AI变体数据
func (vp *SkuVariantProcessor) buildAIVariant(
	product *model.Product,
	attributes map[string]any,
	productDimensions string,
	itemWeight string,
	productDetailsMap map[string]string,
) types.AmazonVariantForAI {
	aiVariant := types.AmazonVariantForAI{
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
