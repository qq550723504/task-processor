// Package task 提供统一的任务清理服务
package task

import (
	"context"
	"task-processor/internal/core/config"
	"time"

	"github.com/sirupsen/logrus"
)

// CleanupService 统一清理服务
type CleanupService struct {
	fetcher    *TaskFetcher
	config     *CleanupConfig
	strategies []CleanupStrategy
}

// NewCleanupService 创建清理服务
func NewCleanupService(fetcher *TaskFetcher, cfg *config.Config) *CleanupService {
	cleanupConfig := DefaultCleanupConfig()

	// 从配置文件读取参数
	if cfg != nil && cfg.Worker.CleanupInterval > 0 {
		cleanupConfig.CleanupInterval = time.Duration(cfg.Worker.CleanupInterval) * time.Second
	}
	if cfg != nil && cfg.Worker.TaskTimeout > 0 {
		cleanupConfig.TaskTimeout = time.Duration(cfg.Worker.TaskTimeout) * time.Second
	}
	if cfg != nil && cfg.Worker.StuckTaskThreshold > 0 {
		cleanupConfig.StuckTaskThreshold = time.Duration(cfg.Worker.StuckTaskThreshold) * time.Second
	}
	if cfg != nil && cfg.Worker.ForceCleanupAfter > 0 {
		cleanupConfig.ForceCleanupAfter = time.Duration(cfg.Worker.ForceCleanupAfter) * time.Second
	}

	service := &CleanupService{
		fetcher: fetcher,
		config:  cleanupConfig,
	}

	// 注册清理策略
	service.registerStrategies()

	return service
}

// registerStrategies 注册清理策略
func (s *CleanupService) registerStrategies() {
	s.strategies = []CleanupStrategy{
		&ForceCleanupStrategy{threshold: s.config.ForceCleanupAfter},
		&TimeoutCleanupStrategy{threshold: s.config.TaskTimeout},
		&StuckTaskCleanupStrategy{threshold: s.config.StuckTaskThreshold},
	}
}

// Start 启动清理服务
func (s *CleanupService) Start(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("清理服务goroutine panic: %v", r)
		}
	}()

	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	logrus.Infof("🧹 统一清理服务启动，间隔: %v", s.config.CleanupInterval)

	for {
		select {
		case <-ctx.Done():
			logrus.Info("统一清理服务停止")
			return
		case <-ticker.C:
			s.performCleanup()
		}
	}
}

// performCleanup 执行清理
func (s *CleanupService) performCleanup() {
	s.fetcher.tasksMutex.Lock()
	defer s.fetcher.tasksMutex.Unlock()

	stats := NewCleanupStats()
	now := time.Now()

	for taskID, submitTime := range s.fetcher.processingTasks {
		duration := now.Sub(submitTime)

		// 使用策略模式进行清理判断
		for _, strategy := range s.strategies {
			if shouldClean, reason := strategy.ShouldCleanup(taskID, duration); shouldClean {
				delete(s.fetcher.processingTasks, taskID)

				// 根据原因分类统计
				status := s.getTaskStatusByReason(reason)
				stats.AddCleaned(status)

				s.logCleanup(taskID, duration, reason)
				break
			}
		}
	}

	stats.RemainingTasks = len(s.fetcher.processingTasks)
	s.reportCleanupStats(stats)

	// 检查是否需要紧急处理
	if stats.RemainingTasks > 15 {
		s.handleEmergencyCleanup()
	}
}

// getTaskStatusByReason 根据清理原因获取任务状态
func (s *CleanupService) getTaskStatusByReason(reason string) TaskStatus {
	switch reason {
	case "30分钟强制清理", "强制清理":
		return TaskStatusForced
	case "任务超时":
		return TaskStatusExpired
	case "任务卡住":
		return TaskStatusStuck
	default:
		return TaskStatusExpired
	}
}

// logCleanup 记录清理日志
func (s *CleanupService) logCleanup(taskID string, duration time.Duration, reason string) {
	if reason == "30分钟强制清理" {
		logrus.Errorf("🚨 %s: TaskID=%s, 运行时长=%.1fm", reason, taskID, duration.Minutes())
	} else {
		logrus.Warnf("🗑️ 清理任务: TaskID=%s, 运行时长=%.1fm, 原因=%s",
			taskID, duration.Minutes(), reason)
	}
}

