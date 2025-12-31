package openai

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// EnhancedClient 增强的OpenAI客户端，集成请求池和上下文管理
type EnhancedClient struct {
	pool           *RequestPool
	contextManager *ContextManager
	metrics        *Metrics
	logger         *logrus.Entry
}

// Metrics 请求指标
type Metrics struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	TimeoutRequests int64
	AverageLatency  time.Duration
	MaxLatency      time.Duration
	MinLatency      time.Duration
	mutex           sync.RWMutex
}

// EnhancedClientConfig 增强客户端配置
type EnhancedClientConfig struct {
	Pool    *PoolConfig
	Context *ContextConfig
}

// NewEnhancedClient 创建增强的OpenAI客户端
func NewEnhancedClient(config *EnhancedClientConfig) (*EnhancedClient, error) {
	// 创建请求池
	pool, err := NewRequestPool(config.Pool)
	if err != nil {
		return nil, fmt.Errorf("创建请求池失败: %w", err)
	}

	// 创建上下文管理器
	contextManager := NewContextManager(config.Context)

	// 创建指标收集器
	metrics := &Metrics{
		MinLatency: time.Hour, // 初始化为一个大值
	}

	return &EnhancedClient{
		pool:           pool,
		contextManager: contextManager,
		metrics:        metrics,
		logger:         logrus.WithField("component", "EnhancedOpenAIClient"),
	}, nil
}

// CreateChatCompletion 创建聊天完成（增强版本）
func (ec *EnhancedClient) CreateChatCompletion(ctx context.Context, req *ChatCompletionRequest, taskType string) (*ChatCompletionResponse, error) {
	// 1. 创建带超时的上下文
	apiCtx, contextID, err := ec.contextManager.CreateContext(ctx, taskType, 0) // 使用默认超时
	if err != nil {
		return nil, fmt.Errorf("创建API上下文失败: %w", err)
	}
	defer ec.contextManager.ReleaseContext(contextID)

	// 2. 记录请求开始
	startTime := time.Now()
	ec.updateMetrics(func(m *Metrics) {
		m.TotalRequests++
	})

	// 3. 执行请求
	resp, err := ec.pool.CreateChatCompletion(apiCtx, req)
	duration := time.Since(startTime)

	// 4. 更新指标
	ec.updateRequestMetrics(duration, err)

	// 5. 记录日志
	ec.logRequest(taskType, contextID, duration, err)

	return resp, err
}

// CreateChatCompletionWithTimeout 创建带自定义超时的聊天完成
func (ec *EnhancedClient) CreateChatCompletionWithTimeout(ctx context.Context, req *ChatCompletionRequest, taskType string, timeout time.Duration) (*ChatCompletionResponse, error) {
	// 1. 创建带自定义超时的上下文
	apiCtx, contextID, err := ec.contextManager.CreateContext(ctx, taskType, timeout)
	if err != nil {
		return nil, fmt.Errorf("创建API上下文失败: %w", err)
	}
	defer ec.contextManager.ReleaseContext(contextID)

	// 2. 记录请求开始
	startTime := time.Now()
	ec.updateMetrics(func(m *Metrics) {
		m.TotalRequests++
	})

	// 3. 执行请求
	resp, err := ec.pool.CreateChatCompletion(apiCtx, req)
	duration := time.Since(startTime)

	// 4. 更新指标
	ec.updateRequestMetrics(duration, err)

	// 5. 记录日志
	ec.logRequest(taskType, contextID, duration, err)

	return resp, err
}

// updateMetrics 更新指标（线程安全）
func (ec *EnhancedClient) updateMetrics(updateFunc func(*Metrics)) {
	ec.metrics.mutex.Lock()
	defer ec.metrics.mutex.Unlock()
	updateFunc(ec.metrics)
}

