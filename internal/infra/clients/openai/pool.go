// Package openai 提供OpenAI API客户端功能
package openai

import (
	"context"
	"fmt"
	"sync"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

// RequestPool OpenAI请求池，负责并发控制、速率限制和负载均衡
type RequestPool struct {
	clients    []*BaseClient
	semaphore  chan struct{}
	rateLimit  *RateLimiter
	roundRobin int
	mutex      sync.Mutex
	logger     *logrus.Entry
}

// BaseClient 基础OpenAI客户端封装
type BaseClient struct {
	client *openai.Client
	config *ClientConfig
}

// RateLimiter 速率限制器
type RateLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mutex      sync.Mutex
}

// NewRequestPool 创建新的请求池
func NewRequestPool(config *PoolConfig) (*RequestPool, error) {
	if len(config.ClientConfigs) == 0 {
		return nil, fmt.Errorf("至少需要一个客户端配置")
	}

	// 创建多个客户端实例
	clients := make([]*BaseClient, len(config.ClientConfigs))
	for i, clientConfig := range config.ClientConfigs {
		clients[i] = newBaseClient(clientConfig)
	}

	// 创建速率限制器
	rateLimiter := &RateLimiter{
		tokens:     config.BurstLimit,
		maxTokens:  config.BurstLimit,
		refillRate: config.RateLimit,
		lastRefill: time.Now(),
	}

	return &RequestPool{
		clients:   clients,
		semaphore: make(chan struct{}, config.MaxConcurrent),
		rateLimit: rateLimiter,
		logger:    logrus.WithField("component", "OpenAIRequestPool"),
	}, nil
}

// newBaseClient 创建基础客户端
func newBaseClient(config *ClientConfig) *BaseClient {
	// 创建OpenAI客户端配置
	clientConfig := openai.DefaultConfig(config.APIKey)
	if config.BaseURL != "" {
		clientConfig.BaseURL = config.BaseURL
	}

	// 创建OpenAI客户端
	client := openai.NewClientWithConfig(clientConfig)

	return &BaseClient{
		client: client,
		config: config,
	}
}

// CreateChatCompletion 通过请求池创建聊天完成
func (p *RequestPool) CreateChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	// 1. 等待速率限制
	if err := p.waitForRateLimit(ctx); err != nil {
		return nil, fmt.Errorf("速率限制等待失败: %w", err)
	}

	// 2. 获取并发控制信号量
	select {
	case p.semaphore <- struct{}{}:
		defer func() { <-p.semaphore }()
	case <-ctx.Done():
		return nil, fmt.Errorf("等待并发槽位时上下文取消: %w", ctx.Err())
	}

	// 3. 选择客户端（轮询负载均衡）
	client := p.getNextClient()

	// 4. 执行请求
	startTime := time.Now()
	resp, err := client.createChatCompletion(ctx, req)
	duration := time.Since(startTime)

	// 5. 记录指标
	p.logMetrics(duration, err)

	return resp, err
}

// createChatCompletion 基础客户端的聊天完成实现
func (bc *BaseClient) createChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= bc.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// 计算指数退避延迟时间
			delay := bc.config.RetryDelay * time.Duration(1<<uint(attempt-1))
			logrus.Warnf("OpenAI API调用失败，第%d次重试，等待%v后重试: %v", attempt, delay, lastErr)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, fmt.Errorf("上下文已取消: %w", ctx.Err())
			}
		}

		// 设置超时
		timeoutCtx, cancel := context.WithTimeout(ctx, bc.config.Timeout)

		// 转换请求参数
		openaiReq := openai.ChatCompletionRequest{
			Model:    req.Model,
			Messages: convertMessages(req.Messages),
		}

		// 设置可选参数
		if req.Temperature != nil {
			openaiReq.Temperature = *req.Temperature
		}
		if req.Seed != nil {
			openaiReq.Seed = req.Seed
		}
		if req.MaxTokens != nil {
			openaiReq.MaxTokens = *req.MaxTokens
		}

		// 调用OpenAI API
		resp, err := bc.client.CreateChatCompletion(timeoutCtx, openaiReq)
		cancel() // 立即释放资源

		if err == nil {
			// 成功，返回响应
			if attempt > 0 {
				logrus.Infof("OpenAI API调用在第%d次重试后成功", attempt)
			}
			return convertResponse(&resp), nil
		}

		lastErr = err

		// 检查是否应该重试
		if !shouldRetry(err) {
			logrus.Warnf("OpenAI API调用失败，错误不可重试: %v", err)
			break
		}
	}

	return nil, fmt.Errorf("调用OpenAI API失败，已重试%d次: %w", bc.config.MaxRetries, lastErr)
}

