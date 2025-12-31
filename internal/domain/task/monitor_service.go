// Package task 提供统一的队列监控服务
package task

import (
	"context"
	"time"

	"task-processor/internal/worker"

	"github.com/sirupsen/logrus"
)

// MonitorService 统一监控服务
type MonitorService struct {
	fetcher    *TaskFetcher
	strategies []MonitorStrategy
}

// NewMonitorService 创建监控服务
func NewMonitorService(fetcher *TaskFetcher) *MonitorService {
	service := &MonitorService{
		fetcher: fetcher,
	}

	// 注册监控策略
	service.registerStrategies()

	return service
}

// registerStrategies 注册监控策略
func (s *MonitorService) registerStrategies() {
	s.strategies = []MonitorStrategy{
		&CriticalMonitorStrategy{threshold: 90.0},
		&WarningMonitorStrategy{threshold: 75.0},
		&NormalMonitorStrategy{threshold: 50.0},
	}
}

// StartMonitoring 启动监控
func (s *MonitorService) StartMonitoring(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("监控服务goroutine panic: %v", r)
		}
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	logrus.Info("🔍 统一监控服务启动")

	for {
		select {
		case <-ctx.Done():
			logrus.Info("统一监控服务停止")
			return
		case <-ticker.C:
			s.performHealthCheck()
		}
	}
}

// performHealthCheck 执行健康检查
func (s *MonitorService) performHealthCheck() {
	if s.fetcher == nil || s.fetcher.submitters == nil {
		return
	}

	totalQueued := 0
	totalCapacity := 0
	problemPlatforms := make([]string, 0)

	logrus.Debug("📊 执行队列健康检查")

	for platform, submitter := range s.fetcher.submitters {
		stats := submitter.GetQueueStats()
		totalQueued += stats.QueueSize
		totalCapacity += stats.BufferSize

		// 使用策略模式评估健康状态
		status, issues := s.evaluateQueueHealth(stats)

		logrus.Debugf("[%s] 队列状态: %d/%d (%.1f%%) - %s",
			platform, stats.QueueSize, stats.BufferSize,
			stats.UsagePercent, status)

		if len(issues) > 0 {
			problemPlatforms = append(problemPlatforms, platform)
			for _, issue := range issues {
				logrus.Warnf("[%s] ⚠️ %s", platform, issue)
			}
		}
	}

	// 检查处理中任务状态
	s.checkProcessingTasks()

	// 总体健康评估
	overallUsage := float64(totalQueued) / float64(totalCapacity) * 100
	logrus.Debugf("🎯 总体队列使用率: %.1f%% (%d/%d)",
		overallUsage, totalQueued, totalCapacity)

	if len(problemPlatforms) > 0 {
		logrus.Warnf("⚠️ 发现队列压力过大的平台: %v", problemPlatforms)
	}
}

// evaluateQueueHealth 评估队列健康状态
func (s *MonitorService) evaluateQueueHealth(stats worker.QueueStats) (string, []string) {
	for _, strategy := range s.strategies {
		if stats.UsagePercent >= strategy.GetThreshold() {
			return strategy.EvaluateHealth(stats)
		}
	}
	return "🔵 空闲", []string{}
}

// checkProcessingTasks 检查处理中任务状态
func (s *MonitorService) checkProcessingTasks() {
	s.fetcher.tasksMutex.RLock()
	processingCount := len(s.fetcher.processingTasks)
	s.fetcher.tasksMutex.RUnlock()

	if processingCount > 0 {
		logrus.Debugf("⏳ 当前处理中任务数量: %d", processingCount)

		// 检查长时间运行的任务
		longRunningTasks := s.getLongRunningTasks(10 * time.Minute)
		if len(longRunningTasks) > 0 {
			taskIDs := make([]string, len(longRunningTasks))
			for i, task := range longRunningTasks {
				taskIDs[i] = task.ID
			}
			logrus.Warnf("⏰ 发现长时间运行任务(%d): %v", len(longRunningTasks), taskIDs)
		}
	}
}

// getLongRunningTasks 获取长时间运行的任务
func (s *MonitorService) getLongRunningTasks(threshold time.Duration) []QueueTaskInfo {
	s.fetcher.tasksMutex.RLock()
	defer s.fetcher.tasksMutex.RUnlock()

	now := time.Now()
	longRunningTasks := make([]QueueTaskInfo, 0)

	for taskID, submitTime := range s.fetcher.processingTasks {
		duration := now.Sub(submitTime)
		if duration > threshold {
			longRunningTasks = append(longRunningTasks, QueueTaskInfo{
				ID:       taskID,
				Duration: duration,
			})
		}
	}

	return longRunningTasks
}

