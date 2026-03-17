// Package worker 提供工作池监控指标
package worker

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics 工作池指标
type Metrics struct {
	// 任务统计
	totalSubmitted atomic.Int64 // 总提交数
	totalProcessed atomic.Int64 // 总处理数
	totalSucceeded atomic.Int64 // 成功数
	totalFailed    atomic.Int64 // 失败数
	totalPanicked  atomic.Int64 // Panic数

	// 性能统计
	processingTimes sync.Map // map[int64]time.Duration - 处理时间记录

	// 队列统计
	queueFullCount atomic.Int64 // 队列满次数

	// 时间统计
	startTime time.Time
}

// NewMetrics 创建指标收集器
func NewMetrics() *Metrics {
	return &Metrics{
		startTime: time.Now(),
	}
}

// RecordSubmit 记录任务提交
func (m *Metrics) RecordSubmit() {
	m.totalSubmitted.Add(1)
}

// RecordProcessStart 记录任务开始处理
func (m *Metrics) RecordProcessStart(taskID int64) {
	m.processingTimes.Store(taskID, time.Now())
}

// RecordProcessSuccess 记录任务处理成功
func (m *Metrics) RecordProcessSuccess(taskID int64) {
	m.totalProcessed.Add(1)
	m.totalSucceeded.Add(1)

	// 计算处理时间
	if startTime, ok := m.processingTimes.LoadAndDelete(taskID); ok {
		_ = time.Since(startTime.(time.Time))
		// 可以在这里记录到时间序列数据库
	}
}

// RecordProcessFailure 记录任务处理失败
func (m *Metrics) RecordProcessFailure(taskID int64) {
	m.totalProcessed.Add(1)
	m.totalFailed.Add(1)

	m.processingTimes.Delete(taskID)
}

// RecordPanic 记录Panic
func (m *Metrics) RecordPanic(taskID int64) {
	m.totalProcessed.Add(1)
	m.totalPanicked.Add(1)

	m.processingTimes.Delete(taskID)
}

// RecordQueueFull 记录队列满
func (m *Metrics) RecordQueueFull() {
	m.queueFullCount.Add(1)
}

// GetSnapshot 获取指标快照
func (m *Metrics) GetSnapshot() MetricsSnapshot {
	return MetricsSnapshot{
		TotalSubmitted: m.totalSubmitted.Load(),
		TotalProcessed: m.totalProcessed.Load(),
		TotalSucceeded: m.totalSucceeded.Load(),
		TotalFailed:    m.totalFailed.Load(),
		TotalPanicked:  m.totalPanicked.Load(),
		QueueFullCount: m.queueFullCount.Load(),
		Uptime:         time.Since(m.startTime),
	}
}

// MetricsSnapshot 指标快照
type MetricsSnapshot struct {
	TotalSubmitted int64         // 总提交数
	TotalProcessed int64         // 总处理数
	TotalSucceeded int64         // 成功数
	TotalFailed    int64         // 失败数
	TotalPanicked  int64         // Panic数
	QueueFullCount int64         // 队列满次数
	Uptime         time.Duration // 运行时间
}

// SuccessRate 计算成功率
func (s MetricsSnapshot) SuccessRate() float64 {
	if s.TotalProcessed == 0 {
		return 0
	}
	return float64(s.TotalSucceeded) / float64(s.TotalProcessed) * 100
}

// FailureRate 计算失败率
func (s MetricsSnapshot) FailureRate() float64 {
	if s.TotalProcessed == 0 {
		return 0
	}
	return float64(s.TotalFailed) / float64(s.TotalProcessed) * 100
}

// PanicRate 计算Panic率
func (s MetricsSnapshot) PanicRate() float64 {
	if s.TotalProcessed == 0 {
		return 0
	}
	return float64(s.TotalPanicked) / float64(s.TotalProcessed) * 100
}
