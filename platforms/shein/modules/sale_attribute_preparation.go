package modules

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"task-processor/common/amazon"
	"task-processor/common/shein/api/attribute"

	"github.com/sirupsen/logrus"
)

// prepareProductsData 准备产品数据
func (h *SaleAttributeHandler) prepareProductsData(ctx *TaskContext) []map[string]string {
	var productsData []map[string]string

	// 检查是否有变体数据
	hasVariants := ctx.Variants != nil && len(*ctx.Variants) > 0

	if !hasVariants {
		// 单体产品：使用主产品信息
		logrus.Infof("📦 检测到单体产品，使用主产品信息")
		if ctx.AmazonProduct != nil {
			productDetails := map[string]string{
				"asin":              ctx.AmazonProduct.Asin,
				"title":             ctx.AmazonProduct.Title,
				"price":             strconv.FormatFloat(ctx.AmazonProduct.FinalPrice, 'f', -1, 64),
				"currency":          ctx.AmazonProduct.Currency,
				"productdimensions": ctx.AmazonProduct.ProductDimensions,
				"weight":            ctx.AmazonProduct.ItemWeight, // 添加重量信息
			}

			// 从主产品的ProductDetails中提取属性信息
			if len(ctx.AmazonProduct.ProductDetails) > 0 {
				for _, detail := range ctx.AmazonProduct.ProductDetails {
					// 将产品详情作为属性添加
					productDetails[detail.Type] = detail.Value
					logrus.Debugf("✅ 为单体产品添加属性: %s = %s", detail.Type, detail.Value)
				}
			}

			productsData = append(productsData, productDetails)
		}
	} else {
		// 多变体产品：使用变体信息
		logrus.Infof("📊 检测到多变体产品，变体数量: %d", len(*ctx.Variants))

		// 调试：检查主产品的Variations数据
		if ctx.AmazonProduct != nil {
			logrus.Infof("🔍 主产品Variations数量: %d", len(ctx.AmazonProduct.Variations))
			if len(ctx.AmazonProduct.Variations) > 0 {
				logrus.Infof("🔍 第一个Variation示例: ASIN=%s, Attributes=%v",
					ctx.AmazonProduct.Variations[0].Asin,
					ctx.AmazonProduct.Variations[0].Attributes)
			}
		}

		// 转换为所需格式
		for _, variant := range *ctx.Variants {
			productDetails := map[string]string{
				"asin":              variant.Asin,
				"title":             variant.Title,
				"price":             strconv.FormatFloat(variant.FinalPrice, 'f', -1, 64),
				"currency":          variant.Currency,
				"productdimensions": variant.ProductDimensions,
				"weight":            variant.ItemWeight, // 添加重量信息
			}

			// 关键修复：从主产品的Variations字段中提取该变体的属性信息
			attributeFound := false
			if ctx.AmazonProduct != nil && len(ctx.AmazonProduct.Variations) > 0 {
				for _, variation := range ctx.AmazonProduct.Variations {
					if variation.Asin == variant.Asin {
						if len(variation.Attributes) > 0 {
							// 将属性信息添加到productDetails中
							for attrKey, attrValue := range variation.Attributes {
								// 将interface{}转换为字符串
								attrValueStr := fmt.Sprintf("%v", attrValue)
								productDetails[attrKey] = attrValueStr
								logrus.Debugf("✅ 为ASIN %s 添加属性: %s = %s", variant.Asin, attrKey, attrValueStr)
							}
							attributeFound = true
						} else {
							logrus.Warnf("⚠️ ASIN %s 的Attributes为空", variant.Asin)
						}
						break
					}
				}
				if !attributeFound {
					logrus.Warnf("⚠️ 未找到ASIN %s 在主产品Variations中的匹配项", variant.Asin)
				}
			} else {
				logrus.Warnf("⚠️ 主产品的Variations为空，无法提取属性信息")
			}

			productsData = append(productsData, productDetails)
		}
	}

	logrus.Infof("✅ 准备了 %d 个产品数据（包含属性信息）", len(productsData))
	return productsData
}

