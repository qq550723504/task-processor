// Package task provides a conservative cleanup service for in-flight tasks.
package task

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
)

// CleanupService periodically inspects long-running tasks and reports suspicious ones.
//
// It intentionally does not remove entries from processingTasks automatically, because
// local cleanup without a verified remote/task-runtime completion signal can cause the
// same task to be fetched and executed twice.
type CleanupService struct {
	fetcher    *TaskFetcher
	config     *CleanupConfig
	strategies []CleanupStrategy
}

func NewCleanupService(fetcher *TaskFetcher, cfg *config.Config) *CleanupService {
	cleanupConfig := DefaultCleanupConfig()

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
	service.registerStrategies()
	return service
}

func (s *CleanupService) registerStrategies() {
	s.strategies = []CleanupStrategy{
		&ForceCleanupStrategy{threshold: s.config.ForceCleanupAfter},
		&TimeoutCleanupStrategy{threshold: s.config.TaskTimeout},
		&StuckTaskCleanupStrategy{threshold: s.config.StuckTaskThreshold},
	}
}

func (s *CleanupService) Start(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logger.GetGlobalLogger("app/task").Errorf("清理服务goroutine panic: %v", r)
		}
	}()

	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	logger.GetGlobalLogger("app/task").Infof("🧹 统一清理服务启动，间隔: %v", s.config.CleanupInterval)

	for {
		select {
		case <-ctx.Done():
			logger.GetGlobalLogger("app/task").Info("统一清理服务停止")
			return
		case <-ticker.C:
			s.performCleanup()
		}
	}
}

func (s *CleanupService) performCleanup() {
	stats := NewCleanupStats()
	now := time.Now()
	processingTasks := s.snapshotProcessingTasks()
	claimEntries := s.loadClaimJournalEntries()

	for taskID, submitTime := range processingTasks {
		duration := now.Sub(submitTime)

		for _, strategy := range s.strategies {
			if shouldFlag, reason := strategy.ShouldCleanup(taskID, duration); shouldFlag {
				status := s.getTaskStatusByReason(reason)
				stats.AddCleaned(status)
				recovered := false
				if reason == "30分钟强制清理" {
					recovered = s.tryRecoverClaimedTask(taskID, duration, reason, claimEntries)
				}
				if !recovered {
					s.logCleanupCandidate(taskID, duration, reason)
				}
				break
			}
		}
	}

	stats.RemainingTasks = len(s.snapshotProcessingTasks())
	s.reportCleanupStats(stats)

	if stats.RemainingTasks > 15 {
		s.handleEmergencyCleanup()
	}
}

func (s *CleanupService) snapshotProcessingTasks() map[string]time.Time {
	s.fetcher.tasksMutex.RLock()
	defer s.fetcher.tasksMutex.RUnlock()

	snapshot := make(map[string]time.Time, len(s.fetcher.processingTasks))
	for taskID, submitTime := range s.fetcher.processingTasks {
		snapshot[taskID] = submitTime
	}
	return snapshot
}

func (s *CleanupService) loadClaimJournalEntries() map[string]ClaimJournalEntry {
	if s == nil || s.fetcher == nil || s.fetcher.claimJournal == nil {
		return nil
	}

	entries, err := s.fetcher.claimJournal.LoadAll()
	if err != nil {
		logger.GetGlobalLogger("app/task").WithError(err).Warn("加载 claim journal 失败，跳过自动恢复")
		return nil
	}

	result := make(map[string]ClaimJournalEntry, len(entries))
	for _, entry := range entries {
		result[fmt.Sprintf("%d", entry.TaskID)] = entry
	}
	return result
}

func (s *CleanupService) tryRecoverClaimedTask(
	taskID string,
	duration time.Duration,
	reason string,
	claimEntries map[string]ClaimJournalEntry,
) bool {
	if s == nil || s.fetcher == nil {
		return false
	}

	entry, exists := claimEntries[taskID]
	if !exists {
		return false
	}

	statusService := s.fetcher.newTaskStatusService("app/task_cleanup_recovery")
	input := taskStatusRecoveryInputWithFallback(entry, "task processing lease expired, recovered by cleanup service")
	if err := statusService.UpdateSyncWithInput(input); err != nil {
		logger.GetGlobalLogger("app/task").WithError(err).Warnf(
			"自动恢复超时任务失败: TaskID=%s, Duration=%.1fm, Reason=%s",
			taskID,
			duration.Minutes(),
			reason,
		)
		return false
	}

	s.fetcher.RemoveProcessingTask(taskID)
	logger.GetGlobalLogger("app/task").Warnf(
		"自动回收超时 claim 任务为 pending_retry: TaskID=%s, 运行时长=%.1fm, Reason=%s, ProductID=%s",
		taskID,
		duration.Minutes(),
		reason,
		entry.ProductID,
	)
	return true
}

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

