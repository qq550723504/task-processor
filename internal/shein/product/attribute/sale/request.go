// Package sale 提供SHEIN平台销售属性的请求构建功能
package sale

import (
	"task-processor/internal/core/logger"
	"encoding/json"
	"fmt"
	"strconv"
	"task-processor/internal/model"
	sheinctx "task-processor/internal/shein/context"
	sheinattr "task-processor/internal/shein/product/attribute"

)

// SaleAttributeRequestBuilder 销售属性请求构建器
type SaleAttributeRequestBuilder struct {
	contextBuilder *SaleAttributeContextBuilder
	valueFilter    *SaleAttributeValueFilter
}

// NewSaleAttributeRequestBuilder 创建请求构建器实例
func NewSaleAttributeRequestBuilder() *SaleAttributeRequestBuilder {
	return &SaleAttributeRequestBuilder{
		contextBuilder: NewSaleAttributeContextBuilder(),
		valueFilter:    NewSaleAttributeValueFilter(),
	}
}

// BuildGenerationRequest 构建生成请求
func (r *SaleAttributeRequestBuilder) BuildGenerationRequest(
	ctx *sheinctx.TaskContext,
	productsData []map[string]string,
	attributeMetadata []sheinattr.AttributeMetadata,
	attributeNameMappings map[int]string) *sheinattr.GenerationRequest {

	var attributeMappings []sheinattr.AttributeNameMapping
	for attrID, attrName := range attributeNameMappings {
		attributeMappings = append(attributeMappings, sheinattr.AttributeNameMapping{
			AttrID:               attrID,
			VariantAttributeName: attrName,
		})
	}

	var productVariantData []sheinattr.ProductVariantData
	emptyDimensionsCount := 0
	emptyWeightCount := 0

	for _, product := range productsData {
		attributes := make(map[string]string)
		for key, value := range product {
			if key != "asin" && key != "title" && key != "price" &&
				key != "currency" && key != "productdimensions" && key != "product_details" && key != "weight" {
				if value != "" {
					attributes[key] = value
				}
			}
		}
		price := 0.0
		if priceStr, ok := product["price"]; ok {
			price, _ = strconv.ParseFloat(priceStr, 64)
		}

		// 检测空值
		dimensions := product["productdimensions"]
		weight := product["weight"]
		if dimensions == "" {
			emptyDimensionsCount++
		}
		if weight == "" {
			emptyWeightCount++
		}

		productVariantData = append(productVariantData, sheinattr.ProductVariantData{
			ASIN:       product["asin"],
			Title:      product["title"],
			Attributes: attributes,
			Price:      price,
			Dimensions: dimensions, // omitempty会自动处理空字符串
			Weight:     weight,     // omitempty会自动处理空字符串
		})
	}

	// 数据质量报告
	if emptyDimensionsCount > 0 {
		logger.GetGlobalLogger("shein/product").Warnf("⚠️ 检测到 %d/%d 个变体缺少尺寸信息", emptyDimensionsCount, len(productsData))
	}
	if emptyWeightCount > 0 {
		logger.GetGlobalLogger("shein/product").Warnf("⚠️ 检测到 %d/%d 个变体缺少重量信息", emptyWeightCount, len(productsData))
	}

	variationAttributeValues := &ctx.AmazonProduct.VariationsValues
	if ctx.Variants == nil || len(*ctx.Variants) == 0 {
		emptyVariations := []model.VariationValue{}
		variationAttributeValues = &emptyVariations
	}

	return &sheinattr.GenerationRequest{
		ProductsData:             productVariantData,
		VariationData:            ctx.AmazonProduct.Variations,
		VariationAttributeValues: variationAttributeValues,
		SaleAttributesData:       attributeMetadata,
		AttributeMappings:        attributeMappings,
		RequiredVariantCount:     len(productsData),
	}
}