// updateRequestMetrics 更新请求指标
func (ec *EnhancedClient) updateRequestMetrics(duration time.Duration, err error) {
	ec.updateMetrics(func(m *Metrics) {
		if err != nil {
			m.FailedRequests++
			// 检查是否是超时错误
			if isTimeoutError(err) {
				m.TimeoutRequests++
			}
		} else {
			m.SuccessRequests++
		}

		// 更新延迟统计
		if duration > m.MaxLatency {
			m.MaxLatency = duration
		}
		if duration < m.MinLatency {
			m.MinLatency = duration
		}

		// 计算平均延迟（简化版本）
		totalRequests := m.SuccessRequests + m.FailedRequests
		if totalRequests > 0 {
			m.AverageLatency = time.Duration(
				(int64(m.AverageLatency)*int64(totalRequests-1) + int64(duration)) / int64(totalRequests),
			)
		}
	})
}

// logRequest 记录请求日志
func (ec *EnhancedClient) logRequest(taskType, contextID string, duration time.Duration, err error) {
	fields := logrus.Fields{
		"task_type":  taskType,
		"context_id": contextID,
		"duration":   duration,
	}

	if err != nil {
		fields["error"] = err.Error()
		ec.logger.WithFields(fields).Error("OpenAI API请求失败")
	} else {
		ec.logger.WithFields(fields).Debug("OpenAI API请求成功")
	}
}

// isTimeoutError 检查是否是超时错误
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	timeoutKeywords := []string{
		"timeout",
		"deadline exceeded",
		"context canceled",
		"context deadline exceeded",
	}

	for _, keyword := range timeoutKeywords {
		if contains(errStr, keyword) {
			return true
		}
	}
	return false
}

// contains 检查字符串是否包含子字符串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsInMiddle(s, substr))))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetMetrics 获取请求指标
func (ec *EnhancedClient) GetMetrics() map[string]interface{} {
	ec.metrics.mutex.RLock()
	defer ec.metrics.mutex.RUnlock()

	successRate := float64(0)
	if ec.metrics.TotalRequests > 0 {
		successRate = float64(ec.metrics.SuccessRequests) / float64(ec.metrics.TotalRequests) * 100
	}

	return map[string]interface{}{
		"total_requests":   ec.metrics.TotalRequests,
		"success_requests": ec.metrics.SuccessRequests,
		"failed_requests":  ec.metrics.FailedRequests,
		"timeout_requests": ec.metrics.TimeoutRequests,
		"success_rate":     successRate,
		"average_latency":  ec.metrics.AverageLatency,
		"max_latency":      ec.metrics.MaxLatency,
		"min_latency":      ec.metrics.MinLatency,
	}
}

// GetPoolStats 获取请求池统计信息
func (ec *EnhancedClient) GetPoolStats() map[string]interface{} {
	return ec.pool.GetStats()
}

// GetContextStats 获取上下文管理器统计信息
func (ec *EnhancedClient) GetContextStats() map[string]interface{} {
	return ec.contextManager.GetStats()
}

// GetAllStats 获取所有统计信息
func (ec *EnhancedClient) GetAllStats() map[string]interface{} {
	return map[string]interface{}{
		"metrics": ec.GetMetrics(),
		"pool":    ec.GetPoolStats(),
		"context": ec.GetContextStats(),
	}
}

// CancelLongRunningRequests 取消长时间运行的请求
func (ec *EnhancedClient) CancelLongRunningRequests(maxDuration time.Duration) int {
	return ec.contextManager.CancelLongRunningContexts(maxDuration)
}

// Close 关闭增强客户端
func (ec *EnhancedClient) Close() error {
	ec.logger.Info("正在关闭增强OpenAI客户端...")

	// 清理上下文管理器
	ec.contextManager.Cleanup()

	// 关闭请求池
	if err := ec.pool.Close(); err != nil {
		return fmt.Errorf("关闭请求池失败: %w", err)
	}

	ec.logger.Info("增强OpenAI客户端已关闭")
	return nil
}
