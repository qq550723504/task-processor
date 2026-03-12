// Package utils 提供熔断器工具
package utils

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrCircuitOpen 熔断器打开错误
	ErrCircuitOpen = errors.New("circuit breaker is open")
	// ErrTooManyRequests 请求过多错误
	ErrTooManyRequests = errors.New("too many requests")
)

// CircuitState 熔断器状态
type CircuitState int

const (
	// StateClosed 关闭状态（正常）
	StateClosed CircuitState = iota
	// StateOpen 打开状态（熔断）
	StateOpen
	// StateHalfOpen 半开状态（尝试恢复）
	StateHalfOpen
)

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	MaxRequests       uint32        // 半开状态下允许的最大请求数
	Interval          time.Duration // 统计时间窗口
	Timeout           time.Duration // 打开状态持续时间
	FailureThreshold  uint32        // 失败阈值（次数）
	FailureRateThresh float64       // 失败率阈值（0-1）
	MinRequests       uint32        // 最小请求数（用于计算失败率）
}

// DefaultCircuitBreakerConfig 默认熔断器配置
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		MaxRequests:       1,
		Interval:          60 * time.Second,
		Timeout:           30 * time.Second,
		FailureThreshold:  5,
		FailureRateThresh: 0.5,
		MinRequests:       10,
	}
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	config        *CircuitBreakerConfig
	state         CircuitState
	counts        *counts
	expiry        time.Time
	mu            sync.RWMutex
	onStateChange func(from, to CircuitState)
}

// counts 计数器
type counts struct {
	requests             uint32
	totalSuccesses       uint32
	totalFailures        uint32
	consecutiveSuccesses uint32
	consecutiveFailures  uint32
}

// NewCircuitBreaker 创建新的熔断器
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	cb := &CircuitBreaker{
		config: config,
		state:  StateClosed,
		counts: &counts{},
		expiry: time.Now().Add(config.Interval),
	}

	return cb
}

// Execute 执行函数（带熔断保护）
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	// 检查是否允许执行
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// 执行函数
	err := fn()

	// 记录结果
	cb.afterRequest(err == nil)

	return err
}

// beforeRequest 请求前检查
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	state := cb.state

	// 检查是否需要重置计数器
	if cb.expiry.Before(now) {
		cb.counts = &counts{}
		cb.expiry = now.Add(cb.config.Interval)
	}

	switch state {
	case StateClosed:
		// 关闭状态，允许请求
		cb.counts.requests++
		return nil

	case StateOpen:
		// 打开状态，检查是否可以转为半开
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen)
			cb.counts = &counts{}
			cb.expiry = now.Add(cb.config.Interval)
			cb.counts.requests++
			return nil
		}
		return ErrCircuitOpen

	case StateHalfOpen:
		// 半开状态，限制请求数
		if cb.counts.requests >= cb.config.MaxRequests {
			return ErrTooManyRequests
		}
		cb.counts.requests++
		return nil

	default:
		return fmt.Errorf("unknown circuit breaker state: %d", state)
	}
}

// afterRequest 请求后处理
func (cb *CircuitBreaker) afterRequest(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if success {
		cb.onSuccess()
	} else {
		cb.onFailure()
	}
}

// onSuccess 成功处理
func (cb *CircuitBreaker) onSuccess() {
	cb.counts.totalSuccesses++
	cb.counts.consecutiveSuccesses++
	cb.counts.consecutiveFailures = 0

	// 半开状态下，如果连续成功，转为关闭状态
	if cb.state == StateHalfOpen && cb.counts.consecutiveSuccesses >= cb.config.MaxRequests {
		cb.setState(StateClosed)
		cb.counts = &counts{}
	}
}

// onFailure 失败处理
func (cb *CircuitBreaker) onFailure() {
	cb.counts.totalFailures++
	cb.counts.consecutiveFailures++
	cb.counts.consecutiveSuccesses = 0

	// 检查是否需要打开熔断器
	if cb.shouldOpen() {
		cb.setState(StateOpen)
		cb.expiry = time.Now().Add(cb.config.Timeout)
	}
}

// shouldOpen 判断是否应该打开熔断器
func (cb *CircuitBreaker) shouldOpen() bool {
	// 连续失败次数超过阈值
	if cb.counts.consecutiveFailures >= cb.config.FailureThreshold {
		return true
	}

	// 失败率超过阈值（需要最小请求数）
	if cb.counts.requests >= cb.config.MinRequests {
		failureRate := float64(cb.counts.totalFailures) / float64(cb.counts.requests)
		if failureRate >= cb.config.FailureRateThresh {
			return true
		}
	}

	return false
}

// setState 设置状态
func (cb *CircuitBreaker) setState(state CircuitState) {
	if cb.state == state {
		return
	}

	oldState := cb.state
	cb.state = state

	// 触发状态变更回调
	if cb.onStateChange != nil {
		cb.onStateChange(oldState, state)
	}
}

// State 获取当前状态
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Counts 获取计数器
func (cb *CircuitBreaker) Counts() (requests, successes, failures uint32) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.counts.requests, cb.counts.totalSuccesses, cb.counts.totalFailures
}

// OnStateChange 设置状态变更回调
func (cb *CircuitBreaker) OnStateChange(fn func(from, to CircuitState)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = fn
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.counts = &counts{}
	cb.expiry = time.Now().Add(cb.config.Interval)
}
