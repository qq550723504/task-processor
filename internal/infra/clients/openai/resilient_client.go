// Package openai 提供带弹性机制的OpenAI客户端
package openai

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// ResilientClient 带弹性机制的OpenAI客户端装饰器
type ResilientClient struct {
	client         *Client
	logger         *logrus.Entry
	circuitBreaker *CircuitBreaker
	clientName     string
}

// ResilientClientConfig 弹性客户端配置
type ResilientClientConfig struct {
	Client               *Client
	ClientName           string
	CircuitBreakerConfig *CircuitBreakerConfig
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	Interval         time.Duration
	Timeout          time.Duration
	MaxRequests      uint32
	FailureThreshold uint32
}

// CircuitState 熔断器状态
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	config        *CircuitBreakerConfig
	state         atomic.Value
	counts        *Counts
	expiry        atomic.Value
	mu            sync.Mutex
	onStateChange func(from, to CircuitState)
}

// Counts 计数器
type Counts struct {
	requests  atomic.Uint32
	successes atomic.Uint32
	failures  atomic.Uint32
}

// DefaultCircuitBreakerConfig 默认熔断器配置
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		MaxRequests:      10,
		Interval:         60 * time.Second,
		Timeout:          30 * time.Second,
		FailureThreshold: 5,
	}
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}
	cb := &CircuitBreaker{config: config, counts: &Counts{}}
	cb.state.Store(StateClosed)
	cb.expiry.Store(time.Now().Add(config.Interval))
	return cb
}

// Execute 执行函数(带熔断保护)
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if err := cb.beforeRequest(); err != nil {
		return err
	}
	err := fn()
	cb.afterRequest(err == nil)
	return err
}

// beforeRequest 请求前检查
func (cb *CircuitBreaker) beforeRequest() error {
	state := cb.State()
	if state == StateOpen {
		if time.Now().After(cb.expiry.Load().(time.Time)) {
			cb.setState(StateHalfOpen)
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	}
	if state == StateHalfOpen {
		if cb.counts.requests.Load() >= cb.config.MaxRequests {
			return fmt.Errorf("circuit breaker is half-open, too many requests")
		}
	}
	cb.counts.requests.Add(1)
	return nil
}

// afterRequest 请求后处理
func (cb *CircuitBreaker) afterRequest(success bool) {
	if success {
		cb.onSuccess()
	} else {
		cb.onFailure()
	}
}

// onSuccess 成功处理
func (cb *CircuitBreaker) onSuccess() {
	cb.counts.successes.Add(1)
	state := cb.State()
	if state == StateHalfOpen && cb.counts.successes.Load() >= cb.config.MaxRequests {
		cb.setState(StateClosed)
	}
}

// onFailure 失败处理
func (cb *CircuitBreaker) onFailure() {
	cb.counts.failures.Add(1)
	state := cb.State()
	failures := cb.counts.failures.Load()
	if state == StateClosed && failures >= cb.config.FailureThreshold {
		cb.setState(StateOpen)
	} else if state == StateHalfOpen {
		cb.setState(StateOpen)
	}
}

// setState 设置状态
func (cb *CircuitBreaker) setState(newState CircuitState) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	oldState := cb.State()
	if oldState == newState {
		return
	}
	cb.state.Store(newState)
	cb.counts = &Counts{}
	now := time.Now()
	switch newState {
	case StateClosed:
		cb.expiry.Store(now.Add(cb.config.Interval))
	case StateOpen:
		cb.expiry.Store(now.Add(cb.config.Timeout))
	case StateHalfOpen:
		cb.expiry.Store(now.Add(cb.config.Interval))
	}
	if cb.onStateChange != nil {
		cb.onStateChange(oldState, newState)
	}
}

// State 获取当前状态
func (cb *CircuitBreaker) State() CircuitState {
	return cb.state.Load().(CircuitState)
}

// Counts 获取计数
func (cb *CircuitBreaker) Counts() (requests, successes, failures uint32) {
	return cb.counts.requests.Load(), cb.counts.successes.Load(), cb.counts.failures.Load()
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.setState(StateClosed)
}

// OnStateChange 设置状态变更回调
func (cb *CircuitBreaker) OnStateChange(fn func(from, to CircuitState)) {
	cb.onStateChange = fn
}