// reportCleanupStats 报告清理统计
func (s *CleanupService) reportCleanupStats(stats *CleanupStats) {
	if stats.TotalCleaned > 0 {
		logrus.Infof("🧹 清理完成: 强制=%d, 超时=%d, 卡住=%d, 总计=%d, 剩余=%d",
			stats.ForcedTasks, stats.ExpiredTasks, stats.StuckTasks,
			stats.TotalCleaned, stats.RemainingTasks)
	}
}

// handleEmergencyCleanup 处理紧急清理
func (s *CleanupService) handleEmergencyCleanup() {
	logrus.Warnf("⚠️ 处理中任务过多(%d)，执行紧急清理", len(s.fetcher.processingTasks))

	emergencyThreshold := 2 * time.Minute
	now := time.Now()
	emergencyCleaned := 0

	for taskID, submitTime := range s.fetcher.processingTasks {
		if now.Sub(submitTime) > emergencyThreshold {
			delete(s.fetcher.processingTasks, taskID)
			emergencyCleaned++

			logrus.Warnf("🚨 紧急清理: TaskID=%s", taskID)
		}
	}

	if emergencyCleaned > 0 {
		logrus.Warnf("🚨 紧急清理完成: 清理=%d, 剩余=%d",
			emergencyCleaned, len(s.fetcher.processingTasks))
	}
}

// ForceCleanupAll 强制清理所有超过阈值的任务
func (s *CleanupService) ForceCleanupAll(threshold time.Duration) int {
	s.fetcher.tasksMutex.Lock()
	defer s.fetcher.tasksMutex.Unlock()

	now := time.Now()
	forceCleaned := 0

	logrus.Warnf("🚨 执行强制清理，阈值: %.1fm", threshold.Minutes())

	for taskID, submitTime := range s.fetcher.processingTasks {
		duration := now.Sub(submitTime)

		if duration > threshold {
			delete(s.fetcher.processingTasks, taskID)
			forceCleaned++

			logrus.Errorf("🚨 强制清理: TaskID=%s, 运行时长=%.1fm",
				taskID, duration.Minutes())
		}
	}

	if forceCleaned > 0 {
		logrus.Errorf("🚨 强制清理完成: 清理=%d, 剩余=%d",
			forceCleaned, len(s.fetcher.processingTasks))
	}

	return forceCleaned
}

// GetLongRunningTasks 获取长时间运行的任务
func (s *CleanupService) GetLongRunningTasks(threshold time.Duration) []QueueTaskInfo {
	return s.fetcher.GetLongRunningTasks(threshold)
}

// 清理策略实现

// ForceCleanupStrategy 强制清理策略
type ForceCleanupStrategy struct {
	threshold time.Duration
}

func (s *ForceCleanupStrategy) ShouldCleanup(taskID string, duration time.Duration) (bool, string) {
	if duration > s.threshold {
		return true, "30分钟强制清理"
	}
	return false, ""
}

func (s *ForceCleanupStrategy) GetPriority() int {
	return 1 // 最高优先级
}

// TimeoutCleanupStrategy 超时清理策略
type TimeoutCleanupStrategy struct {
	threshold time.Duration
}

func (s *TimeoutCleanupStrategy) ShouldCleanup(taskID string, duration time.Duration) (bool, string) {
	if duration > s.threshold {
		return true, "任务超时"
	}
	return false, ""
}

func (s *TimeoutCleanupStrategy) GetPriority() int {
	return 2
}

// StuckTaskCleanupStrategy 卡住任务清理策略
type StuckTaskCleanupStrategy struct {
	threshold time.Duration
}

func (s *StuckTaskCleanupStrategy) ShouldCleanup(taskID string, duration time.Duration) (bool, string) {
	if duration > s.threshold {
		// 简单的卡住检测逻辑
		return true, "任务卡住"
	}
	return false, ""
}

func (s *StuckTaskCleanupStrategy) GetPriority() int {
	return 3
}
