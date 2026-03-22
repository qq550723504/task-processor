package llm

import (
	"context"
	"fmt"
	"task-processor/internal/infra/clients/openai"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// OpenAILLMClient OpenAI LLM客户端适配器
type OpenAILLMClient struct {
	client *openai.Client
	model  string
	logger *logrus.Entry
}

// NewOpenAILLMClient 创建OpenAI LLM客户端
func NewOpenAILLMClient(client *openai.Client) *OpenAILLMClient {
	return &OpenAILLMClient{
		client: client,
		model:  client.GetDefaultModel(),
		logger: logger.GetGlobalLogger("OpenAILLMClient"),
	}
}

// Chat 实现LLMClient接口的Chat方法
func (c *OpenAILLMClient) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	c.logger.WithFields(logrus.Fields{
		"model":         c.model,
		"message_count": len(messages),
	}).Debug("发送OpenAI聊天请求")

	// 转换消息格式
	openaiMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// 构建请求
	req := &openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: openaiMessages,
	}

	// 设置参数优化LLM输出质量
	temperature := float32(0.1) // 低温度确保输出稳定
	maxTokens := 2000           // 足够的token数量
	req.Temperature = &temperature
	req.MaxTokens = &maxTokens

	// 调用OpenAI API
	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		c.logger.WithError(err).Error("OpenAI API调用失败")
		return nil, fmt.Errorf("OpenAI API调用失败: %w", err)
	}

	// 检查响应
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI响应中没有选择项")
	}

	choice := resp.Choices[0]
	content := choice.Message.Content

	c.logger.WithFields(logrus.Fields{
		"response_id":       resp.ID,
		"model":             resp.Model,
		"prompt_tokens":     resp.Usage.PromptTokens,
		"completion_tokens": resp.Usage.CompletionTokens,
		"total_tokens":      resp.Usage.TotalTokens,
		"finish_reason":     choice.FinishReason,
	}).Info("OpenAI API调用成功")

	// 转换响应格式
	return &ChatResponse{
		Content: content,
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

// GetModel 获取当前使用的模型
func (c *OpenAILLMClient) GetModel() string {
	return c.model
}

// SetModel 设置使用的模型
func (c *OpenAILLMClient) SetModel(model string) {
	c.model = model
	c.logger.WithField("new_model", model).Info("切换LLM模型")
}