// GenerateReport 生成监控报告
func (s *MonitorService) GenerateReport() *MonitorReport {
	report := &MonitorReport{
		Timestamp:       time.Now(),
		PlatformReports: make([]PlatformReport, 0),
		Recommendations: make([]string, 0),
	}

	if s.fetcher == nil || s.fetcher.submitters == nil {
		report.OverallHealth = "🔴 系统异常"
		report.Recommendations = append(report.Recommendations, "TaskFetcher未正确初始化")
		return report
	}

	// 分析各平台状态
	totalUsage := 0.0
	problemCount := 0

	for platform, submitter := range s.fetcher.submitters {
		stats := submitter.GetQueueStats()

		platformReport := PlatformReport{
			Platform:       platform,
			QueueSize:      stats.QueueSize,
			BufferSize:     stats.BufferSize,
			UsagePercent:   stats.UsagePercent,
			AvailableSlots: stats.AvailableSlots,
			Issues:         make([]string, 0),
		}

		// 评估平台状态
		platformReport.Status, platformReport.Issues = s.evaluateQueueHealth(stats)

		if stats.UsagePercent > 75 {
			problemCount++
		}

		totalUsage += stats.UsagePercent
		report.PlatformReports = append(report.PlatformReports, platformReport)
	}

	report.TotalPlatforms = len(s.fetcher.submitters)

	// 检查处理中任务
	s.fetcher.tasksMutex.RLock()
	report.ProcessingTasks = len(s.fetcher.processingTasks)
	s.fetcher.tasksMutex.RUnlock()

	// 评估整体健康状态
	avgUsage := totalUsage / float64(report.TotalPlatforms)
	report.OverallHealth = s.evaluateOverallHealth(avgUsage, problemCount, report.ProcessingTasks)
	report.HealthScore = s.calculateHealthScore(avgUsage, report.ProcessingTasks)

	// 生成建议
	report.Recommendations = s.generateRecommendations(avgUsage, problemCount, report.ProcessingTasks)

	return report
}

// evaluateOverallHealth 评估整体健康状态
func (s *MonitorService) evaluateOverallHealth(avgUsage float64, problemCount, processingTasks int) string {
	switch {
	case problemCount > 0 || avgUsage > 80:
		return "🔴 需要关注"
	case avgUsage > 60 || processingTasks > 20:
		return "🟡 轻微压力"
	case avgUsage > 30:
		return "🟢 健康"
	default:
		return "🔵 空闲"
	}
}

// calculateHealthScore 计算健康评分
func (s *MonitorService) calculateHealthScore(avgUsage float64, processingTasks int) int {
	score := 100.0 - avgUsage

	// 处理中任务过多会降低分数
	if processingTasks > 15 {
		penalty := float64(processingTasks-15) * 2
		score -= penalty
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return int(score)
}

// generateRecommendations 生成优化建议
func (s *MonitorService) generateRecommendations(avgUsage float64, problemCount, processingTasks int) []string {
	recommendations := make([]string, 0)

	if avgUsage > 80 {
		recommendations = append(recommendations, "建议增加 bufferSize 配置")
		recommendations = append(recommendations, "建议降低 queueThreshold 阈值")
	}

	if problemCount > 0 {
		recommendations = append(recommendations, "建议减少 maxFetchPerCycle 数量")
		recommendations = append(recommendations, "检查任务处理性能")
	}

	if processingTasks > 20 {
		recommendations = append(recommendations, "检查任务完成回调机制")
		recommendations = append(recommendations, "考虑缩短任务超时时间")
	}

	if avgUsage < 20 && processingTasks < 5 {
		recommendations = append(recommendations, "系统负载较低，可以适当增加并发数")
	}

	return recommendations
}

// 监控策略实现

// CriticalMonitorStrategy 严重监控策略
type CriticalMonitorStrategy struct {
	threshold float64
}

func (s *CriticalMonitorStrategy) EvaluateHealth(stats worker.QueueStats) (string, []string) {
	issues := []string{"队列几乎满载，可能阻塞新任务"}
	return "🔴 严重拥堵", issues
}

func (s *CriticalMonitorStrategy) GetThreshold() float64 {
	return s.threshold
}

// WarningMonitorStrategy 警告监控策略
type WarningMonitorStrategy struct {
	threshold float64
}

func (s *WarningMonitorStrategy) EvaluateHealth(stats worker.QueueStats) (string, []string) {
	issues := []string{"队列使用率过高，需要关注"}
	return "🟡 轻度拥堵", issues
}

func (s *WarningMonitorStrategy) GetThreshold() float64 {
	return s.threshold
}

// NormalMonitorStrategy 正常监控策略
type NormalMonitorStrategy struct {
	threshold float64
}

func (s *NormalMonitorStrategy) EvaluateHealth(stats worker.QueueStats) (string, []string) {
	return "🟢 正常", []string{}
}

func (s *NormalMonitorStrategy) GetThreshold() float64 {
	return s.threshold
}
