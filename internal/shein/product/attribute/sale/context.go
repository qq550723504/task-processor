// Package sale 提供SHEIN平台销售属性的上下文构建功能
package sale

import (
	"fmt"
	"strings"
	"task-processor/internal/domain/model"
	shein "task-processor/internal/shein"

	"github.com/sirupsen/logrus"
)

// SaleAttributeContextBuilder 销售属性上下文构建器
type SaleAttributeContextBuilder struct{}

// NewSaleAttributeContextBuilder 创建上下文构建器实例
func NewSaleAttributeContextBuilder() *SaleAttributeContextBuilder {
	return &SaleAttributeContextBuilder{}
}

// BuildCompactProductContext 构建精简的产品上下文信息
func (c *SaleAttributeContextBuilder) BuildCompactProductContext(amazonProduct model.Product, variants []model.Product) string {
	var contextParts []string
	if amazonProduct.Title != "" {
		contextParts = append(contextParts, fmt.Sprintf("标题: %s", amazonProduct.Title))
	}
	if amazonProduct.Brand != "" {
		contextParts = append(contextParts, fmt.Sprintf("品牌: %s", amazonProduct.Brand))
	}
	if len(amazonProduct.Categories) > 0 {
		contextParts = append(contextParts, fmt.Sprintf("分类: %s", strings.Join(amazonProduct.Categories, " > ")))
	}
	if len(amazonProduct.Features) > 0 {
		featureCount := len(amazonProduct.Features)
		if featureCount > 3 {
			featureCount = 3
		}
		contextParts = append(contextParts, fmt.Sprintf("关键特征: %s",
			strings.Join(amazonProduct.Features[:featureCount], "; ")))
	} else if amazonProduct.Description != "" {
		desc := amazonProduct.Description
		if len(desc) > 200 {
			desc = desc[:200] + "..."
		}
		contextParts = append(contextParts, fmt.Sprintf("描述: %s", desc))
	}
	variantCount := 1
	if variants != nil {
		variantCount = len(variants)
	}
	contextParts = append(contextParts, fmt.Sprintf("变体数: %d", variantCount))
	return strings.Join(contextParts, "\n")
}

// BuildExtraContext 构建额外上下文信息（仅在检测到缺少信息时提供）
func (c *SaleAttributeContextBuilder) BuildExtraContext(amazonProduct model.Product, variants []model.Product, productsData []shein.ProductVariantData) string {
	logrus.Debug("🔍 开始检测是否需要额外上下文...")

	// 检测缺失信息
	missingInfo := c.detectMissingInfo(productsData)
	if !missingInfo.needsExtraContext {
		logrus.Info("✅ 产品信息完整，无需额外上下文")
		return ""
	}

	logrus.Infof("⚠️ 检测到缺失信息：缺少尺寸(%d个) 缺少重量(%d个) 缺少属性(%d个)，开始构建针对性额外上下文...",
		missingInfo.missingDimensions, missingInfo.missingWeight, missingInfo.missingAttributes)

	var extraParts []string

	// 根据缺失情况有针对性地添加信息
	if missingInfo.missingDimensions > 0 || missingInfo.missingWeight > 0 {
		c.addPhysicalInfoToContext(amazonProduct, variants, &extraParts, missingInfo)
	}

	if missingInfo.missingAttributes > 0 {
		c.addAttributeInfoToContext(amazonProduct, &extraParts, missingInfo)
	}

	if len(extraParts) == 0 {
		logrus.Warn("⚠️ 额外上下文为空：所有可用信息源都为空")
		return ""
	}

	logrus.Infof("✅ 构建了针对性额外上下文，包含 %d 个信息块", len(extraParts))
	return strings.Join(extraParts, "\n")
}

// MissingInfoDetection 缺失信息检测结果
type MissingInfoDetection struct {
	needsExtraContext      bool
	missingDimensions      int
	missingWeight          int
	missingAttributes      int
	missingDimensionsASINs []string // 缺少尺寸的ASIN列表
	missingWeightASINs     []string // 缺少重量的ASIN列表
	missingAttributesASINs []string // 缺少属性的ASIN列表
}

