package common

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// TaskMetrics 任务指标统计
type TaskMetrics struct {
	mu sync.RWMutex

	// 任务流转统计
	PendingCount    int64
	ProcessingCount int64
	CompletedCount  int64
	FailedCount     int64
	RequeuedCount   int64

	// 优先级分布统计
	HighPriorityCount   int64 // Priority >= 80
	MediumPriorityCount int64 // 50 <= Priority < 80
	LowPriorityCount    int64 // Priority < 50

	// 时间统计
	TotalWaitTime    time.Duration
	TotalProcessTime time.Duration
	TaskCount        int64

	// 异常统计
	HeartbeatTimeoutCount int64
	TaskLossCount         int64
	RequeueFailureCount   int64
	MarkFailedErrorCount  int64

	// 最后更新时间
	LastUpdateTime time.Time
}

var (
	globalMetrics = &TaskMetrics{
		LastUpdateTime: time.Now(),
	}
)

// GetGlobalMetrics 获取全局指标实例
func GetGlobalMetrics() *TaskMetrics {
	return globalMetrics
}

// IncrementPending 增加待处理任务计数
func (m *TaskMetrics) IncrementPending() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PendingCount++
	m.LastUpdateTime = time.Now()
}

// IncrementProcessing 增加处理中任务计数
func (m *TaskMetrics) IncrementProcessing() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProcessingCount++
	m.LastUpdateTime = time.Now()
}

// IncrementCompleted 增加完成任务计数
func (m *TaskMetrics) IncrementCompleted() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CompletedCount++
	m.LastUpdateTime = time.Now()
}

// IncrementFailed 增加失败任务计数
func (m *TaskMetrics) IncrementFailed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FailedCount++
	m.LastUpdateTime = time.Now()
}

// IncrementRequeued 增加重新入队任务计数
func (m *TaskMetrics) IncrementRequeued() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RequeuedCount++
	m.LastUpdateTime = time.Now()
}

// RecordPriority 记录任务优先级
func (m *TaskMetrics) RecordPriority(priority int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if priority >= 80 {
		m.HighPriorityCount++
	} else if priority >= 50 {
		m.MediumPriorityCount++
	} else {
		m.LowPriorityCount++
	}
	m.LastUpdateTime = time.Now()
}

// RecordWaitTime 记录任务等待时间
func (m *TaskMetrics) RecordWaitTime(waitTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalWaitTime += waitTime
	m.TaskCount++
	m.LastUpdateTime = time.Now()
}

// RecordProcessTime 记录任务处理时间
func (m *TaskMetrics) RecordProcessTime(processTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalProcessTime += processTime
	m.LastUpdateTime = time.Now()
}

// IncrementHeartbeatTimeout 增加心跳超时计数
func (m *TaskMetrics) IncrementHeartbeatTimeout() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.HeartbeatTimeoutCount++
	m.LastUpdateTime = time.Now()
}

// IncrementTaskLoss 增加任务丢失计数
func (m *TaskMetrics) IncrementTaskLoss() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TaskLossCount++
	m.LastUpdateTime = time.Now()
}

// IncrementRequeueFailure 增加重新入队失败计数
func (m *TaskMetrics) IncrementRequeueFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RequeueFailureCount++
	m.LastUpdateTime = time.Now()
}

// IncrementMarkFailedError 增加标记失败错误计数
func (m *TaskMetrics) IncrementMarkFailedError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MarkFailedErrorCount++
	m.LastUpdateTime = time.Now()
}

// GetSnapshot 获取指标快照（返回值的副本，不包含锁）
func (m *TaskMetrics) GetSnapshot() TaskMetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return TaskMetricsSnapshot{
		PendingCount:          m.PendingCount,
		ProcessingCount:       m.ProcessingCount,
		CompletedCount:        m.CompletedCount,
		FailedCount:           m.FailedCount,
		RequeuedCount:         m.RequeuedCount,
		HighPriorityCount:     m.HighPriorityCount,
		MediumPriorityCount:   m.MediumPriorityCount,
		LowPriorityCount:      m.LowPriorityCount,
		TotalWaitTime:         m.TotalWaitTime,
		TotalProcessTime:      m.TotalProcessTime,
		TaskCount:             m.TaskCount,
		HeartbeatTimeoutCount: m.HeartbeatTimeoutCount,
		TaskLossCount:         m.TaskLossCount,
		RequeueFailureCount:   m.RequeueFailureCount,
		MarkFailedErrorCount:  m.MarkFailedErrorCount,
		LastUpdateTime:        m.LastUpdateTime,
	}
}

// TaskMetricsSnapshot 任务指标快照（不包含锁）
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

// GetAverageWaitTime 获取平均等待时间
func (m *TaskMetrics) GetAverageWaitTime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.TaskCount == 0 {
		return 0
	}
	return m.TotalWaitTime / time.Duration(m.TaskCount)
}

// GetAverageProcessTime 获取平均处理时间
func (m *TaskMetrics) GetAverageProcessTime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.CompletedCount == 0 {
		return 0
	}
	return m.TotalProcessTime / time.Duration(m.CompletedCount)
}

// LogMetricsSummary 输出指标摘要日志
func (m *TaskMetrics) LogMetricsSummary() {
	snapshot := m.GetSnapshot()

	logrus.Infof("📊 [Metrics] 任务流转统计 - Pending: %d, Processing: %d, Completed: %d, Failed: %d, Requeued: %d",
		snapshot.PendingCount, snapshot.ProcessingCount, snapshot.CompletedCount, snapshot.FailedCount, snapshot.RequeuedCount)

	logrus.Infof("📊 [Metrics] 优先级分布 - High(>=80): %d, Medium(50-79): %d, Low(<50): %d",
		snapshot.HighPriorityCount, snapshot.MediumPriorityCount, snapshot.LowPriorityCount)

	avgWaitTime := m.GetAverageWaitTime()
	avgProcessTime := m.GetAverageProcessTime()
	logrus.Infof("📊 [Metrics] 平均时间 - 等待时间: %v, 处理时间: %v",
		avgWaitTime.Truncate(time.Millisecond), avgProcessTime.Truncate(time.Millisecond))

	if snapshot.HeartbeatTimeoutCount > 0 || snapshot.TaskLossCount > 0 || snapshot.RequeueFailureCount > 0 || snapshot.MarkFailedErrorCount > 0 {
		logrus.Warnf("⚠️ [Metrics] 异常统计 - 心跳超时: %d, 任务丢失: %d, 重新入队失败: %d, 标记失败错误: %d",
			snapshot.HeartbeatTimeoutCount, snapshot.TaskLossCount, snapshot.RequeueFailureCount, snapshot.MarkFailedErrorCount)
	}
}

// Reset 重置所有指标
func (m *TaskMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PendingCount = 0
	m.ProcessingCount = 0
	m.CompletedCount = 0
	m.FailedCount = 0
	m.RequeuedCount = 0
	m.HighPriorityCount = 0
	m.MediumPriorityCount = 0
	m.LowPriorityCount = 0
	m.TotalWaitTime = 0
	m.TotalProcessTime = 0
	m.TaskCount = 0
	m.HeartbeatTimeoutCount = 0
	m.TaskLossCount = 0
	m.RequeueFailureCount = 0
	m.MarkFailedErrorCount = 0
	m.LastUpdateTime = time.Now()
}
