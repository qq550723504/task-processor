// Package utils 提供工具方法
package utils

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Metrics 应用监控指标
type Metrics struct {
	mutex          sync.RWMutex
	requestCount   int64
	errorCount     int64
	totalLatency   time.Duration
	goroutineCount int
	lastUpdateTime time.Time
	logger         *logrus.Logger
}

// NewMetrics 创建监控指标实例
func NewMetrics(logger *logrus.Logger) *Metrics {
	return &Metrics{
		lastUpdateTime: time.Now(),
		logger:         logger,
	}
}

// IncrementRequests 增加请求计数
func (m *Metrics) IncrementRequests() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.requestCount++
}

// IncrementErrors 增加错误计数
func (m *Metrics) IncrementErrors() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.errorCount++
}

// RecordLatency 记录延迟
func (m *Metrics) RecordLatency(latency time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.totalLatency += latency
}

// UpdateGoroutineCount 更新协程数量
func (m *Metrics) UpdateGoroutineCount(count int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.goroutineCount = count
}

// GetMetrics 获取当前指标
func (m *Metrics) GetMetrics() MetricsSnapshot {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return MetricsSnapshot{
		RequestCount:   m.requestCount,
		ErrorCount:     m.errorCount,
		TotalLatency:   m.totalLatency,
		GoroutineCount: m.goroutineCount,
		Timestamp:      time.Now(),
	}
}

// LogMetrics 记录指标到日志
func (m *Metrics) LogMetrics() {
	metrics := m.GetMetrics()

	avgLatency := time.Duration(0)
	if metrics.RequestCount > 0 {
		avgLatency = metrics.TotalLatency / time.Duration(metrics.RequestCount)
	}

	errorRate := float64(0)
	if metrics.RequestCount > 0 {
		errorRate = float64(metrics.ErrorCount) / float64(metrics.RequestCount) * 100
	}

	m.logger.WithFields(logrus.Fields{
		"requests":       metrics.RequestCount,
		"errors":         metrics.ErrorCount,
		"error_rate":     errorRate,
		"avg_latency_ms": avgLatency.Milliseconds(),
		"goroutines":     metrics.GoroutineCount,
	}).Info("应用监控指标")
}

// MetricsSnapshot 指标快照
type MetricsSnapshot struct {
	RequestCount   int64
	ErrorCount     int64
	TotalLatency   time.Duration
	GoroutineCount int
	Timestamp      time.Time
}
