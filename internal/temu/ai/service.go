// Package ai 提供TEMU平台的各种处理器，包括AI服务调用等功能
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	models "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"
	"task-processor/internal/temu/property"

	"github.com/sirupsen/logrus"
)

// AIService AI服务调用器
type AIService struct {
	openaiClient openaiClient.ChatCompleter
	logger       *logrus.Entry
}

// NewAIService 创建新的AI服务调用器
func NewAIService(client openaiClient.ChatCompleter, logger *logrus.Entry) *AIService {
	return &AIService{
		openaiClient: client,
		logger:       logger,
	}
}

// CallAIForPropertyMapping 调用AI进行属性映射
// 参数:
//   - ctx: 上下文
//   - data: 属性映射数据
//
// 返回值:
//   - []common.PropertyItem: 映射后的属性列表
//   - error: 错误信息
func (s *AIService) CallAIForPropertyMapping(ctx context.Context, data temucontext.PropertyMappingData) ([]models.PropertyItem, error) {
	s.logger.Info("🤖 开始调用AI进行属性映射")

	// 检查AI客户端是否可用
	if s.openaiClient == nil {
		s.logger.Warn("OpenAI客户端未配置，返回空结果")
		return []models.PropertyItem{}, nil
	}

	// 构建提示词
	promptBuilder := NewPromptBuilder()
	systemPrompt := promptBuilder.BuildSystemPrompt()
	userPrompt := promptBuilder.BuildUserPrompt(data)

	s.logger.Debugf("系统提示词长度: %d 字符", len(systemPrompt))
	s.logger.Debugf("用户提示词长度: %d 字符", len(userPrompt))

	// 创建请求并调用API
	req := s.createChatCompletionRequest(systemPrompt, userPrompt)
	response, err := s.openaiClient.CreateChatCompletion(ctx, req)
	if err != nil {
		s.logger.WithError(err).Error("调用OpenAI API失败")
		return nil, fmt.Errorf("调用AI服务失败: %w", err)
	}

	if len(response.Choices) == 0 {
		s.logger.Error("AI响应为空")
		return nil, fmt.Errorf("AI响应为空")
	}

	// 处理AI响应
	return s.processAIResponse(response, data)
}

// createChatCompletionRequest 创建聊天完成请求
func (s *AIService) createChatCompletionRequest(systemPrompt, userPrompt string) *openaiClient.ChatCompletionRequest {
	seed := 42
	temperature := float32(0.1)

	return &openaiClient.ChatCompletionRequest{
		Model: s.openaiClient.GetDefaultModel(),
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
func (s *AIService) processAIResponse(response *openaiClient.ChatCompletionResponse, data temucontext.PropertyMappingData) ([]models.PropertyItem, error) {
	content := strings.TrimSpace(response.Choices[0].Message.Content)

	// 清理JSON格式
	content = jsonx.CleanLLMResponse(content)

	// 验证JSON格式
	if !json.Valid([]byte(content)) {
		s.logger.Errorf("AI返回的JSON格式无效: %s", content)
		return nil, fmt.Errorf("AI返回的JSON格式无效")
	}

	// 解析AI响应
	var aiResponse struct {
		Properties []models.PropertyItem `json:"properties"`
	}

	if err := jsonx.UnmarshalBytes([]byte(content), &aiResponse, "解析AI响应失败"); err != nil {
		s.logger.WithError(err).Errorf("解析AI响应失败: %s", content)
		return nil, fmt.Errorf("解析AI响应失败: %w", err)
	}

	// 使用属性验证器验证和修复属性值
	validator := property.NewPropertyValidator(s.logger)
	validatedProperties := validator.ValidateAndFixProperties(aiResponse.Properties, data)

	s.logger.Infof("AI属性映射成功，返回 %d 个属性", len(validatedProperties))
	return validatedProperties, nil
}