// buildAttributeMetadata 构建属性元数据
func (h *SaleAttributeHandler) buildAttributeMetadata(ctx *TaskContext, importanceCalc *AttributeImportanceCalculator) []AttributeMetadata {
	var attributeMetadata []AttributeMetadata
	isSingleVariant := ctx.Variants == nil || len(*ctx.Variants) == 0

	for _, saleAttr := range ctx.BuildAttributeData.SaleAttributeData {
		metadata := AttributeMetadata{
			AttrID:    saleAttr.AttrID,
			AttrValue: append([]GenerateAttributeValue{}, saleAttr.AttrValue...),
			Required:  saleAttr.Required,
			Type:      saleAttr.Type,
		}

		// 从属性模板中查找对应的属性信息
		if ctx.AttributeTemplates != nil && len(ctx.AttributeTemplates.Data) > 0 {
			for _, attribute := range ctx.AttributeTemplates.Data[0].AttributeInfos {
				if attribute.AttributeID == saleAttr.AttrID {
					metadata.Importance = importanceCalc.CalculateImportance(&attribute)
					metadata.AttributeName = attribute.AttributeName
					metadata.AttributeNameEn = attribute.AttributeNameEn
					metadata.VariantName = h.findMappedName(saleAttr.AttrID, ctx.AttributeTemplates)
					break
				}
			}
		}

		if metadata.VariantName == "" {
			metadata.VariantName = fmt.Sprintf("attr_%d", saleAttr.AttrID)
		}

		// 单变体产品优化
		if isSingleVariant && len(metadata.AttrValue) > 3 {
			logrus.Debugf("单变体产品：属性 %s (ID:%d) 的候选值从 %d 个简化为 3 个",
				metadata.AttributeNameEn, metadata.AttrID, len(metadata.AttrValue))
			metadata.AttrValue = metadata.AttrValue[:3]
		}

		// 多变体产品优化：根据实际变体值过滤候选列表
		if !isSingleVariant && ctx.AmazonProduct != nil {
			metadata.AttrValue = h.filterAttributeValuesByActualUsage(
				metadata.AttrValue,
			)
		}

		attributeMetadata = append(attributeMetadata, metadata)
	}

	return attributeMetadata
}

// CalculateImportance 计算属性重要性
func (calc *AttributeImportanceCalculator) CalculateImportance(attribute *attribute.AttributeInfo) int {
	importance := 0
	if len(attribute.AttributeRemarkList) > 0 {
		importance += calc.rules.RemarkListScore
	}
	if attribute.AttributeLabel == 1 {
		importance += calc.rules.RequiredScore
	}
	if attribute.IsSample == 1 {
		importance += calc.rules.SampleScore
	}
	if attribute.AttributeStatus == 3 {
		importance += calc.rules.ActiveScore
	}
	if attribute.AttributeIsShow == 1 {
		importance += calc.rules.DisplayScore
	}
	return importance
}

// findMappedName 查找映射的属性名称
func (h *SaleAttributeHandler) findMappedName(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) string {
	if attributeTemplates == nil || len(attributeTemplates.Data) == 0 {
		return ""
	}
	for _, attribute := range attributeTemplates.Data[0].AttributeInfos {
		if attribute.AttributeID == attrID {
			if attribute.AttributeNameEn != "" {
				return attribute.AttributeNameEn
			}
			if attribute.AttributeName != "" {
				return attribute.AttributeName
			}
			break
		}
	}
	return ""
}

// buildAttributeNameMappings 构建属性名称映射
func (h *SaleAttributeHandler) buildAttributeNameMappings(
	attributeData BuildAttributeInfo,
	attributeTemplates *attribute.AttributeTemplateInfo,
) map[int]string {
	mappings := make(map[int]string)
	for _, saleAttr := range attributeData.SaleAttributeData {
		if mappedName := h.findMappedName(saleAttr.AttrID, attributeTemplates); mappedName != "" {
			mappings[saleAttr.AttrID] = mappedName
		} else {
			mappings[saleAttr.AttrID] = fmt.Sprintf("attr_%d", saleAttr.AttrID)
		}
	}
	return mappings
}

// filterAttributeValuesByActualUsage 根据实际变体值过滤属性候选列表
func (h *SaleAttributeHandler) filterAttributeValuesByActualUsage(
	candidateValues []GenerateAttributeValue,
) []GenerateAttributeValue {
	return candidateValues
}

