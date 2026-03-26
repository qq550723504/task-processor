package attribute

import (
	"encoding/json"
	"strings"

	"task-processor/internal/core/logger"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/shein/aicache"
	apiattribute "task-processor/internal/shein/api/attribute"
	sheinctx "task-processor/internal/shein/context"
	"task-processor/internal/shein/sherr"
)

// AttributeSelectorHandler handles AI-driven attribute selection.
type AttributeSelectorHandler struct {
	openaiClient      openaiClient.ChatCompleter
	promptGenerator   *AttributePromptGenerator
	validator         *AttributeSelectionValidator
	importanceService *ImportanceService
	utils             *AttributeUtils
}

func NewAttributeSelectorHandler(client openaiClient.ChatCompleter) *AttributeSelectorHandler {
	return &AttributeSelectorHandler{
		openaiClient:      client,
		promptGenerator:   NewAttributePromptGenerator(),
		validator:         NewAttributeSelectionValidator(),
		importanceService: NewImportanceService(),
		utils:             NewAttributeUtils(),
	}
}

func (h *AttributeSelectorHandler) Name() string {
	return "AI属性选择"
}

func (h *AttributeSelectorHandler) Handle(ctx *sheinctx.TaskContext) error {
	input, err := buildAttributeSelectionInput(ctx, h.openaiClient)
	if err != nil {
		return sherr.NewNonRetryableError("构建属性选择输入失败", err)
	}

	if input.AICache != nil {
		var cached AttributeData
		if input.AICache.Get(aicache.TypeAttribute, input.CacheKey, &cached) {
			logger.GetGlobalLogger("shein/product").Infof("AI属性选择命中缓存: key=%s", input.CacheKey)
			ctx.GenerateAttribute = &cached
			return nil
		}
	}

	attributeInfo, err := h.convertAttributeFromGpt(input, ctx)
	if err != nil {
		return sherr.NewRetryableError("转换属性数据失败", err)
	}

	if input.AICache != nil {
		input.AICache.Set(aicache.TypeAttribute, input.CacheKey, attributeInfo)
	}

	ctx.GenerateAttribute = &attributeInfo
	return nil
}

func (h *AttributeSelectorHandler) convertAttributeFromGpt(input *AttributeSelectionInput, ctx *sheinctx.TaskContext) (AttributeData, error) {
	systemPrompt, err := h.promptGenerator.GenerateSystemPrompt(input.AttributeTemplates)
	if err != nil {
		logger.GetGlobalLogger("shein/product").Warnf("生成动态系统提示词失败，使用默认提示词: %v", err)
		systemPrompt = h.promptGenerator.GenerateDefaultSystemPrompt()
	}

	enhancedAttributeData := h.importanceService.EnhanceAttributeDataWithTemplateInfo(
		input.BuildAttributeData.AttributeData,
		input.AttributeTemplates,
	)
	userPrompt := h.promptGenerator.GenerateUserPrompt(ctx, enhancedAttributeData)
	req := h.createChatCompletionRequest(systemPrompt, userPrompt)

	response, err := input.OpenAIClient.CreateChatCompletion(input.Context, req)
	if err != nil {
		return AttributeData{}, sherr.NewRetryableError("生成产品属性失败", err)
	}
	if len(response.Choices) == 0 {
		return AttributeData{}, sherr.NewRetryableError("AI响应为空", nil)
	}

	return h.processAIResponse(response, *input.BuildAttributeData, input.AttributeTemplates)
}

func (h *AttributeSelectorHandler) createChatCompletionRequest(systemPrompt, userPrompt string) *openaiClient.ChatCompletionRequest {
	seed := 42
	temperature := float32(0.1)

	return &openaiClient.ChatCompletionRequest{
		Model: h.openaiClient.GetDefaultModel(),
		Messages: []openaiClient.ChatCompletionMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: &temperature,
		Seed:        &seed,
	}
}

func (h *AttributeSelectorHandler) processAIResponse(response *openaiClient.ChatCompletionResponse, attributeInfo BuildAttributeInfo, attributeTemplates *apiattribute.AttributeTemplateInfo) (AttributeData, error) {
	content := strings.TrimSpace(response.Choices[0].Message.Content)
	content = h.utils.CleanJSONContent(content)

	if !json.Valid([]byte(content)) {
		logger.GetGlobalLogger("shein/product").Errorf("AI返回的JSON格式无效，清理后内容: %s", content)
		fixedContent := h.utils.FixCommonJSONIssues(content)
		logger.GetGlobalLogger("shein/product").Infof("修复后内容: %s", fixedContent)
		if !json.Valid([]byte(fixedContent)) {
			var temp any
			jsonErr := json.Unmarshal([]byte(fixedContent), &temp)
			return AttributeData{}, sherr.NewRetryableError("AI返回的JSON格式无效且无法修复", jsonErr)
		}
		content = fixedContent
	}

	var attributeData AttributeData
	if err := jsonx.UnmarshalBytes([]byte(content), &attributeData, "解析属性数据失败"); err != nil {
		logger.GetGlobalLogger("shein/product").Errorf("解析属性数据失败: %v，清理后内容: %s", err, content)
		return AttributeData{}, sherr.NewRetryableError("解析属性数据失败", err)
	}

	attributeData = h.validator.ValidateAndFixAttributeSelection(attributeData, attributeInfo, attributeTemplates)
	return attributeData, nil
}