// NewResilientClient 创建带弹性机制的OpenAI客户端
func NewResilientClient(config *ResilientClientConfig) (*ResilientClient, error) {
	if config == nil || config.Client == nil {
		return nil, fmt.Errorf("invalid config")
	}
	if config.ClientName == "" {
		config.ClientName = "default"
	}
	if config.CircuitBreakerConfig == nil {
		config.CircuitBreakerConfig = DefaultCircuitBreakerConfig()
	}
	cb := NewCircuitBreaker(config.CircuitBreakerConfig)
	client := &ResilientClient{
		client:         config.Client,
		logger:         logger.GetGlobalLogger("ResilientOpenAIClient"),
		circuitBreaker: cb,
		clientName:     config.ClientName,
	}
	cb.OnStateChange(func(from, to CircuitState) {
		client.logger.Warnf("熔断器状态变更: %s -> %s", stateString(from), stateString(to))
	})
	return client, nil
}

// CreateChatCompletion 创建聊天完成(带熔断保护)
// 注意: 重试逻辑由底层BaseClient处理,这里只提供熔断保护
func (r *ResilientClient) CreateChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	startTime := time.Now()
	var resp *ChatCompletionResponse

	// 使用熔断器保护,不额外重试(避免与BaseClient的重试重复)
	err := r.circuitBreaker.Execute(ctx, func() error {
		var execErr error
		resp, execErr = r.client.CreateChatCompletion(ctx, req)
		return execErr
	})

	duration := time.Since(startTime)
	if err != nil {
		r.logger.WithFields(logrus.Fields{
			"client":      r.clientName,
			"duration_ms": duration.Milliseconds(),
		}).Error("OpenAI API调用失败")
		return nil, err
	}

	r.logger.WithFields(logrus.Fields{
		"client":      r.clientName,
		"duration_ms": duration.Milliseconds(),
	}).Debug("OpenAI API调用成功")
	return resp, nil
}

// Generate 简单文本生成（委托给内部客户端，带熔断保护）
func (r *ResilientClient) Generate(ctx context.Context, prompt string) (string, error) {
	var result string
	err := r.circuitBreaker.Execute(ctx, func() error {
		var execErr error
		result, execErr = r.client.Generate(ctx, prompt)
		return execErr
	})
	return result, err
}

// AnalyzeImage 图片分析（委托给内部客户端，带熔断保护）
func (r *ResilientClient) AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error) {
	var result string
	err := r.circuitBreaker.Execute(ctx, func() error {
		var execErr error
		result, execErr = r.client.AnalyzeImage(ctx, imageURL, prompt)
		return execErr
	})
	return result, err
}

// GetDefaultModel 获取默认模型
func (r *ResilientClient) GetDefaultModel() string {
	return r.client.GetDefaultModel()
}

func (r *ResilientClient) SupportsAsyncImageGeneration() bool {
	return r.client.SupportsAsyncImageGeneration()
}

func (r *ResilientClient) SubmitImageGeneration(ctx context.Context, req *ImageGenerateRequest) (*ImageAsyncSubmitResponse, error) {
	var result *ImageAsyncSubmitResponse
	err := r.circuitBreaker.Execute(ctx, func() error {
		var execErr error
		result, execErr = r.client.SubmitImageGeneration(ctx, req)
		return execErr
	})
	return result, err
}

func (r *ResilientClient) QueryImageGeneration(ctx context.Context, jobID string) (*ImageAsyncQueryResponse, error) {
	var result *ImageAsyncQueryResponse
	err := r.circuitBreaker.Execute(ctx, func() error {
		var execErr error
		result, execErr = r.client.QueryImageGeneration(ctx, jobID)
		return execErr
	})
	return result, err
}

// GetCircuitBreakerState 获取熔断器状态
func (r *ResilientClient) GetCircuitBreakerState() CircuitState {
	return r.circuitBreaker.State()
}

// GetCircuitBreakerCounts 获取熔断器计数
func (r *ResilientClient) GetCircuitBreakerCounts() (requests, successes, failures uint32) {
	return r.circuitBreaker.Counts()
}

// ResetCircuitBreaker 重置熔断器
func (r *ResilientClient) ResetCircuitBreaker() {
	r.circuitBreaker.Reset()
	r.logger.Infof("熔断器已重置")
}

// GetStats 获取客户端统计信息
func (r *ResilientClient) GetStats() map[string]any {
	stats := r.client.GetStats()
	state := r.circuitBreaker.State()
	requests, successes, failures := r.circuitBreaker.Counts()
	stats["circuit_breaker"] = map[string]any{
		"state": stateString(state), "requests": requests,
		"successes": successes, "failures": failures,
	}
	return stats
}

// Close 关闭客户端
func (r *ResilientClient) Close() error {
	return r.client.Close()
}

// stateString 状态转字符串
func stateString(state CircuitState) string {
	switch state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}