// filterVariantsByRules 在生成销售属性之前过滤变体
func (h *SaleAttributeHandler) filterVariantsByRules(ctx *TaskContext) {
	if ctx.Variants == nil {
		return
	}
	filteredVariants := make([]amazon.Product, 0, len(*ctx.Variants))
	filteredOutCount := 0
	for _, variant := range *ctx.Variants {
		filterInfo := ctx.GetVariantFilterInfo(variant.Asin)
		if filterInfo != nil && filterInfo.FilteredOut {
			logrus.Infof("变体ASIN %s 已被筛选规则排除: %s，将被排除\n", variant.Asin, filterInfo.FilterReason)
			filteredOutCount++
		} else {
			filteredVariants = append(filteredVariants, variant)
		}
	}
	*ctx.Variants = filteredVariants
	logrus.Infof("在生成销售属性之前，已过滤掉 %d 个不符合筛选规则的变体，剩余 %d 个变体\n", filteredOutCount, len(filteredVariants))
}

// filterVariantsByRulesAfterGeneration 在生成销售属性之后过滤变体
func (h *SaleAttributeHandler) filterVariantsByRulesAfterGeneration(ctx *TaskContext, saleAttributeData *ResultSaleAttribute) {
	if saleAttributeData == nil {
		return
	}
	filteredVariants := make([]Variant, 0, len(saleAttributeData.Variants))
	filteredOutCount := 0
	for _, variant := range saleAttributeData.Variants {
		filterInfo := ctx.GetVariantFilterInfo(variant.ASIN)
		if filterInfo != nil && filterInfo.FilteredOut {
			logrus.Infof("变体ASIN %s 已被筛选规则排除: %s，将被排除\n", variant.ASIN, filterInfo.FilterReason)
			filteredOutCount++
			continue
		}
		filteredVariants = append(filteredVariants, variant)
	}
	saleAttributeData.Variants = filteredVariants
	logrus.Infof("在生成销售属性之后，已过滤掉 %d 个不符合筛选规则的变体，剩余 %d 个变体\n", filteredOutCount, len(filteredVariants))
}

// buildCompactProductContext 构建精简的产品上下文信息
func (h *SaleAttributeHandler) buildCompactProductContext(ctx *TaskContext) string {
	var contextParts []string
	if ctx.AmazonProduct.Title != "" {
		contextParts = append(contextParts, fmt.Sprintf("标题: %s", ctx.AmazonProduct.Title))
	}
	if ctx.AmazonProduct.Brand != "" {
		contextParts = append(contextParts, fmt.Sprintf("品牌: %s", ctx.AmazonProduct.Brand))
	}
	if len(ctx.AmazonProduct.Categories) > 0 {
		contextParts = append(contextParts, fmt.Sprintf("分类: %s", strings.Join(ctx.AmazonProduct.Categories, " > ")))
	}
	if len(ctx.AmazonProduct.Features) > 0 {
		featureCount := len(ctx.AmazonProduct.Features)
		if featureCount > 3 {
			featureCount = 3
		}
		contextParts = append(contextParts, fmt.Sprintf("关键特征: %s",
			strings.Join(ctx.AmazonProduct.Features[:featureCount], "; ")))
	} else if ctx.AmazonProduct.Description != "" {
		desc := ctx.AmazonProduct.Description
		if len(desc) > 200 {
			desc = desc[:200] + "..."
		}
		contextParts = append(contextParts, fmt.Sprintf("描述: %s", desc))
	}
	variantCount := 1
	if ctx.Variants != nil {
		variantCount = len(*ctx.Variants)
	}
	contextParts = append(contextParts, fmt.Sprintf("变体数: %d", variantCount))
	return strings.Join(contextParts, "\n")
}