// detectMissingInfo 检测缺失的信息
func (c *SaleAttributeContextBuilder) detectMissingInfo(productsData []shein.ProductVariantData) MissingInfoDetection {
	var detection MissingInfoDetection

	for _, product := range productsData {
		if product.Dimensions == "" {
			detection.missingDimensions++
			detection.missingDimensionsASINs = append(detection.missingDimensionsASINs, product.ASIN)
		}
		if product.Weight == "" {
			detection.missingWeight++
			detection.missingWeightASINs = append(detection.missingWeightASINs, product.ASIN)
		}
		if len(product.Attributes) == 0 {
			detection.missingAttributes++
			detection.missingAttributesASINs = append(detection.missingAttributesASINs, product.ASIN)
		}
	}

	totalProducts := len(productsData)
	attributeThreshold := totalProducts / 2 // 超过50%缺少属性

	// 判断是否需要额外上下文
	detection.needsExtraContext = detection.missingDimensions > 0 ||
		detection.missingWeight > 0 ||
		detection.missingAttributes > attributeThreshold

	// 详细记录缺失信息
	if detection.missingDimensions > 0 {
		logrus.Warnf("⚠️ 缺少尺寸信息的ASIN: %v", detection.missingDimensionsASINs)
	}
	if detection.missingWeight > 0 {
		logrus.Warnf("⚠️ 缺少重量信息的ASIN: %v", detection.missingWeightASINs)
	}
	if detection.missingAttributes > 0 {
		logrus.Warnf("⚠️ 缺少属性信息的ASIN: %v", detection.missingAttributesASINs)
	}

	return detection
}

// addPhysicalInfoToContext 添加物理信息到上下文（尺寸、重量相关）
func (c *SaleAttributeContextBuilder) addPhysicalInfoToContext(amazonProduct model.Product, variants []model.Product, extraParts *[]string, missingInfo MissingInfoDetection) {
	// 添加产品描述（可能包含尺寸信息）
	if amazonProduct.Description != "" {
		*extraParts = append(*extraParts, fmt.Sprintf("\n【产品完整描述】（用于估算缺失尺寸重量，涉及ASIN: %v）\n%s",
			c.getMissingPhysicalASINs(missingInfo), amazonProduct.Description))
		logrus.Debug("✅ 添加了产品描述用于尺寸重量推断")
	}

	// 添加产品特征（可能包含物理规格）
	if len(amazonProduct.Features) > 0 {
		*extraParts = append(*extraParts, fmt.Sprintf("\n【完整产品特征】（可能包含尺寸、重量、材质等信息）\n%s",
			strings.Join(amazonProduct.Features, "\n")))
		logrus.Debugf("✅ 添加了 %d 个产品特征用于物理信息推断", len(amazonProduct.Features))
	}

	// 添加主产品的物理信息
	if amazonProduct.ProductDimensions != "" || amazonProduct.ItemWeight != "" {
		var physicalInfo []string
		if amazonProduct.ProductDimensions != "" {
			physicalInfo = append(physicalInfo, fmt.Sprintf("包装尺寸: %s", amazonProduct.ProductDimensions))
		}
		if amazonProduct.ItemWeight != "" {
			physicalInfo = append(physicalInfo, fmt.Sprintf("产品重量: %s", amazonProduct.ItemWeight))
		}
		*extraParts = append(*extraParts, fmt.Sprintf("\n【主产品物理信息】（作为参考或估算基准）\n%s",
			strings.Join(physicalInfo, "\n")))
	}

	// 添加ProductDetails中的物理信息
	c.addProductDetailsToContext(amazonProduct, extraParts)
	c.addVariantDetailsToContext(variants, extraParts)
}

