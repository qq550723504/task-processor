// Package metrics 提供任务流转指标统计
package metrics

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// TaskMetrics 任务流转指标统计
type TaskMetrics struct {
	mu sync.RWMutex

	PendingCount    int64
	ProcessingCount int64
	CompletedCount  int64
	FailedCount     int64
	RequeuedCount   int64

	HighPriorityCount   int64
	MediumPriorityCount int64
	LowPriorityCount    int64

	TotalWaitTime    time.Duration
	TotalProcessTime time.Duration
	TaskCount        int64

	HeartbeatTimeoutCount int64
	TaskLossCount         int64
	RequeueFailureCount   int64
	MarkFailedErrorCount  int64

	LastUpdateTime time.Time
}

// TaskMetricsSnapshot 指标快照（不含锁）
type TaskMetricsSnapshot struct {
	PendingCount          int64
	ProcessingCount       int64
	CompletedCount        int64
	FailedCount           int64
	RequeuedCount         int64
	HighPriorityCount     int64
	MediumPriorityCount   int64
	LowPriorityCount      int64
	TotalWaitTime         time.Duration
	TotalProcessTime      time.Duration
	TaskCount             int64
	HeartbeatTimeoutCount int64
	TaskLossCount         int64
	RequeueFailureCount   int64
	MarkFailedErrorCount  int64
	LastUpdateTime        time.Time
}

var globalTaskMetrics = &TaskMetrics{LastUpdateTime: time.Now()}

// GlobalTaskMetrics 获取全局任务指标实例
func GlobalTaskMetrics() *TaskMetrics {
	return globalTaskMetrics
}

func (m *TaskMetrics) IncrementPending() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PendingCount++
	m.LastUpdateTime = time.Now()
}

func (m *TaskMetrics) IncrementProcessing() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProcessingCount++
	m.LastUpdateTime = time.Now()
}

func (m *TaskMetrics) IncrementCompleted() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CompletedCount++
	m.LastUpdateTime = time.Now()
}

func (m *TaskMetrics) IncrementFailed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FailedCount++
	m.LastUpdateTime = time.Now()
}

func (m *TaskMetrics) IncrementRequeued() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RequeuedCount++
	m.LastUpdateTime = time.Now()
}

func (m *TaskMetrics) RecordPriority(priority int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case priority >= 80:
		m.HighPriorityCount++
	case priority >= 50:
		m.MediumPriorityCount++
	default:
		m.LowPriorityCount++
	}
	m.LastUpdateTime = time.Now()
}

func (m *TaskMetrics) RecordWaitTime(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalWaitTime += d
	m.TaskCount++
	m.LastUpdateTime = time.Now()
}

func (m *TaskMetrics) RecordProcessTime(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalProcessTime += d
	m.LastUpdateTime = time.Now()
}

func (m *TaskMetrics) IncrementHeartbeatTimeout() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.HeartbeatTimeoutCount++
	m.LastUpdateTime = time.Now()
}

func (m *TaskMetrics) IncrementTaskLoss() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TaskLossCount++
	m.LastUpdateTime = time.Now()
}

func (m *TaskMetrics) IncrementRequeueFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RequeueFailureCount++
	m.LastUpdateTime = time.Now()
}

func (m *TaskMetrics) IncrementMarkFailedError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MarkFailedErrorCount++
	m.LastUpdateTime = time.Now()
}

func (m *TaskMetrics) GetSnapshot() TaskMetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return TaskMetricsSnapshot{
		PendingCount: m.PendingCount, ProcessingCount: m.ProcessingCount,
		CompletedCount: m.CompletedCount, FailedCount: m.FailedCount,
		RequeuedCount: m.RequeuedCount, HighPriorityCount: m.HighPriorityCount,
		MediumPriorityCount: m.MediumPriorityCount, LowPriorityCount: m.LowPriorityCount,
		TotalWaitTime: m.TotalWaitTime, TotalProcessTime: m.TotalProcessTime,
		TaskCount: m.TaskCount, HeartbeatTimeoutCount: m.HeartbeatTimeoutCount,
		TaskLossCount: m.TaskLossCount, RequeueFailureCount: m.RequeueFailureCount,
		MarkFailedErrorCount: m.MarkFailedErrorCount, LastUpdateTime: m.LastUpdateTime,
	}
}

func (m *TaskMetrics) GetAverageWaitTime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.TaskCount == 0 {
		return 0
	}
	return m.TotalWaitTime / time.Duration(m.TaskCount)
}

func (m *TaskMetrics) GetAverageProcessTime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.CompletedCount == 0 {
		return 0
	}
	return m.TotalProcessTime / time.Duration(m.CompletedCount)
}

func (m *TaskMetrics) LogSummary() {
	s := m.GetSnapshot()
	logrus.Infof("📊 [Metrics] 任务流转 - Pending:%d Processing:%d Completed:%d Failed:%d Requeued:%d",
		s.PendingCount, s.ProcessingCount, s.CompletedCount, s.FailedCount, s.RequeuedCount)
	logrus.Infof("📊 [Metrics] 优先级 - High:%d Medium:%d Low:%d",
		s.HighPriorityCount, s.MediumPriorityCount, s.LowPriorityCount)
	if s.HeartbeatTimeoutCount > 0 || s.TaskLossCount > 0 || s.RequeueFailureCount > 0 || s.MarkFailedErrorCount > 0 {
		logrus.Warnf("⚠️ [Metrics] 异常 - 心跳超时:%d 任务丢失:%d 重入队失败:%d 标记失败:%d",
			s.HeartbeatTimeoutCount, s.TaskLossCount, s.RequeueFailureCount, s.MarkFailedErrorCount)
	}
}

func (m *TaskMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	*m = TaskMetrics{LastUpdateTime: time.Now()}
}
