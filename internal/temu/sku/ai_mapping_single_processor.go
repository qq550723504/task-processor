// Package sku 提供TEMU平台的AI SKU映射单批次处理功能
package sku

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/model"
	"task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/pkg/timeout"
	temutemplate "task-processor/internal/temu/api/template"
	temucontext "task-processor/internal/temu/context"
	"task-processor/internal/temu/property"
	"task-processor/internal/temu/template"
)

// generateAISkuMappingSingleBatch 单批次生成AI SKU映射
func (vp *SkuVariantProcessor) GenerateAISkuMappingSingleBatch(temuCtx *temucontext.TemuTaskContext, variants []*model.Product) (*temucontext.AISkuMappingResponse, error) {
	// 准备AI请求数据
	vp.logger.Infof("开始准备AI请求数据，变体数量: %d", len(variants))

	aiVariants := make([]temucontext.AmazonVariantForAI, len(variants))
	// 创建ASIN到attributes的映射，用于后续填充VariantAttributes
	asinToAttributes := make(map[string]map[string]any)
	successCount := 0
	failedCount := 0

	// 创建ASIN到完整变体信息的映射
	asinToFullVariant := make(map[string]*model.Product)
	if amazonVariants := temuCtx.Variants; len(amazonVariants) > 0 {
		for _, fullVariant := range amazonVariants {
			asinToFullVariant[fullVariant.Asin] = fullVariant
		}
		vp.logger.Infof("从上下文获取到%d个完整变体信息", len(asinToFullVariant))
	}

	// 获取Amazon产品信息
	amazonProduct := temuCtx.AmazonProduct

	// 创建属性提取管理器
	attributeManager := property.NewAttributeExtractionManager(vp.logger)

	for i, variant := range variants {
		// 使用variant作为fullVariant（保持后续代码兼容）
		fullVariant := variant

		// 使用属性提取管理器提取属性
		result := attributeManager.ExtractAttributes(variant, amazonProduct, i)

		if result.Success {
			asinToAttributes[variant.Asin] = result.Attributes
			successCount++
			vp.logger.Infof("✅ 变体[%d] ASIN=%s 属性提取成功: 方法=%s, %s, Attributes=%+v",
				i, variant.Asin, result.Method, result.Details, result.Attributes)
		} else {
			failedCount++
			vp.logger.Warningf("⚠️变体[%d] ASIN=%s 属性提取失败: %s", i, variant.Asin, result.Details)
		}

		// 转换ProductDetails为map
		productDetailsMap := make(map[string]string)
		for _, detail := range fullVariant.ProductDetails {
			if detail.Type != "" && detail.Value != "" {
				productDetailsMap[detail.Type] = detail.Value
			}
		}

		// 获取物流信息（优先使用直接字段，如果为空则从ProductDetails中提取）
		productDimensions := vp.getProductDimensions(fullVariant)
		itemWeight := vp.getItemWeight(fullVariant)

		// 构建AI变体数据，只包含有值的字段
		aiVariant := vp.buildAIVariant(fullVariant, result.Attributes, productDimensions, itemWeight, productDetailsMap)

		// 记录物流信息
		if productDimensions != "" || itemWeight != "" {
			vp.logger.Infof("📦 变体[%d] ASIN=%s 物流信息: dimensions=%s, weight=%s",
				i, fullVariant.Asin, productDimensions, itemWeight)
		} else {
			vp.logger.Warnf("⚠️ 变体[%d] ASIN=%s 缺少物流信息，AI将根据产品信息估算", i, fullVariant.Asin)
		}

		aiVariants[i] = aiVariant
	}

	vp.logger.Infof("AI请求数据准备完成: 成功=%d, 失败=%d, 总计=%d", successCount, failedCount, len(aiVariants))

	// 验证ASIN与属性映射的正确性
	vp.validateAsinAttributeMapping(asinToAttributes, aiVariants)

	// 从上下文获取TEMU模板信息，合并使用goods_spec_properties和user_input_parent_spec_list
	temuSpecProperties := vp.getTemuSpecProperties(temuCtx)

	request := temucontext.VariantMappingRequest{
		ProductTitle:       amazonProduct.Title,
		Variants:           aiVariants,
		TemuSpecProperties: temuSpecProperties, // 直接使用，无需转换
	}

	// 调用AI API获取响应
	aiResponse, err := vp.callAIAPI(request)
	if err != nil {
		return nil, err
	}

	// 验证和修复AI响应，确保spec_id都来自TEMU模板
	vp.validateAndFixAIResponse(aiResponse, temuSpecProperties)

	// 验证规格数量限制，确保每个SKU最多2个规格
	vp.enforceSpecCountLimit(aiResponse)

	// 填充VariantAttributes
	vp.fillVariantAttributes(aiResponse, asinToAttributes)

	return aiResponse, nil
}

