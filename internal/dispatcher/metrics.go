// Package dispatcher 提供指标收集功能
package dispatcher

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// metricsCollector 指标收集器实现
type metricsCollector struct {
	metrics map[string]*TaskMetrics
	tasks   map[string]*taskRecord // taskID -> taskRecord
	mutex   sync.RWMutex
	logger  *logrus.Logger
}

// taskRecord 任务记录
type taskRecord struct {
	TaskID    string
	Platform  string
	StartTime time.Time
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector(logger *logrus.Logger) MetricsCollector {
	return &metricsCollector{
		metrics: make(map[string]*TaskMetrics),
		tasks:   make(map[string]*taskRecord),
		logger:  logger,
	}
}

// RecordTaskStart 记录任务开始
func (m *metricsCollector) RecordTaskStart(platform string, taskID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 记录任务开始时间
	m.tasks[taskID] = &taskRecord{
		TaskID:    taskID,
		Platform:  platform,
		StartTime: time.Now(),
	}

	// 初始化平台指标（如果不存在）
	if _, exists := m.metrics[platform]; !exists {
		m.metrics[platform] = &TaskMetrics{
			LastUpdateTime: time.Now(),
		}
	}

	metrics := m.metrics[platform]
	metrics.TotalTasks++
	metrics.LastUpdateTime = time.Now()

	m.logger.Debugf("[Metrics] 记录任务开始: Platform=%s, TaskID=%s", platform, taskID)
}

// RecordTaskSuccess 记录任务成功
func (m *metricsCollector) RecordTaskSuccess(platform string, taskID string, duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 清理任务记录
	delete(m.tasks, taskID)

	// 更新指标
	if metrics, exists := m.metrics[platform]; exists {
		metrics.SuccessfulTasks++
		m.updateAverageTime(metrics, duration)
		m.updateThroughput(metrics)
		metrics.LastUpdateTime = time.Now()
	}

	m.logger.Debugf("[Metrics] 记录任务成功: Platform=%s, TaskID=%s, Duration=%v", platform, taskID, duration)
}

// RecordTaskFailure 记录任务失败
func (m *metricsCollector) RecordTaskFailure(platform string, taskID string, duration time.Duration, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 清理任务记录
	delete(m.tasks, taskID)

	// 更新指标
	if metrics, exists := m.metrics[platform]; exists {
		metrics.FailedTasks++
		m.updateAverageTime(metrics, duration)
		m.updateThroughput(metrics)
		metrics.LastUpdateTime = time.Now()
	}

	m.logger.Debugf("[Metrics] 记录任务失败: Platform=%s, TaskID=%s, Duration=%v, Error=%v", platform, taskID, duration, err)
}

// updateAverageTime 更新平均处理时间
func (m *metricsCollector) updateAverageTime(metrics *TaskMetrics, duration time.Duration) {
	totalCompleted := metrics.SuccessfulTasks + metrics.FailedTasks
	if totalCompleted == 1 {
		metrics.AverageTime = duration
	} else {
		// 计算加权平均时间
		oldWeight := float64(totalCompleted - 1)
		newWeight := 1.0
		totalWeight := oldWeight + newWeight

		oldAvg := float64(metrics.AverageTime)
		newDuration := float64(duration)

		metrics.AverageTime = time.Duration((oldAvg*oldWeight + newDuration*newWeight) / totalWeight)
	}
}

// updateThroughput 更新吞吐量
func (m *metricsCollector) updateThroughput(metrics *TaskMetrics) {
	totalCompleted := metrics.SuccessfulTasks + metrics.FailedTasks
	if totalCompleted > 0 && metrics.AverageTime > 0 {
		// 计算每秒处理任务数
		metrics.ThroughputPerSecond = 1.0 / metrics.AverageTime.Seconds()
	}
}

// GetMetrics 获取指标
func (m *metricsCollector) GetMetrics(platform string) *TaskMetrics {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if metrics, exists := m.metrics[platform]; exists {
		// 返回副本
		return &TaskMetrics{
			TotalTasks:          metrics.TotalTasks,
			SuccessfulTasks:     metrics.SuccessfulTasks,
			FailedTasks:         metrics.FailedTasks,
			AverageTime:         metrics.AverageTime,
			ThroughputPerSecond: metrics.ThroughputPerSecond,
			LastUpdateTime:      metrics.LastUpdateTime,
		}
	}

	return nil
}

// GetAllMetrics 获取所有平台指标
func (m *metricsCollector) GetAllMetrics() map[string]*TaskMetrics {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*TaskMetrics)
	for platform, metrics := range m.metrics {
		result[platform] = &TaskMetrics{
			TotalTasks:          metrics.TotalTasks,
			SuccessfulTasks:     metrics.SuccessfulTasks,
			FailedTasks:         metrics.FailedTasks,
			AverageTime:         metrics.AverageTime,
			ThroughputPerSecond: metrics.ThroughputPerSecond,
			LastUpdateTime:      metrics.LastUpdateTime,
		}
	}

	return result
}

// GetRunningTasks 获取正在运行的任务数量
func (m *metricsCollector) GetRunningTasks(platform string) int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	count := 0
	for _, task := range m.tasks {
		if task.Platform == platform {
			count++
		}
	}

	return count
}

// GetAllRunningTasks 获取所有平台正在运行的任务
func (m *metricsCollector) GetAllRunningTasks() map[string]int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]int)
	for _, task := range m.tasks {
		result[task.Platform]++
	}

	return result
}

// CleanupOldTasks 清理超时的任务记录
func (m *metricsCollector) CleanupOldTasks(timeout time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	for taskID, task := range m.tasks {
		if now.Sub(task.StartTime) > timeout {
			delete(m.tasks, taskID)
			m.logger.Warnf("[Metrics] 清理超时任务记录: TaskID=%s, Platform=%s", taskID, task.Platform)
		}
	}
}

// Reset 重置指标
func (m *metricsCollector) Reset(platform string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if platform == "" {
		// 重置所有平台
		m.metrics = make(map[string]*TaskMetrics)
		m.tasks = make(map[string]*taskRecord)
		m.logger.Info("[Metrics] 重置所有平台指标")
	} else {
		// 重置指定平台
		delete(m.metrics, platform)

		// 清理该平台的任务记录
		for taskID, task := range m.tasks {
			if task.Platform == platform {
				delete(m.tasks, taskID)
			}
		}

		m.logger.Infof("[Metrics] 重置平台指标: %s", platform)
	}
}