// waitForRateLimit 等待速率限制
func (p *RequestPool) waitForRateLimit(ctx context.Context) error {
	p.rateLimit.mutex.Lock()
	defer p.rateLimit.mutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(p.rateLimit.lastRefill).Seconds()

	// 补充令牌
	p.rateLimit.tokens += elapsed * p.rateLimit.refillRate
	if p.rateLimit.tokens > p.rateLimit.maxTokens {
		p.rateLimit.tokens = p.rateLimit.maxTokens
	}
	p.rateLimit.lastRefill = now

	// 检查是否有可用令牌
	if p.rateLimit.tokens >= 1.0 {
		p.rateLimit.tokens -= 1.0
		return nil
	}

	// 计算等待时间
	waitTime := time.Duration((1.0-p.rateLimit.tokens)/p.rateLimit.refillRate) * time.Second
	p.logger.Debugf("速率限制等待: %v", waitTime)

	// 等待
	select {
	case <-time.After(waitTime):
		p.rateLimit.tokens = 0 // 消耗一个令牌
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// getNextClient 获取下一个客户端（轮询负载均衡）
func (p *RequestPool) getNextClient() *BaseClient {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	client := p.clients[p.roundRobin]
	p.roundRobin = (p.roundRobin + 1) % len(p.clients)
	return client
}

// logMetrics 记录请求指标
func (p *RequestPool) logMetrics(duration time.Duration, err error) {
	if err != nil {
		p.logger.WithFields(logrus.Fields{
			"duration": duration,
			"error":    err.Error(),
		}).Warn("OpenAI API请求失败")
	} else {
		p.logger.WithFields(logrus.Fields{
			"duration": duration,
		}).Debug("OpenAI API请求成功")
	}
}

// GetStats 获取请求池统计信息
func (p *RequestPool) GetStats() map[string]any {
	p.rateLimit.mutex.Lock()
	defer p.rateLimit.mutex.Unlock()

	return map[string]any{
		"available_tokens": p.rateLimit.tokens,
		"max_tokens":       p.rateLimit.maxTokens,
		"refill_rate":      p.rateLimit.refillRate,
		"concurrent_slots": cap(p.semaphore),
		"used_slots":       len(p.semaphore),
		"client_count":     len(p.clients),
	}
}

// Close 关闭请求池
func (p *RequestPool) Close() error {
	// 等待所有请求完成
	for i := 0; i < cap(p.semaphore); i++ {
		p.semaphore <- struct{}{}
	}

	p.logger.Info("OpenAI请求池已关闭")
	return nil
}

// convertMessages 转换消息格式
func convertMessages(messages []ChatCompletionMessage) []openai.ChatCompletionMessage {
	result := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		result[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	return result
}

// convertResponse 转换响应格式
func convertResponse(resp *openai.ChatCompletionResponse) *ChatCompletionResponse {
	choices := make([]ChatCompletionChoice, len(resp.Choices))
	for i, choice := range resp.Choices {
		choices[i] = ChatCompletionChoice{
			Index: choice.Index,
			Message: ChatCompletionMessage{
				Role:    choice.Message.Role,
				Content: choice.Message.Content,
			},
			FinishReason: string(choice.FinishReason),
		}
	}

	return &ChatCompletionResponse{
		ID:      resp.ID,
		Object:  resp.Object,
		Created: resp.Created,
		Model:   resp.Model,
		Choices: choices,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}

// shouldRetry 判断错误是否应该重试
func shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	// 这里可以根据具体错误类型判断是否重试
	// 简化处理，大部分错误都重试
	return true
}
