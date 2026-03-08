// Package task 提供任务相关的数据模型定义
package task

import (
	"task-processor/internal/app/worker"
	"time"
)

// QueueTaskInfo 队列任务信息
type QueueTaskInfo struct {
	ID       string        `json:"id"`
	Duration time.Duration `json:"duration"`
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusProcessing TaskStatus = "processing" // 处理中
	TaskStatusStuck      TaskStatus = "stuck"      // 卡住
	TaskStatusExpired    TaskStatus = "expired"    // 过期
	TaskStatusForced     TaskStatus = "forced"     // 强制清理
)

// CleanupStats 清理统计信息
type CleanupStats struct {
	ExpiredTasks   int `json:"expired_tasks"`
	StuckTasks     int `json:"stuck_tasks"`
	ForcedTasks    int `json:"forced_tasks"`
	OrphanedTasks  int `json:"orphaned_tasks"`
	TotalCleaned   int `json:"total_cleaned"`
	RemainingTasks int `json:"remaining_tasks"`
}

// CleanupConfig 清理配置
type CleanupConfig struct {
	CleanupInterval    time.Duration `json:"cleanup_interval"`
	TaskTimeout        time.Duration `json:"task_timeout"`
	StuckTaskThreshold time.Duration `json:"stuck_task_threshold"`
	ForceCleanupAfter  time.Duration `json:"force_cleanup_after"`
	EmergencyThreshold time.Duration `json:"emergency_threshold"`
}

// MonitorReport 监控报告
type MonitorReport struct {
	Timestamp       time.Time        `json:"timestamp"`
	TotalPlatforms  int              `json:"total_platforms"`
	PlatformReports []PlatformReport `json:"platform_reports"`
	ProcessingTasks int              `json:"processing_tasks"`
	OverallHealth   string           `json:"overall_health"`
	HealthScore     int              `json:"health_score"`
	Recommendations []string         `json:"recommendations"`
}

// PlatformReport 平台报告
type PlatformReport struct {
	Platform       string   `json:"platform"`
	QueueSize      int      `json:"queue_size"`
	BufferSize     int      `json:"buffer_size"`
	UsagePercent   float64  `json:"usage_percent"`
	AvailableSlots int      `json:"available_slots"`
	Status         string   `json:"status"`
	Issues         []string `json:"issues"`
}

// QueueSummary 队列摘要
type QueueSummary struct {
	TotalQueued     int                    `json:"total_queued"`
	TotalCapacity   int                    `json:"total_capacity"`
	OverallUsage    float64                `json:"overall_usage"`
	ProcessingTasks int                    `json:"processing_tasks"`
	HealthScore     int                    `json:"health_score"`
	Platforms       map[string]interface{} `json:"platforms"`
	Timestamp       int64                  `json:"timestamp"`
}

// CleanupStrategy 清理策略接口
type CleanupStrategy interface {
	ShouldCleanup(taskID string, duration time.Duration) (bool, string)
	GetPriority() int
}

// MonitorStrategy 监控策略接口
type MonitorStrategy interface {
	EvaluateHealth(stats worker.QueueStats) (string, []string)
	GetThreshold() float64
}

// DefaultCleanupConfig 默认清理配置
func DefaultCleanupConfig() *CleanupConfig {
	return &CleanupConfig{
		CleanupInterval:    2 * time.Minute,
		TaskTimeout:        15 * time.Minute,
		StuckTaskThreshold: 5 * time.Minute,
		ForceCleanupAfter:  30 * time.Minute,
		EmergencyThreshold: 30 * time.Minute,
	}
}

// NewCleanupStats 创建清理统计
func NewCleanupStats() *CleanupStats {
	return &CleanupStats{}
}

// AddCleaned 添加清理统计
func (s *CleanupStats) AddCleaned(status TaskStatus) {
	switch status {
	case TaskStatusExpired:
		s.ExpiredTasks++
	case TaskStatusStuck:
		s.StuckTasks++
	case TaskStatusForced:
		s.ForcedTasks++
	default:
		s.OrphanedTasks++
	}
	s.TotalCleaned++
}

// IsHealthy 检查是否健康
func (r *MonitorReport) IsHealthy() bool {
	return r.HealthScore >= 70
}

// HasIssues 检查是否有问题
func (r *MonitorReport) HasIssues() bool {
	for _, platform := range r.PlatformReports {
		if len(platform.Issues) > 0 {
			return true
		}
	}
	return false
}
