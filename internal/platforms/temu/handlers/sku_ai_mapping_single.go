package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"task-processor/internal/common/amazon/model"
	"task-processor/internal/common/pipeline"
	"task-processor/internal/clients/openai"
)

// generateAISkuMappingSingleBatch 单批次生成AI SKU映射
func (sb *SkuBuilder) generateAISkuMappingSingleBatch(ctx *pipeline.TaskContext, variants []*model.Product) (*AISkuMappingResponse, error) {

	// 准备AI请求数据
	sb.logger.Infof("开始准备AI请求数据，变体数量: %d", len(variants))

	aiVariants := make([]AmazonVariantForAI, len(variants))
	// 创建ASIN到attributes的映射，用于后续填充VariantAttributes
	asinToAttributes := make(map[string]map[string]any)
	successCount := 0
	failedCount := 0

	// 创建ASIN到完整变体信息的映射
	asinToFullVariant := make(map[string]*model.Product)
	if ctx.AmazonVariants != nil {
		for _, fullVariant := range ctx.AmazonVariants {
			asinToFullVariant[fullVariant.Asin] = fullVariant
		}
		sb.logger.Infof("从上下文获取到%d个完整变体信息", len(asinToFullVariant))
	}

	for i, variant := range variants {
		// 使用variant作为fullVariant（保持后续代码兼容）
		fullVariant := variant

		// 提取变体的属性信息
		// 关键修复：优先使用索引从父产品的Variations中获取正确的attributes
		// 因为同一个ASIN可能对应多个不同的尺寸组合
		attributes := make(map[string]any)

		// 方法1：优先从父产品的Variations中按索引获取（最可靠）
		// 这样可以避免ASIN重复导致的匹配错误
		if ctx.AmazonProduct != nil && i < len(ctx.AmazonProduct.Variations) {
			variation := ctx.AmazonProduct.Variations[i]
			if len(variation.Attributes) > 0 {
				attributes = variation.Attributes
				asinToAttributes[variant.Asin] = attributes
				successCount++
				sb.logger.Infof("✅ 变体[%d]从父产品Variations匹配（按索引）: ASIN=%s, Attributes=%+v", i, variant.Asin, attributes)
			} else {
				sb.logger.Warnf("⚠️ 变体[%d]从父产品Variations找到但Attributes为空: ASIN=%s", i, variant.Asin)
			}
		}

		// 方法2：如果方法1失败，尝试从variant自己的Variations中获取
		if len(attributes) == 0 && len(variant.Variations) > 0 {
			sb.logger.Infof("🔍 变体[%d]从自身Variations中查找: ASIN=%s", i, variant.Asin)
			for _, variation := range variant.Variations {
				if variation.Asin == variant.Asin {
					if len(variation.Attributes) > 0 {
						attributes = variation.Attributes
						asinToAttributes[variant.Asin] = attributes
						successCount++
						sb.logger.Infof("✅ 变体[%d]从自身Variations匹配: ASIN=%s, Attributes=%+v", i, variant.Asin, attributes)
					}
					break
				}
			}
		}

		// 如果还是没有属性，检查是否为单一产品（没有变体）
		if len(attributes) == 0 {
			// 对于单一产品（没有变体），这是正常情况
			if len(variants) == 1 {
				sb.logger.Infof("ℹ️ 单一产品（无变体），AI将根据产品标题和描述生成规格: ASIN=%s", variant.Asin)
			} else {
				// 对于多变体产品，缺少attributes是个问题
				failedCount++
				sb.logger.Errorf("❌ 变体[%d]无法获取Attributes: ASIN=%s, 这将导致AI生成不完整的规格", i, variant.Asin)
				sb.logger.Errorf("   变体信息: Title=%s, Variations数量=%d", variant.Title, len(variant.Variations))
			}
		}

		// 转换ProductDetails为map
		productDetailsMap := make(map[string]string)
		for _, detail := range fullVariant.ProductDetails {
			if detail.Type != "" && detail.Value != "" {
				productDetailsMap[detail.Type] = detail.Value
			}
		}

		aiVariant := AmazonVariantForAI{
			Name:              fullVariant.Title,
			Asin:              fullVariant.Asin,
			Price:             fullVariant.FinalPrice,
			Image:             fullVariant.ImageURL,
			Attributes:        attributes,
			ProductDimensions: fullVariant.ProductDimensions,
			ItemWeight:        fullVariant.ItemWeight,
			Description:       fullVariant.Description,
			Features:          fullVariant.Features,
			ProductDetails:    productDetailsMap,
		}

		// 记录物流信息
		if fullVariant.ProductDimensions != "" || fullVariant.ItemWeight != "" {
			sb.logger.Infof("📦 变体[%d] ASIN=%s 物流信息: dimensions=%s, weight=%s",
				i, fullVariant.Asin, fullVariant.ProductDimensions, fullVariant.ItemWeight)
		} else {
			sb.logger.Warnf("⚠️ 变体[%d] ASIN=%s 缺少物流信息，AI将根据产品信息估算", i, fullVariant.Asin)
		}

		// 记录Description和Features信息
		if fullVariant.Description != "" {
			sb.logger.Infof("📝 变体[%d] ASIN=%s 有描述信息，长度: %d", i, fullVariant.Asin, len(fullVariant.Description))
		} else {
			sb.logger.Warnf("⚠️ 变体[%d] ASIN=%s 缺少描述信息", i, fullVariant.Asin)
		}
		if len(fullVariant.Features) > 0 {
			sb.logger.Infof("📝 变体[%d] ASIN=%s 有特性信息，数量: %d", i, fullVariant.Asin, len(fullVariant.Features))
		} else {
			sb.logger.Warnf("⚠️ 变体[%d] ASIN=%s 缺少特性信息", i, fullVariant.Asin)
		}

		aiVariants[i] = aiVariant
	}

	sb.logger.Infof("AI请求数据准备完成: 成功=%d, 失败=%d, 总计=%d", successCount, failedCount, len(aiVariants))

	productTitle := "Product"
	if ctx.AmazonProduct != nil {
		productTitle = ctx.AmazonProduct.Title
	}

	// 从上下文获取TEMU模板信息，优先使用goods_spec_properties
	var temuSpecProperties []GoodsSpecProperty
	if templateInfo, exists := GetTemplateInfoFromContext(ctx); exists {
		temuSpecProperties = templateInfo.GoodsSpecProperties
		sb.logger.Infof("成功获取TEMU模板信息，goods_spec_properties数量: %d", len(temuSpecProperties))

		// 如果goods_spec_properties为空，尝试使用user_input_parent_spec_list
		if len(temuSpecProperties) == 0 {
			if userInputSpecs, exists := GetUserInputParentSpecListFromContext(ctx); exists {
				sb.logger.Infof("goods_spec_properties为空，使用user_input_parent_spec_list，数量: %d", len(userInputSpecs))
				temuSpecProperties = sb.convertUserInputSpecsToGoodsSpecProperties(userInputSpecs)
			} else {
				sb.logger.Warn("goods_spec_properties和user_input_parent_spec_list都为空")
			}
		}

	} else {
		sb.logger.Warn("未能从上下文获取TEMU模板信息")

		// 尝试直接获取user_input_parent_spec_list作为备选
		if userInputSpecs, exists := GetUserInputParentSpecListFromContext(ctx); exists {
			sb.logger.Infof("使用备选方案：user_input_parent_spec_list，数量: %d", len(userInputSpecs))
			temuSpecProperties = sb.convertUserInputSpecsToGoodsSpecProperties(userInputSpecs)
		}
	}

	request := VariantMappingRequest{
		ProductTitle:       productTitle,
		Variants:           aiVariants,
		Instructions:       sb.getAIInstructions(),
		TemuSpecProperties: temuSpecProperties,
	}

	// 构建AI提示
	prompt := sb.buildAIPrompt(request)

	sb.logger.Infof("不限制MaxTokens，允许AI自由生成完整响应 (变体数=%d)", len(variants))

	// 调用AI API
	aiCtx := context.Background()
	resp, err := sb.aiClient.CreateChatCompletion(aiCtx, &openai.ChatCompletionRequest{
		Model: sb.aiClient.GetDefaultModel(),
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    "system",
				Content: "你是一个专业的电商产品数据转换专家，擅长将Amazon产品变体转换为TEMU平台的SKC/SKU结构。",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: func() *float32 { t := float32(0.1); return &t }(),
		MaxTokens:   nil, // 不限制输出token数量
	})

	if err != nil {
		return nil, fmt.Errorf("调用AI API失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("AI响应为空")
	}

	// 解析AI响应
	var aiResponse AISkuMappingResponse
	content := resp.Choices[0].Message.Content

	// 提取JSON部分（去除可能的解释文本）
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}") + 1
	if jsonStart == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("AI响应中未找到有效JSON")
	}

	jsonContent := content[jsonStart:jsonEnd]

	// 检查响应是否被截断
	if resp.Choices[0].FinishReason == "length" {
		sb.logger.Errorf("❌ AI响应被截断（达到token限制），响应长度: %d字符", len(content))
		sb.logger.Errorf("❌ 建议：增加MaxTokens参数或减少变体数量")
		return nil, fmt.Errorf("AI响应被截断，请增加MaxTokens或减少变体数量")
	}

	if err := json.Unmarshal([]byte(jsonContent), &aiResponse); err != nil {
		// 提供更详细的错误信息
		sb.logger.Errorf("❌ JSON解析失败: %v", err)
		sb.logger.Errorf("❌ 响应长度: %d字符, JSON长度: %d字符", len(content), len(jsonContent))
		sb.logger.Errorf("❌ FinishReason: %s", resp.Choices[0].FinishReason)

		// 截取部分内容用于调试（避免日志过长）
		previewLen := 500
		if len(jsonContent) < previewLen {
			previewLen = len(jsonContent)
		}
		sb.logger.Errorf("❌ JSON开头: %s", jsonContent[:previewLen])

		// 显示JSON结尾（帮助诊断截断问题）
		if len(jsonContent) > previewLen {
			endStart := len(jsonContent) - previewLen
			if endStart < 0 {
				endStart = 0
			}
			sb.logger.Errorf("❌ JSON结尾: %s", jsonContent[endStart:])
		}

		return nil, fmt.Errorf("解析AI响应失败: %w, 响应内容长度: %d", err, len(jsonContent))
	}

	// 验证和修复AI响应，确保spec_id都来自TEMU模板
	sb.validateAndFixAIResponse(&aiResponse, temuSpecProperties)

	// 验证规格数量限制，确保每个SKU最多2个规格
	sb.enforceSpecCountLimit(&aiResponse)

	// 调试：打印asinToAttributes映射内容
	sb.logger.Infof("🔍 asinToAttributes映射内容 (共%d个):", len(asinToAttributes))
	for asin, attrs := range asinToAttributes {
		sb.logger.Infof("  ASIN: %s -> Attributes: %+v", asin, attrs)
	}

	// 填充VariantAttributes：将匹配到的Amazon attributes转换为string格式
	sb.logger.Infof("🔄 开始填充VariantAttributes，SKU数量: %d", len(aiResponse.SkuList))
	for i := range aiResponse.SkuList {
		sku := &aiResponse.SkuList[i]
		sb.logger.Infof("🔍 处理SKU[%d]: UniqueID=%s, ASIN=%s", i, sku.UniqueID, sku.Asin)

		if attributes, exists := asinToAttributes[sku.Asin]; exists {
			sku.VariantAttributes = make(map[string]string)
			for key, value := range attributes {
				sku.VariantAttributes[key] = fmt.Sprintf("%v", value)
			}
			sb.logger.Infof("✅ 为SKU %s (ASIN: %s) 填充了VariantAttributes: %+v",
				sku.UniqueID, sku.Asin, sku.VariantAttributes)
		} else {
			sb.logger.Warnf("SKU %s (ASIN: %s) 未找到对应的attributes", sku.UniqueID, sku.Asin)
		}
	}

	sb.logger.Infof("AI成功生成%d个SKU映射", len(aiResponse.SkuList))
	return &aiResponse, nil
}