// addAttributeInfoToContext 添加属性信息到上下文
func (c *SaleAttributeContextBuilder) addAttributeInfoToContext(amazonProduct model.Product, extraParts *[]string, missingInfo MissingInfoDetection) {
	// 添加产品描述（用于推断属性）
	if amazonProduct.Description != "" {
		*extraParts = append(*extraParts, fmt.Sprintf("\n【产品完整描述】（用于推断缺失属性，涉及ASIN: %v）\n%s",
			missingInfo.missingAttributesASINs, amazonProduct.Description))
		logrus.Debug("✅ 添加了产品描述用于属性推断")
	}

	// 添加产品特征（用于推断属性）
	if len(amazonProduct.Features) > 0 {
		*extraParts = append(*extraParts, fmt.Sprintf("\n【完整产品特征】（用于推断产品属性）\n%s",
			strings.Join(amazonProduct.Features, "\n")))
		logrus.Debugf("✅ 添加了 %d 个产品特征用于属性推断", len(amazonProduct.Features))
	}
}

// addProductDetailsToContext 添加产品详情到上下文
func (c *SaleAttributeContextBuilder) addProductDetailsToContext(amazonProduct model.Product, extraParts *[]string) {
	detailParts := c.extractPhysicalDetails(amazonProduct.ProductDetails)
	if len(detailParts) > 0 {
		*extraParts = append(*extraParts, fmt.Sprintf("\n【产品详细规格】（包含精确的尺寸重量信息）\n%s",
			strings.Join(detailParts, "\n")))
	}
}

// addVariantDetailsToContext 添加变体详情到上下文
func (c *SaleAttributeContextBuilder) addVariantDetailsToContext(variants []model.Product, extraParts *[]string) {
	if len(variants) == 0 {
		return
	}

	variantDetailsMap := make(map[string][]string)
	for _, variant := range variants {
		detailParts := c.extractPhysicalDetails(variant.ProductDetails)
		if len(detailParts) > 0 {
			variantDetailsMap[variant.Asin] = detailParts
		}
	}

	if len(variantDetailsMap) > 0 {
		var variantDetailsParts []string
		for asin, details := range variantDetailsMap {
			variantDetailsParts = append(variantDetailsParts,
				fmt.Sprintf("ASIN %s:\n  %s", asin, strings.Join(details, "\n  ")))
		}
		*extraParts = append(*extraParts, fmt.Sprintf("\n【各变体详细规格】（每个变体的精确尺寸重量）\n%s",
			strings.Join(variantDetailsParts, "\n")))
	}
}

// extractPhysicalDetails 提取物理相关的产品详情（尺寸、重量、材质等）
func (c *SaleAttributeContextBuilder) extractPhysicalDetails(productDetails []model.ProductDetail) []string {
	if len(productDetails) == 0 {
		return nil
	}

	// 定义物理相关的关键词
	physicalKeywords := []string{"dimension", "weight", "size", "material", "package"}

	var detailParts []string
	for _, detail := range productDetails {
		detailType := strings.ToLower(detail.Type)

		// 检查是否包含物理相关关键词
		for _, keyword := range physicalKeywords {
			if strings.Contains(detailType, keyword) {
				detailParts = append(detailParts, fmt.Sprintf("%s: %s", detail.Type, detail.Value))
				continue // 找到匹配就跳出内层循环
			}
		}
	}

	return detailParts
}

// getMissingPhysicalASINs 获取缺少物理信息的ASIN列表
func (c *SaleAttributeContextBuilder) getMissingPhysicalASINs(missingInfo MissingInfoDetection) []string {
	var allMissingASINs []string

	// 合并缺少尺寸和重量的ASIN，去重
	asinSet := make(map[string]bool)

	for _, asin := range missingInfo.missingDimensionsASINs {
		if !asinSet[asin] {
			allMissingASINs = append(allMissingASINs, asin)
			asinSet[asin] = true
		}
	}

	for _, asin := range missingInfo.missingWeightASINs {
		if !asinSet[asin] {
			allMissingASINs = append(allMissingASINs, asin)
			asinSet[asin] = true
		}
	}

	return allMissingASINs
}