func (s *CleanupService) logCleanupCandidate(taskID string, duration time.Duration, reason string) {
	if reason == "30分钟强制清理" {
		logger.GetGlobalLogger("app/task").Errorf(
			"🚨 检测到需要人工介入的长跑任务: TaskID=%s, 运行时长=%.1fm, 原因=%s",
			taskID,
			duration.Minutes(),
			reason,
		)
		return
	}

	logger.GetGlobalLogger("app/task").Warnf(
		"⏰ 检测到可疑处理中任务: TaskID=%s, 运行时长=%.1fm, 原因=%s",
		taskID,
		duration.Minutes(),
		reason,
	)
}

func (s *CleanupService) reportCleanupStats(stats *CleanupStats) {
	if stats.TotalCleaned > 0 {
		logger.GetGlobalLogger("app/task").Infof(
			"🧹 任务巡检完成: 强制候选=%d, 超时候选=%d, 卡住候选=%d, 总候选=%d, 当前处理中=%d",
			stats.ForcedTasks,
			stats.ExpiredTasks,
			stats.StuckTasks,
			stats.TotalCleaned,
			stats.RemainingTasks,
		)
	}
}

func (s *CleanupService) handleEmergencyCleanup() {
	logger.GetGlobalLogger("app/task").Warnf(
		"⚠️ 处理中任务过多(%d)，进入保护模式，仅告警不自动释放任务",
		len(s.fetcher.processingTasks),
	)

	emergencyThreshold := 2 * time.Minute
	now := time.Now()
	emergencyCandidates := 0

	for taskID, submitTime := range s.fetcher.processingTasks {
		if now.Sub(submitTime) > emergencyThreshold {
			emergencyCandidates++
			logger.GetGlobalLogger("app/task").Warnf(
				"🚨 紧急巡检命中: TaskID=%s, 运行时长=%.1fm，保留 processing 标记等待真实完成信号",
				taskID,
				now.Sub(submitTime).Minutes(),
			)
		}
	}

	if emergencyCandidates > 0 {
		logger.GetGlobalLogger("app/task").Warnf(
			"🚨 紧急巡检完成: 候选=%d, 当前处理中=%d",
			emergencyCandidates,
			len(s.fetcher.processingTasks),
		)
	}
}

// ForceCleanupAll keeps the old API for callers, but no longer releases tasks automatically.
// It now reports how many tasks exceed the threshold and require manual intervention.
func (s *CleanupService) ForceCleanupAll(threshold time.Duration) int {
	s.fetcher.tasksMutex.RLock()
	defer s.fetcher.tasksMutex.RUnlock()

	now := time.Now()
	candidates := 0

	logger.GetGlobalLogger("app/task").Warnf("🚨 执行人工巡检，阈值: %.1fm", threshold.Minutes())

	for taskID, submitTime := range s.fetcher.processingTasks {
		duration := now.Sub(submitTime)
		if duration > threshold {
			candidates++
			logger.GetGlobalLogger("app/task").Errorf(
				"🚨 人工介入候选: TaskID=%s, 运行时长=%.1fm，未自动移除 processing 标记",
				taskID,
				duration.Minutes(),
			)
		}
	}

	if candidates > 0 {
		logger.GetGlobalLogger("app/task").Errorf(
			"🚨 人工巡检完成: 候选=%d, 当前处理中=%d",
			candidates,
			len(s.fetcher.processingTasks),
		)
	}

	return candidates
}

func (s *CleanupService) GetLongRunningTasks(threshold time.Duration) []QueueTaskInfo {
	return s.fetcher.GetLongRunningTasks(threshold)
}

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
	return 1
}

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

type StuckTaskCleanupStrategy struct {
	threshold time.Duration
}

func (s *StuckTaskCleanupStrategy) ShouldCleanup(taskID string, duration time.Duration) (bool, string) {
	if duration > s.threshold {
		return true, "任务卡住"
	}
	return false, ""
}

func (s *StuckTaskCleanupStrategy) GetPriority() int {
	return 3
}