// getTemuSpecProperties 获取TEMU规格属性
func (vp *SkuVariantProcessor) getTemuSpecProperties(temuCtx *temucontext.TemuTaskContext) []temutemplate.TemplateRespGoodsSpecProperty {
	// 从上下文获取TEMU模板信息，优先使用goods_spec_properties
	var temuSpecProperties []temutemplate.TemplateRespGoodsSpecProperty
	if templateInfo, exists := template.GetTemplateInfoFromContext(temuCtx); exists {
		temuSpecProperties = templateInfo.GoodsSpecProperties
		vp.logger.Infof("成功获取TEMU模板信息，goods_spec_properties数量: %d", len(temuSpecProperties))

		// 如果goods_spec_properties为空，尝试使用user_input_parent_spec_list
		if len(temuSpecProperties) == 0 {
			if userInputSpecs, exists := template.GetUserInputParentSpecListFromContext(temuCtx); exists {
				vp.logger.Infof("goods_spec_properties为空，使用user_input_parent_spec_list，数量: %d", len(userInputSpecs))
				temuSpecProperties = vp.specHandler.convertUserInputSpecsToGoodsSpecProperties(userInputSpecs)
			} else {
				vp.logger.Warn("goods_spec_properties和user_input_parent_spec_list都为空")
			}
		}

	} else {
		vp.logger.Warn("未能从上下文获取TEMU模板信息")

		// 尝试直接获取user_input_parent_spec_list作为备选
		if userInputSpecs, exists := template.GetUserInputParentSpecListFromContext(temuCtx); exists {
			vp.logger.Infof("使用备选方案：user_input_parent_spec_list，数量: %d", len(userInputSpecs))
			temuSpecProperties = vp.specHandler.convertUserInputSpecsToGoodsSpecProperties(userInputSpecs)
		}
	}
	return temuSpecProperties
}

// callAIAPI 调用AI API
func (vp *SkuVariantProcessor) callAIAPI(request temucontext.VariantMappingRequest) (*temucontext.AISkuMappingResponse, error) {
	// 构建系统提示词和用户提示词
	systemPrompt := vp.buildSystemPrompt()
	userPrompt := vp.buildUserPrompt(request)

	// 调用AI API
	aiCtx, cancel := timeout.WithAITimeout(context.Background())
	defer cancel()

	resp, err := vp.aiClient.CreateChatCompletion(aiCtx, &openai.ChatCompletionRequest{
		Model: vp.aiClient.GetDefaultModel(),
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
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
	return vp.parseAIResponse(resp)
}

// parseAIResponse 解析AI响应
func (vp *SkuVariantProcessor) parseAIResponse(resp *openai.ChatCompletionResponse) (*temucontext.AISkuMappingResponse, error) {
	var aiResponse temucontext.AISkuMappingResponse
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
		vp.logger.Errorf("❌ AI响应被截断（达到token限制），响应长度: %d字符", len(content))
		vp.logger.Errorf("❌ 建议：增加MaxTokens参数或减少变体数量")
		return nil, fmt.Errorf("AI响应被截断，请增加MaxTokens或减少变体数量")
	}

	// 清理JSON内容，移除可能导致解析失败的字符
	jsonContent = vp.cleanJSONContent(jsonContent)

	if err := jsonx.UnmarshalBytes([]byte(jsonContent), &aiResponse, "解析AI响应失败"); err != nil {
		vp.logParseError(err, content, jsonContent)
		return nil, fmt.Errorf("解析AI响应失败: %w, 响应内容长度: %d", err, len(jsonContent))
	}

	vp.logger.Infof("AI成功生成%d个SKU映射", len(aiResponse.SkuList))
	return &aiResponse, nil
}

// logParseError 记录解析错误的详细信息
func (vp *SkuVariantProcessor) logParseError(err error, content, jsonContent string) {
	vp.logger.Errorf("❌ JSON解析失败: %v", err)
	vp.logger.Errorf("❌ 响应长度: %d字符, JSON长度: %d字符", len(content), len(jsonContent))

	// 截取部分内容用于调试（避免日志过长）
	previewLen := 500
	if len(jsonContent) < previewLen {
		previewLen = len(jsonContent)
	}
	vp.logger.Errorf("❌ JSON开头: %s", jsonContent[:previewLen])

	// 显示JSON结尾（帮助诊断截断问题）
	if len(jsonContent) > previewLen {
		endStart := len(jsonContent) - previewLen
		if endStart < 0 {
			endStart = 0
		}
		vp.logger.Errorf("❌ JSON结尾: %s", jsonContent[endStart:])
	}
}

// fillVariantAttributes 填充变体属性
func (vp *SkuVariantProcessor) fillVariantAttributes(aiResponse *temucontext.AISkuMappingResponse, asinToAttributes map[string]map[string]any) {
	vp.logger.Infof("🔄 开始填充VariantAttributes，SKU数量: %d", len(aiResponse.SkuList))

	for i := range aiResponse.SkuList {
		sku := &aiResponse.SkuList[i]
		vp.logger.Infof("🔍 处理SKU[%d]: UniqueID=%s, ASIN=%s", i, sku.UniqueID, sku.Asin)

		if attributes, exists := asinToAttributes[sku.Asin]; exists {
			sku.VariantAttributes = make(map[string]string)
			for key, value := range attributes {
				sku.VariantAttributes[key] = fmt.Sprintf("%v", value)
			}
			vp.logger.Infof("✅ 为SKU %s (ASIN: %s) 填充了VariantAttributes: %+v",
				sku.UniqueID, sku.Asin, sku.VariantAttributes)
		} else {
			vp.logger.Warnf("SKU %s (ASIN: %s) 未找到对应的attributes", sku.UniqueID, sku.Asin)
		}
	}
}

