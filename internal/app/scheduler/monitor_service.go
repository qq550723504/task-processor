// Package scheduler 提供任务监控服务功能
package scheduler

import (
	"encoding/json"
	"fmt"
	"sync"
	"task-processor/internal/core/logger"
	"time"

	"github.com/sirupsen/logrus"
)

// MonitorService 任务监控服务
type MonitorService struct {
	manager *Manager
	logger  *logrus.Entry
	mutex   sync.RWMutex
}

// NewMonitorService 创建新的监控服务
func NewMonitorService(manager *Manager) *MonitorService {
	return &MonitorService{
		manager: manager,
		logger: logger.GetGlobalLogger("monitor_service").WithField(
			logger.FieldComponent, "monitor_service",
		),
	}
}

// TaskMonitorInfo 任务监控信息
type TaskMonitorInfo struct {
	TaskID       string                `json:"task_id"`
	TaskType     TaskType              `json:"task_type"`
	Platform     string                `json:"platform"`
	StoreID      int64                 `json:"store_id"`
	Interval     time.Duration         `json:"interval"`
	IsRunning    bool                  `json:"is_running"`
	Status       TaskStatus            `json:"status"`
	Stats        ExecutorStatsSnapshot `json:"stats"`
	LastActivity time.Time             `json:"last_activity"`
}

// GetAllTasksMonitorInfo 获取所有任务的监控信息
func (m *MonitorService) GetAllTasksMonitorInfo() []TaskMonitorInfo {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var monitorInfos []TaskMonitorInfo

	for taskID, executor := range m.manager.executors {
		task := executor.GetTask()
		stats := executor.GetStats()

		info := TaskMonitorInfo{
			TaskID:       taskID,
			TaskType:     task.GetType(),
			Platform:     task.GetPlatform(),
			StoreID:      task.GetStoreID(),
			Interval:     task.GetInterval(),
			IsRunning:    executor.IsRunning(),
			Status:       task.GetStatus(),
			Stats:        stats,
			LastActivity: stats.LastExecutionTime,
		}

		monitorInfos = append(monitorInfos, info)
	}

	return monitorInfos
}

// GetTaskMonitorInfo 获取指定任务的监控信息
func (m *MonitorService) GetTaskMonitorInfo(taskID string) (*TaskMonitorInfo, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	executor, exists := m.manager.executors[taskID]
	if !exists {
		return nil, fmt.Errorf("任务 %s 不存在", taskID)
	}

	task := executor.GetTask()
	stats := executor.GetStats()

	info := &TaskMonitorInfo{
		TaskID:       taskID,
		TaskType:     task.GetType(),
		Platform:     task.GetPlatform(),
		StoreID:      task.GetStoreID(),
		Interval:     task.GetInterval(),
		IsRunning:    executor.IsRunning(),
		Status:       task.GetStatus(),
		Stats:        stats,
		LastActivity: stats.LastExecutionTime,
	}

	return info, nil
}

// SystemMonitorInfo 系统监控信息
type SystemMonitorInfo struct {
	TotalTasks     int                 `json:"total_tasks"`
	RunningTasks   int                 `json:"running_tasks"`
	StoppedTasks   int                 `json:"stopped_tasks"`
	ErrorTasks     int                 `json:"error_tasks"`
	PlatformStats  map[string]int      `json:"platform_stats"`
	TaskTypeStats  map[TaskType]int    `json:"task_type_stats"`
	OverallStats   SystemStatsSnapshot `json:"overall_stats"`
	LastUpdateTime time.Time           `json:"last_update_time"`
}

// SystemStatsSnapshot 系统统计快照
type SystemStatsSnapshot struct {
	TotalExecutions    int64         `json:"total_executions"`
	TotalSkips         int64         `json:"total_skips"`
	AverageSuccessRate float64       `json:"average_success_rate"`
	AverageSkipRate    float64       `json:"average_skip_rate"`
	TotalDuration      time.Duration `json:"total_duration"`
}

