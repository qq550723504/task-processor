// Package sale 提供SHEIN平台销售属性的产品数据准备功能
package sale

import (
	"strconv"
	"strings"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/validation"

	"github.com/sirupsen/logrus"
)

// SaleAttributeProductDataPreparer 销售属性产品数据准备器
type SaleAttributeProductDataPreparer struct{}

// NewSaleAttributeProductDataPreparer 创建产品数据准备器实例
func NewSaleAttributeProductDataPreparer() *SaleAttributeProductDataPreparer {
	return &SaleAttributeProductDataPreparer{}
}

// PrepareProductsData 准备产品数据
func (p *SaleAttributeProductDataPreparer) PrepareProductsData(ctx *shein.TaskContext) []map[string]string {
	var productsData []map[string]string

	// 检查是否有变体数据
	hasVariants := ctx.Variants != nil && len(*ctx.Variants) > 0

	if !hasVariants {
		// 单体产品：使用主产品信息
		logrus.Infof("📦 检测到单体产品，使用主产品信息")
		if ctx.AmazonProduct != nil {
			productDetails := p.prepareSingleProductData(ctx)
			productsData = append(productsData, productDetails)
		}
	} else {
		// 多变体产品：使用变体信息
		logrus.Infof("📊 检测到多变体产品，变体数量: %d", len(*ctx.Variants))
		productsData = p.prepareMultiVariantProductsData(ctx)
	}

	logrus.Infof("✅ 准备了 %d 个产品数据（包含属性信息）", len(productsData))
	return productsData
}

// prepareSingleProductData 准备单体产品数据
func (p *SaleAttributeProductDataPreparer) prepareSingleProductData(ctx *shein.TaskContext) map[string]string {
	priceType := ctx.StoreInfo.PriceType

	productDetails := map[string]string{
		"asin":     ctx.AmazonProduct.Asin,
		"title":    ctx.AmazonProduct.Title,
		"price":    strconv.FormatFloat(validation.GetProductPrice(ctx.AmazonProduct, priceType), 'f', -1, 64),
		"currency": ctx.AmazonProduct.Currency,
	}

	// 提取属性、尺寸和重量
	p.enrichProductDetails(ctx.AmazonProduct, productDetails, "单体产品")

	return productDetails
}

// prepareMultiVariantProductsData 准备多变体产品数据
func (p *SaleAttributeProductDataPreparer) prepareMultiVariantProductsData(ctx *shein.TaskContext) []map[string]string {
	var productsData []map[string]string
	priceType := ctx.StoreInfo.PriceType

	// 转换为所需格式
	for _, variant := range *ctx.Variants {
		productDetails := map[string]string{
			"asin":     variant.Asin,
			"title":    variant.Title,
			"price":    strconv.FormatFloat(validation.GetProductPrice(ctx.AmazonProduct, priceType), 'f', -1, 64),
			"currency": variant.Currency,
		}

		// 提取属性、尺寸和重量
		p.enrichProductDetails(&variant, productDetails, "变体")

		productsData = append(productsData, productDetails)
	}

	return productsData
}

// enrichProductDetails 丰富产品详情（属性、尺寸、重量）
func (p *SaleAttributeProductDataPreparer) enrichProductDetails(product *model.Product, productDetails map[string]string, productType string) {
	// 提取属性信息
	p.extractAllAttributes(product, productDetails)

	// 设置尺寸和重量
	p.setDimensionsAndWeight(product, productDetails)

	// 统计并记录属性数量
	p.logAttributeCount(productDetails, product.Asin, productType)
}

// extractAllAttributes 提取所有属性信息
func (p *SaleAttributeProductDataPreparer) extractAllAttributes(product *model.Product, productDetails map[string]string) {
	// 从 Variations 中提取属性（优先级最高）
	foundInVariations := p.extractFromVariations(product, productDetails)

	// 如果从 Variations 中找到了属性，就不再从其他地方提取
	if foundInVariations {
		logrus.Infof("✅ 从 Variations 中找到属性，跳过其他来源")
	} else {
		// 从 ProductDetails 中提取其他属性
		p.extractFromProductDetails(product, productDetails)
	}

}

// extractFromVariations 从 Variations 中提取属性
func (p *SaleAttributeProductDataPreparer) extractFromVariations(product *model.Product, productDetails map[string]string) bool {
	if len(product.Variations) == 0 {
		return false
	}

	// 查找匹配的变体或使用第一个变体
	var targetVariation *model.Variation
	for _, variation := range product.Variations {
		if variation.Asin == product.Asin {
			targetVariation = &variation
			break
		}
	}

	// 提取属性
	attributeCount := 0
	if targetVariation != nil && targetVariation.Attributes != nil {
		for attrName, attrValue := range targetVariation.Attributes {
			if attrValue != nil {
				if strValue, ok := attrValue.(string); ok && strValue != "" {
					productDetails[strings.ToLower(attrName)] = strValue
					attributeCount++
				}
			}
		}
	}

	return attributeCount > 0
}

// extractFromProductDetails 从 ProductDetails 中提取属性
func (p *SaleAttributeProductDataPreparer) extractFromProductDetails(product *model.Product, productDetails map[string]string) {
	for _, detail := range product.ProductDetails {
		switch detail.Type {
		case "Material", "Color", "Size", "Style", "Pattern":
			if detail.Value != "" {
				attrName := strings.ToLower(strings.ReplaceAll(detail.Type, " ", "_"))
				// 如果还没有这个属性，则添加
				if _, exists := productDetails[attrName]; !exists {
					productDetails[attrName] = detail.Value
				}
			}
		}
	}
}

// setDimensionsAndWeight 设置产品尺寸和重量
func (p *SaleAttributeProductDataPreparer) setDimensionsAndWeight(product *model.Product, productDetails map[string]string) {
	// 设置产品尺寸
	productDetails["productdimensions"] = p.getProductDimensions(product)

	// 设置产品重量
	productDetails["weight"] = p.getItemWeight(product)
}

// getProductDimensions 获取产品尺寸
func (p *SaleAttributeProductDataPreparer) getProductDimensions(product *model.Product) string {
	if product.ProductDimensions != "" {
		return product.ProductDimensions
	}

	// 从ProductDetails中查找
	for _, detail := range product.ProductDetails {
		if detail.Type == "Product Dimensions" {
			logrus.Debugf("✅ 从ProductDetails中获取产品尺寸: %s", detail.Value)
			return detail.Value
		}
	}

	return ""
}

// getItemWeight 获取产品重量
func (p *SaleAttributeProductDataPreparer) getItemWeight(product *model.Product) string {
	if product.ItemWeight != "" {
		return product.ItemWeight
	}

	// 从ProductDetails中查找
	for _, detail := range product.ProductDetails {
		if detail.Type == "Item Weight" {
			logrus.Debugf("✅ 从ProductDetails中获取产品重量: %s", detail.Value)
			return detail.Value
		}
	}

	return ""
}

// logAttributeCount 记录属性数量
func (p *SaleAttributeProductDataPreparer) logAttributeCount(productDetails map[string]string, asin, productType string) {
	// 排除基本字段，统计属性数量
	excludeFields := map[string]bool{
		"asin": true, "title": true, "price": true,
		"currency": true, "productdimensions": true, "weight": true,
	}

	attributeCount := 0
	for key := range productDetails {
		if !excludeFields[key] {
			attributeCount++
		}
	}

	logrus.Debugf("✅ 为%s %s 提取了 %d 个属性信息", productType, asin, attributeCount)
}

