// Package attribute 提供SHEIN平台AI属性选择核心处理器
package attribute

import (
	"task-processor/internal/core/logger"
	"encoding/json"
	"fmt"
	"strings"

	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/shein"
	"task-processor/internal/shein/aicache"
	"task-processor/internal/shein/api/attribute"

)

// AttributeSelectorHandler AI属性选择处理器
type AttributeSelectorHandler struct {
	openaiClient      *openaiClient.Client
	promptGenerator   *AttributePromptGenerator
	validator         *AttributeSelectionValidator
	importanceService *ImportanceService
	utils             *AttributeUtils
}

// NewAttributeSelectorHandler 创建新的AI属性选择处理器
func NewAttributeSelectorHandler(config *openaiClient.ClientConfig) *AttributeSelectorHandler {
	return &AttributeSelectorHandler{
		openaiClient:      openaiClient.NewClient(config),
		promptGenerator:   NewAttributePromptGenerator(),
		validator:         NewAttributeSelectionValidator(),
		importanceService: NewImportanceService(),
		utils:             NewAttributeUtils(),
	}
}

// Name 返回处理器名称
func (h *AttributeSelectorHandler) Name() string {
	return "AI属性选择"
}

// Handle 执行AI属性选择处理
func (h *AttributeSelectorHandler) Handle(ctx *shein.TaskContext) error {
	// 检查前置条件
	if err := h.validatePreconditions(ctx); err != nil {
		return err
	}

	// 构造缓存 key：同一产品+分类的属性选择结果可复用
	cacheKey := fmt.Sprintf("%s:%d", ctx.AmazonProduct.Asin, ctx.ProductData.CategoryID)

	// 查缓存
	if ctx.AICache != nil {
		var cached shein.AttributeData
		if ctx.AICache.Get(aicache.TypeAttribute, cacheKey, &cached) {
			logger.GetGlobalLogger("shein/product").Infof("AI属性选择命中缓存: asin=%s, categoryID=%d", ctx.AmazonProduct.Asin, ctx.ProductData.CategoryID)
			ctx.GenerateAttribute = &cached
			return nil
		}
	}

	attributeInfo, err := h.convertAttributeFromGpt(ctx, ctx.BuildAttributeData, ctx.AttributeTemplates)
	if err != nil {
		return shein.NewRetryableError("转换属性数据失败", err)
	}

	// 写缓存
	if ctx.AICache != nil {
		ctx.AICache.Set(aicache.TypeAttribute, cacheKey, attributeInfo)
	}

	ctx.GenerateAttribute = &attributeInfo
	return nil
}

// validatePreconditions 验证处理前置条件
func (h *AttributeSelectorHandler) validatePreconditions(ctx *shein.TaskContext) error {
	if ctx.ProductData == nil {
		// 这是一个程序逻辑错误，不应该发生，不可重试
		return shein.NewNonRetryableError("产品数据未获取，请先执行获取产品数据步骤", nil)
	}

	if len(ctx.BuildAttributeData.AttributeData) == 0 {
		// 这是一个程序逻辑错误，不应该发生，不可重试
		return shein.NewNonRetryableError("属性数据未构建，请先执行构建属性信息步骤", nil)
	}

	return nil
}

// convertAttributeFromGpt 使用GPT生成产品属性
func (h *AttributeSelectorHandler) convertAttributeFromGpt(ctx *shein.TaskContext, attributeInfo *shein.BuildAttributeInfo, attributeTemplates *attribute.AttributeTemplateInfo) (shein.AttributeData, error) {
	// 生成系统提示词
	systemPrompt, err := h.promptGenerator.GenerateSystemPrompt(attributeTemplates)
	if err != nil {
		logger.GetGlobalLogger("shein/product").Warnf("生成动态系统提示词失败，使用默认提示词: %v", err)
		systemPrompt = h.promptGenerator.GenerateDefaultSystemPrompt()
	}

	// 增强属性数据
	enhancedAttributeData := h.importanceService.EnhanceAttributeDataWithTemplateInfo(attributeInfo.AttributeData, attributeTemplates)

	// 生成用户提示词
	userPrompt := h.promptGenerator.GenerateUserPrompt(ctx, enhancedAttributeData)

	// 创建请求
	req := h.createChatCompletionRequest(systemPrompt, userPrompt)

	// 调用OpenAI API
	response, err := h.openaiClient.CreateChatCompletion(ctx.Context, req)
	if err != nil {
		// AI服务调用失败，可重试
		return shein.AttributeData{}, shein.NewRetryableError("生成产品属性失败", err)
	}

	if len(response.Choices) == 0 {
		// AI响应为空，可重试
		return shein.AttributeData{}, shein.NewRetryableError("AI响应为空", nil)
	}

	// 处理AI响应
	return h.processAIResponse(response, *attributeInfo, attributeTemplates)
}

// createChatCompletionRequest 创建聊天完成请求
func (h *AttributeSelectorHandler) createChatCompletionRequest(systemPrompt, userPrompt string) *openaiClient.ChatCompletionRequest {
	seed := 42
	temperature := float32(0.1)

	return &openaiClient.ChatCompletionRequest{
		Model: h.openaiClient.GetDefaultModel(),
		Messages: []openaiClient.ChatCompletionMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Temperature: &temperature,
		Seed:        &seed,
	}
}

// processAIResponse 处理AI响应
func (h *AttributeSelectorHandler) processAIResponse(response *openaiClient.ChatCompletionResponse, attributeInfo shein.BuildAttributeInfo, attributeTemplates *attribute.AttributeTemplateInfo) (shein.AttributeData, error) {
	content := response.Choices[0].Message.Content
	content = strings.TrimSpace(content)

	// 清理JSON格式
	content = h.utils.CleanJSONContent(content)

	// 验证JSON格式
	if !json.Valid([]byte(content)) {
		logger.GetGlobalLogger("shein/product").Errorf("AI返回的JSON格式无效，清理后内容: %s", content)
		logger.GetGlobalLogger("shein/product").Errorf("清理后内容长度: %d", len(content))

		// 尝试修复常见的JSON问题
		fixedContent := h.utils.FixCommonJSONIssues(content)
		logger.GetGlobalLogger("shein/product").Infof("修复后内容: %s", fixedContent)

		if !json.Valid([]byte(fixedContent)) {
			logger.GetGlobalLogger("shein/product").Errorf("修复后JSON仍然无效")
			var jsonErr error
			var temp any
			jsonErr = json.Unmarshal([]byte(fixedContent), &temp)
			// JSON格式无效且无法修复，可能是AI模型问题，可重试
			return shein.AttributeData{}, shein.NewRetryableError("AI返回的JSON格式无效且无法修复", jsonErr)
		}
		content = fixedContent
	}

	var attributeData shein.AttributeData
	if err := jsonx.UnmarshalBytes([]byte(content), &attributeData, "解析属性数据失败"); err != nil {
		logger.GetGlobalLogger("shein/product").Errorf("解析属性数据失败: %v，清理后内容: %s", err, content)
		// 解析属性数据失败，可重试
		return shein.AttributeData{}, shein.NewRetryableError("解析属性数据失败", err)
	}

	// 使用增强版验证并修复AI选择的属性值
	attributeData = h.validator.ValidateAndFixAttributeSelection(attributeData, attributeInfo, attributeTemplates)

	return attributeData, nil
}
