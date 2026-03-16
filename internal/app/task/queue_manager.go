// Package task 提供简化的队列管理功能
package task

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// QueueManager 简化的队列管理器
type QueueManager struct {
	fetcher        *TaskFetcher
	cleanupService *CleanupService
	monitorService *MonitorService
}

// NewQueueManager 创建队列管理器
func NewQueueManager(fetcher *TaskFetcher, cleanupService *CleanupService, monitorService *MonitorService) *QueueManager {
	return &QueueManager{
		fetcher:        fetcher,
		cleanupService: cleanupService,
		monitorService: monitorService,
	}
}

// QueueCommand 队列管理命令
type QueueCommand string

const (
	CmdStatus      QueueCommand = "status"      // 查看状态
	CmdDiagnose    QueueCommand = "diagnose"    // 诊断问题
	CmdCleanup     QueueCommand = "cleanup"     // 强制清理
	CmdReset       QueueCommand = "reset"       // 重置队列
	CmdHealthCheck QueueCommand = "healthcheck" // 健康检查
	CmdForce30Min  QueueCommand = "force30min"  // 30分钟强制清理
)

// ExecuteCommand 执行队列管理命令
func (m *QueueManager) ExecuteCommand(cmd QueueCommand) error {
	switch cmd {
	case CmdStatus:
		return m.showStatus()
	case CmdDiagnose:
		return m.diagnoseIssues()
	case CmdCleanup:
		return m.forceCleanup()
	case CmdReset:
		return m.resetQueues()
	case CmdHealthCheck:
		return m.performHealthCheck()
	case CmdForce30Min:
		return m.force30MinCleanup()
	default:
		return fmt.Errorf("未知命令: %s", cmd)
	}
}

// showStatus 显示队列状态
func (m *QueueManager) showStatus() error {
	logrus.Info("📊 当前队列状态:")

	if m.fetcher == nil || m.fetcher.submitters == nil {
		logrus.Error("TaskFetcher未初始化")
		return fmt.Errorf("TaskFetcher未初始化")
	}

	totalQueued := 0
	totalCapacity := 0

	for platform, submitter := range m.fetcher.submitters {
		stats := submitter.GetQueueStats()
		totalQueued += stats.QueueSize
		totalCapacity += stats.BufferSize

		logrus.Infof("[%s] 队列: %d/%d (%.1f%%), 可用: %d",
			platform, stats.QueueSize, stats.BufferSize,
			stats.UsagePercent, stats.AvailableSlots)
	}

	m.fetcher.tasksMutex.RLock()
	processingCount := len(m.fetcher.processingTasks)
	m.fetcher.tasksMutex.RUnlock()

	logrus.Infof("总计: %d/%d (%.1f%%), 处理中: %d",
		totalQueued, totalCapacity,
		float64(totalQueued)/float64(totalCapacity)*100, processingCount)

	return nil
}

// diagnoseIssues 诊断队列问题
func (m *QueueManager) diagnoseIssues() error {
	logrus.Info("🔍 开始队列诊断...")

	report := m.monitorService.GenerateReport()

	logrus.Infof("📋 队列诊断报告 - %s", report.Timestamp.Format("2006-01-02 15:04:05"))
	logrus.Infof("整体健康状态: %s", report.OverallHealth)
	logrus.Infof("处理中任务数: %d", report.ProcessingTasks)
	logrus.Infof("🎯 队列健康评分: %d/100", report.HealthScore)

	for _, platform := range report.PlatformReports {
		logrus.Infof("[%s] %s - %d/%d (%.1f%%)",
			platform.Platform, platform.Status,
			platform.QueueSize, platform.BufferSize, platform.UsagePercent)

		for _, issue := range platform.Issues {
			logrus.Warnf("  ⚠️ %s", issue)
		}
	}

	if len(report.Recommendations) > 0 {
		logrus.Info("💡 优化建议:")
		for i, rec := range report.Recommendations {
			logrus.Infof("  %d. %s", i+1, rec)
		}
	}

	return nil
}

// forceCleanup 强制清理队列
func (m *QueueManager) forceCleanup() error {
	logrus.Warn("🧹 执行强制队列清理...")

	cleanedCount := m.cleanupService.ForceCleanupAll(2 * time.Minute)

	logrus.Warnf("🧹 强制清理完成: 清理=%d", cleanedCount)

	return nil
}

// resetQueues 重置队列状态
func (m *QueueManager) resetQueues() error {
	logrus.Warn("🔄 重置队列状态...")

	if err := m.forceCleanup(); err != nil {
		return fmt.Errorf("重置失败: %w", err)
	}

	time.Sleep(2 * time.Second)
	return m.showStatus()
}

// performHealthCheck 执行健康检查
func (m *QueueManager) performHealthCheck() error {
	logrus.Info("🏥 执行队列健康检查...")

	report := m.monitorService.GenerateReport()

	logrus.Infof("健康评分: %d/100", report.HealthScore)
	logrus.Infof("整体状态: %s", report.OverallHealth)

	if report.HealthScore < 50 {
		logrus.Warn("⚠️ 检测到队列健康问题，建议执行以下操作:")
		for _, rec := range report.Recommendations {
			logrus.Warnf("  💡 %s", rec)
		}
		logrus.Warn("🔧 可以执行 'cleanup' 命令进行自动修复")
	}

	return nil
}

// force30MinCleanup 执行30分钟强制清理
func (m *QueueManager) force30MinCleanup() error {
	logrus.Warn("🚨 执行30分钟强制清理...")

	cleanedCount := m.cleanupService.ForceCleanupAll(30 * time.Minute)

	if cleanedCount > 0 {
		logrus.Warnf("🚨 30分钟强制清理完成: 清理了 %d 个长时间运行的任务", cleanedCount)
	} else {
		logrus.Info("✅ 没有发现需要30分钟强制清理的任务")
	}

	return m.showStatus()
}

// GetQueueSummary 获取队列摘要信息
func (m *QueueManager) GetQueueSummary() *QueueSummary {
	summary := &QueueSummary{
		Platforms: make(map[string]any),
		Timestamp: time.Now().Unix(),
	}

	if m.fetcher == nil || m.fetcher.submitters == nil {
		return summary
	}

	totalQueued := 0
	totalCapacity := 0

	for platform, submitter := range m.fetcher.submitters {
		stats := submitter.GetQueueStats()
		totalQueued += stats.QueueSize
		totalCapacity += stats.BufferSize

		summary.Platforms[platform] = map[string]any{
			"queueSize":      stats.QueueSize,
			"bufferSize":     stats.BufferSize,
			"usagePercent":   stats.UsagePercent,
			"availableSlots": stats.AvailableSlots,
		}
	}

	m.fetcher.tasksMutex.RLock()
	processingCount := len(m.fetcher.processingTasks)
	m.fetcher.tasksMutex.RUnlock()

	summary.TotalQueued = totalQueued
	summary.TotalCapacity = totalCapacity
	summary.OverallUsage = float64(totalQueued) / float64(totalCapacity) * 100
	summary.ProcessingTasks = processingCount

	report := m.monitorService.GenerateReport()
	summary.HealthScore = report.HealthScore

	return summary
}