// buildGenerationRequest 构建生成请求
func (h *SaleAttributeHandler) buildGenerationRequest(
	ctx *TaskContext,
	productsData []map[string]string,
	attributeMetadata []AttributeMetadata,
	attributeNameMappings map[int]string) *GenerationRequest {
	var attributeMappings []AttributeNameMapping
	for attrID, attrName := range attributeNameMappings {
		attributeMappings = append(attributeMappings, AttributeNameMapping{
			AttrID:               attrID,
			VariantAttributeName: attrName,
		})
	}

	var productVariantData []ProductVariantData
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

		productVariantData = append(productVariantData, ProductVariantData{
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
		logrus.Warnf("⚠️ 检测到 %d/%d 个变体缺少尺寸信息", emptyDimensionsCount, len(productsData))
	}
	if emptyWeightCount > 0 {
		logrus.Warnf("⚠️ 检测到 %d/%d 个变体缺少重量信息", emptyWeightCount, len(productsData))
	}

	variationAttributeValues := &ctx.AmazonProduct.VariationsValues
	if ctx.Variants == nil || len(*ctx.Variants) == 0 {
		emptyVariations := []amazon.VariationValue{}
		variationAttributeValues = &emptyVariations
	}

	return &GenerationRequest{
		ProductsData:             productVariantData,
		VariationData:            nil,
		VariationAttributeValues: variationAttributeValues,
		SaleAttributesData:       attributeMetadata,
		AttributeMappings:        attributeMappings,
		RequiredVariantCount:     len(productsData),
	}
}

// buildUserPrompt 构建用户提示词
func (h *SaleAttributeHandler) buildUserPrompt(ctx *TaskContext, request *GenerationRequest) string {
	saleAttributeDataBytes, _ := json.Marshal(request.SaleAttributesData)
	productsDataBytes, _ := json.Marshal(request.ProductsData)
	attributeMappingBytes, _ := json.Marshal(request.AttributeMappings)
	productContext := h.buildCompactProductContext(ctx)
	isSingleVariant := ctx.Variants == nil || len(*ctx.Variants) == 0

	var originalAttributeValues string
	var attributeValueHint string
	if len(ctx.AmazonProduct.VariationsValues) > 0 {
		originalAttributeValuesBytes, _ := json.Marshal(ctx.AmazonProduct.VariationsValues)
		originalAttributeValues = string(originalAttributeValuesBytes)
		attributeValueHint = "注意：以上属性值必须完全按原样使用，包括大小写、空格、标点符号等，不得进行任何修改！"
		logrus.Infof("📋 原始属性值列表（variations_values）:")
		for i, vv := range ctx.AmazonProduct.VariationsValues {
			logrus.Infof("  [%d] %s: %v", i+1, vv.VariantName, vv.Values)
		}
	} else {
		originalAttributeValues = "[]"
		if isSingleVariant {
			attributeValueHint = "注意：这是单变体产品，请从【产品物理信息】和【产品核心信息】中推断合理的属性值。"
		} else {
			logrus.Warnf("⚠️ 警告：AmazonProduct.VariationsValues 为空")
			attributeValueHint = "注意：原始属性值列表为空，请从产品信息中推断。"
		}
	}

	var productTypeHint string
	if isSingleVariant {
		productTypeHint = "\n⚠️ 特别注意：这是单变体产品（只有1个SKU），请：\n1. 只生成1个variant\n2. 从候选属性值中选择最合理的值（通常选择第一个或最通用的值）\n3. 确保所有必填属性都有值\n4. 物理尺寸和重量必须合理估算"
	} else {
		productTypeHint = fmt.Sprintf("\n这是多变体产品（共%d个SKU），请为每个ASIN生成对应的variant。", request.RequiredVariantCount)
	}

	// 智能降级：检测数据完整性，决定是否需要额外上下文
	needsExtraContext := h.shouldProvideExtraContext(ctx, request.ProductsData)
	var extraContextSection string
	if needsExtraContext {
		extraContextSection = h.buildExtraContext(ctx)
		if extraContextSection != "" {
			logrus.Info("📋 检测到关键信息缺失，已提供额外上下文帮助AI提取或估算")
		}
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

// shouldProvideExtraContext 判断是否需要提供额外上下文
func (h *SaleAttributeHandler) shouldProvideExtraContext(ctx *TaskContext, productsData []ProductVariantData) bool {
	emptyAttributesCount := 0
	emptyDimensionsCount := 0

	for _, product := range productsData {
		if len(product.Attributes) == 0 {
			emptyAttributesCount++
		}
		if product.Dimensions == "" && product.Weight == "" {
			emptyDimensionsCount++
		}
	}

	// 触发条件：任何变体缺少物理尺寸，或超过50%变体缺少属性
	threshold := len(productsData) / 2
	needsForDimensions := emptyDimensionsCount > 0
	needsForAttributes := emptyAttributesCount > threshold

	if needsForDimensions {
		logrus.Warnf("⚠️ 检测到 %d/%d 个变体缺少物理尺寸信息，将提供详细信息帮助AI估算",
			emptyDimensionsCount, len(productsData))
	}
	if needsForAttributes {
		logrus.Warnf("⚠️ 检测到 %d/%d 个变体缺少属性信息，将提供详细信息帮助AI推断",
			emptyAttributesCount, len(productsData))
	}

	return needsForDimensions || needsForAttributes
}

// buildExtraContext 构建额外上下文信息
func (h *SaleAttributeHandler) buildExtraContext(ctx *TaskContext) string {
	var extraParts []string

	logrus.Debug("🔍 开始构建额外上下文...")

	// 添加完整的产品描述
	if ctx.AmazonProduct.Description != "" {
		extraParts = append(extraParts, fmt.Sprintf("\n【产品完整描述】（用于推断属性和估算尺寸）\n%s",
			ctx.AmazonProduct.Description))
		logrus.Debug("✅ 添加了产品描述")
	} else {
		logrus.Debug("⚠️ 产品描述为空")
	}

	// 添加所有产品特征
	if len(ctx.AmazonProduct.Features) > 0 {
		extraParts = append(extraParts, fmt.Sprintf("\n【完整产品特征】（可能包含尺寸、重量、材质等信息）\n%s",
			strings.Join(ctx.AmazonProduct.Features, "\n")))
		logrus.Debugf("✅ 添加了 %d 个产品特征", len(ctx.AmazonProduct.Features))
	} else {
		logrus.Debug("⚠️ 产品特征为空")
	}

	// 添加主产品的物理信息
	if ctx.AmazonProduct.ProductDimensions != "" || ctx.AmazonProduct.ItemWeight != "" {
		var physicalInfo []string
		if ctx.AmazonProduct.ProductDimensions != "" {
			physicalInfo = append(physicalInfo, fmt.Sprintf("包装尺寸: %s", ctx.AmazonProduct.ProductDimensions))
		}
		if ctx.AmazonProduct.ItemWeight != "" {
			physicalInfo = append(physicalInfo, fmt.Sprintf("产品重量: %s", ctx.AmazonProduct.ItemWeight))
		}
		extraParts = append(extraParts, fmt.Sprintf("\n【主产品物理信息】（作为参考或估算基准）\n%s",
			strings.Join(physicalInfo, "\n")))
	}

	// 添加ProductDetails（提取尺寸、重量、材质相关信息）
	if len(ctx.AmazonProduct.ProductDetails) > 0 {
		var detailParts []string
		for _, detail := range ctx.AmazonProduct.ProductDetails {
			detailType := strings.ToLower(detail.Type)
			if strings.Contains(detailType, "dimension") ||
				strings.Contains(detailType, "weight") ||
				strings.Contains(detailType, "size") ||
				strings.Contains(detailType, "material") ||
				strings.Contains(detailType, "package") {
				detailParts = append(detailParts, fmt.Sprintf("%s: %s", detail.Type, detail.Value))
			}
		}
		if len(detailParts) > 0 {
			extraParts = append(extraParts, fmt.Sprintf("\n【产品详细规格】（包含精确的尺寸重量信息）\n%s",
				strings.Join(detailParts, "\n")))
		}
	}

	// 添加各变体的ProductDetails
	if ctx.Variants != nil && len(*ctx.Variants) > 0 {
		variantDetailsMap := make(map[string][]string)
		for _, variant := range *ctx.Variants {
			if len(variant.ProductDetails) > 0 {
				var variantDetails []string
				for _, detail := range variant.ProductDetails {
					detailType := strings.ToLower(detail.Type)
					if strings.Contains(detailType, "dimension") ||
						strings.Contains(detailType, "weight") ||
						strings.Contains(detailType, "size") ||
						strings.Contains(detailType, "package") {
						variantDetails = append(variantDetails, fmt.Sprintf("%s: %s", detail.Type, detail.Value))
					}
				}
				if len(variantDetails) > 0 {
					variantDetailsMap[variant.Asin] = variantDetails
				}
			}
		}

		if len(variantDetailsMap) > 0 {
			var variantDetailsParts []string
			for asin, details := range variantDetailsMap {
				variantDetailsParts = append(variantDetailsParts,
					fmt.Sprintf("ASIN %s:\n  %s", asin, strings.Join(details, "\n  ")))
			}
			extraParts = append(extraParts, fmt.Sprintf("\n【各变体详细规格】（每个变体的精确尺寸重量）\n%s",
				strings.Join(variantDetailsParts, "\n")))
		}
	}

	if len(extraParts) == 0 {
		logrus.Warn("⚠️ 额外上下文为空：所有可用信息源（Description、Features、ProductDetails等）都为空")
		return ""
	}

	logrus.Infof("✅ 构建了额外上下文，包含 %d 个信息块", len(extraParts))
	return strings.Join(extraParts, "\n")
}