// GetSystemMonitorInfo 获取系统监控信息
func (m *MonitorService) GetSystemMonitorInfo() SystemMonitorInfo {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	info := SystemMonitorInfo{
		PlatformStats:  make(map[string]int),
		TaskTypeStats:  make(map[TaskType]int),
		LastUpdateTime: time.Now(),
	}

	var totalExecutions int64
	var totalSkips int64
	var totalSuccessRate float64
	var totalSkipRate float64
	var taskCount int

	for _, executor := range m.manager.executors {
		task := executor.GetTask()
		stats := executor.GetStats()

		// 统计任务数量
		info.TotalTasks++

		// 统计任务状态
		switch task.GetStatus() {
		case TaskStatusRunning:
			info.RunningTasks++
		case TaskStatusStopped:
			info.StoppedTasks++
		case TaskStatusError:
			info.ErrorTasks++
		}

		// 统计平台分布
		info.PlatformStats[task.GetPlatform()]++

		// 统计任务类型分布
		info.TaskTypeStats[task.GetType()]++

		// 累计统计信息
		totalExecutions += stats.TotalExecutions
		totalSkips += stats.SkipExecutions
		totalSuccessRate += stats.GetSuccessRate()
		totalSkipRate += stats.GetSkipRate()
		taskCount++
	}

	// 计算平均值
	if taskCount > 0 {
		info.OverallStats.AverageSuccessRate = totalSuccessRate / float64(taskCount)
		info.OverallStats.AverageSkipRate = totalSkipRate / float64(taskCount)
	}

	info.OverallStats.TotalExecutions = totalExecutions
	info.OverallStats.TotalSkips = totalSkips

	return info
}

// GetTasksHealthCheck 获取任务健康检查信息
func (m *MonitorService) GetTasksHealthCheck() map[string]string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	healthStatus := make(map[string]string)

	for taskID, executor := range m.manager.executors {
		stats := executor.GetStats()

		// 判断任务健康状态
		status := "healthy"

		// 检查是否长时间没有执行
		if !stats.LastExecutionTime.IsZero() {
			timeSinceLastExecution := time.Since(stats.LastExecutionTime)
			task := executor.GetTask()

			// 如果超过3个执行间隔还没有执行，认为不健康
			if timeSinceLastExecution > task.GetInterval()*3 {
				status = "inactive"
			}
		}

		// 检查失败率
		if stats.GetFailureRate() > 50 {
			status = "unhealthy"
		}

		// 检查跳过率
		if stats.GetSkipRate() > 80 {
			status = "overloaded"
		}

		healthStatus[taskID] = status
	}

	return healthStatus
}

// ExportMonitorData 导出监控数据为JSON
func (m *MonitorService) ExportMonitorData() ([]byte, error) {
	data := struct {
		SystemInfo  SystemMonitorInfo `json:"system_info"`
		TasksInfo   []TaskMonitorInfo `json:"tasks_info"`
		HealthCheck map[string]string `json:"health_check"`
		ExportTime  time.Time         `json:"export_time"`
	}{
		SystemInfo:  m.GetSystemMonitorInfo(),
		TasksInfo:   m.GetAllTasksMonitorInfo(),
		HealthCheck: m.GetTasksHealthCheck(),
		ExportTime:  time.Now(),
	}

	return json.MarshalIndent(data, "", "  ")
}

// ResetTaskStats 重置指定任务的统计信息
func (m *MonitorService) ResetTaskStats(taskID string) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	executor, exists := m.manager.executors[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	executor.ResetStats()
	m.logger.WithField("task_id", taskID).Info("已重置任务统计信息")
	return nil
}

// ResetAllTasksStats 重置所有任务的统计信息
func (m *MonitorService) ResetAllTasksStats() {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for taskID, executor := range m.manager.executors {
		executor.ResetStats()
		m.logger.WithField("task_id", taskID).Info("已重置任务统计信息")
	}

	m.logger.Info("已重置所有任务的统计信息")
}