// BuildUserPrompt 构建用户提示词
func (r *SaleAttributeRequestBuilder) BuildUserPrompt(ctx *sheinctx.TaskContext, request *sheinattr.GenerationRequest) string {
	saleAttributeDataBytes, _ := json.Marshal(request.SaleAttributesData)
	productsDataBytes, _ := json.Marshal(request.ProductsData)
	attributeMappingBytes, _ := json.Marshal(request.AttributeMappings)
	productContext := r.contextBuilder.BuildCompactProductContext(*ctx.AmazonProduct, *ctx.Variants)
	isSingleVariant := ctx.Variants == nil || len(*ctx.Variants) == 0

	var originalAttributeValues string
	var attributeValueHint string
	if len(ctx.AmazonProduct.VariationsValues) > 0 {
		originalAttributeValuesBytes, _ := json.Marshal(ctx.AmazonProduct.VariationsValues)
		originalAttributeValues = string(originalAttributeValuesBytes)
		attributeValueHint = "注意：以上属性值必须完全按原样使用，包括大小写、空格、标点符号等，不得进行任何修改！\n⚠️ 重要：saleAttributes中只包含变体实际使用的属性值，不要生成所有可选项！"
		logger.GetGlobalLogger("shein/product").Infof("📋 原始属性值列表（variations_values）:")
		for i, vv := range ctx.AmazonProduct.VariationsValues {
			logger.GetGlobalLogger("shein/product").Infof("  [%d] %s: %v", i+1, vv.VariantName, vv.Values)
		}
	} else {
		originalAttributeValues = "[]"
		if isSingleVariant {
			attributeValueHint = "注意：这是单变体产品，请从【产品物理信息】和【产品核心信息】中推断合理的属性值。"
		} else {
			logger.GetGlobalLogger("shein/product").Warnf("⚠️ 警告：AmazonProduct.VariationsValues 为空")
			attributeValueHint = "注意：原始属性值列表为空，请从产品信息中推断。"
		}
	}

	var productTypeHint string
	if isSingleVariant {
		productTypeHint = "\n⚠️ 特别注意：这是单变体产品（只有1个SKU），请：\n1. 只生成1个variant\n2. 从候选属性值中选择最合理的值（通常选择第一个或最通用的值）\n3. 确保所有必填属性都有值\n4. 缺少物理尺寸和重量的必须合理估算"
	} else {
		productTypeHint = fmt.Sprintf("\n这是多变体产品（共%d个SKU），请为每个ASIN生成对应的variant。", request.RequiredVariantCount)
	}

	// 智能降级：检测数据完整性，决定是否需要额外上下文
	var extraContextSection string
	extraContextSection = r.contextBuilder.BuildExtraContext(*ctx.AmazonProduct, *ctx.Variants, request.ProductsData)
	if extraContextSection != "" {
		logger.GetGlobalLogger("shein/product").Info("📋 检测到关键信息缺失，已提供额外上下文帮助AI提取或估算")
	}

	return fmt.Sprintf(`【任务】为%d个Amazon产品生成SHEIN销售属性%s

【产品信息】
%s

【变体列表】（ASIN、标题、属性、物理信息）
%s

【原始属性值】%s
%s

【可用销售属性】（已过滤，只显示相关候选值）
%s

【属性映射】
%s%s

⚠️ 重要提醒：
1. saleAttributes中的attrValue数组只包含变体实际使用的属性值，不要包含所有可选项
2. 避免重复数据：同一个属性值在attrValue数组中只能出现一次
3. 属性值必须与原始数据完全一致，包括大小写、空格、标点符号
4. quantityType判断：仔细分析每个变体的Title，识别"X Pack/Pcs/Pairs"(同款多件=2)、"Set/Kit"(单套=3)、"X Sets"(多套=4)等关键词，无则为单品=1

请生成准确的销售属性和变体数据。`,
		request.RequiredVariantCount,
		productTypeHint,
		productContext,
		string(productsDataBytes),
		func() string {
			if len(originalAttributeValues) > 2 {
				return "\n" + originalAttributeValues
			}
			return ""
		}(),
		attributeValueHint,
		string(saleAttributeDataBytes),
		string(attributeMappingBytes),
		extraContextSection)
}


