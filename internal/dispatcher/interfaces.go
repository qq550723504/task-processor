// Package dispatcher 提供统一的任务分发功能
package dispatcher

import (
	"context"
	"time"

	"task-processor/internal/model"
)

// PlatformProcessor 平台处理器统一接口
type PlatformProcessor interface {
	// ProcessTask 处理任务
	ProcessTask(ctx context.Context, task *model.UnifiedTask) error

	// Start 启动处理器
	Start(ctx context.Context) error

	// Stop 停止处理器
	Stop(ctx context.Context) error

	// GetStatus 获取处理器状态
	GetStatus() *ProcessorStatus

	// GetPlatformName 获取平台名称
	GetPlatformName() string

	// CanHandle 检查是否可以处理指定任务
	CanHandle(task *model.UnifiedTask) bool
}

// ProcessorStatus 处理器状态
type ProcessorStatus struct {
	Name           string                 `json:"name"`
	Platform       string                 `json:"platform"`
	Status         string                 `json:"status"` // running, stopped, error
	StartTime      time.Time              `json:"start_time"`
	LastActiveTime time.Time              `json:"last_active_time"`
	TasksProcessed int64                  `json:"tasks_processed"`
	TasksSucceeded int64                  `json:"tasks_succeeded"`
	TasksFailed    int64                  `json:"tasks_failed"`
	AvailableSlots int                    `json:"available_slots"`
	Metrics        map[string]interface{} `json:"metrics"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
}

// TaskDispatcher 任务分发器接口
type TaskDispatcher interface {
	// RegisterProcessor 注册平台处理器
	RegisterProcessor(processor PlatformProcessor) error

	// UnregisterProcessor 注销平台处理器
	UnregisterProcessor(platform string) error

	// DispatchTask 分发任务
	DispatchTask(ctx context.Context, task *model.UnifiedTask) error

	// DispatchBatch 批量分发任务
	DispatchBatch(ctx context.Context, tasks []*model.UnifiedTask) error

	// Start 启动分发器
	Start(ctx context.Context) error

	// Stop 停止分发器
	Stop(ctx context.Context) error

	// GetProcessorStatus 获取处理器状态
	GetProcessorStatus(platform string) (*ProcessorStatus, error)

	// GetAllProcessorStatus 获取所有处理器状态
	GetAllProcessorStatus() map[string]*ProcessorStatus

	// GetSupportedPlatforms 获取支持的平台列表
	GetSupportedPlatforms() []string
}

// RouterRule 路由规则接口
type RouterRule interface {
	// Match 检查任务是否匹配此规则
	Match(task *model.UnifiedTask) bool

	// GetTargetPlatform 获取目标平台
	GetTargetPlatform(task *model.UnifiedTask) string

	// GetPriority 获取规则优先级（数字越大优先级越高）
	GetPriority() int
}

// TaskRouter 任务路由器接口
type TaskRouter interface {
	// AddRule 添加路由规则
	AddRule(rule RouterRule) error

	// RemoveRule 移除路由规则
	RemoveRule(ruleID string) error

	// Route 路由任务到目标平台
	Route(task *model.UnifiedTask) (string, error)

	// GetRules 获取所有路由规则
	GetRules() []RouterRule
}

// ProcessorRegistry 处理器注册表接口
type ProcessorRegistry interface {
	// Register 注册处理器
	Register(processor PlatformProcessor) error

	// Unregister 注销处理器
	Unregister(platform string) error

	// Get 获取处理器
	Get(platform string) (PlatformProcessor, error)

	// GetAll 获取所有处理器
	GetAll() map[string]PlatformProcessor

	// List 列出所有平台名称
	List() []string

	// Count 获取处理器数量
	Count() int
}

// TaskMetrics 任务指标
type TaskMetrics struct {
	TotalTasks          int64         `json:"total_tasks"`
	SuccessfulTasks     int64         `json:"successful_tasks"`
	FailedTasks         int64         `json:"failed_tasks"`
	AverageTime         time.Duration `json:"average_time"`
	ThroughputPerSecond float64       `json:"throughput_per_second"`
	LastUpdateTime      time.Time     `json:"last_update_time"`
}

// MetricsCollector 指标收集器接口
type MetricsCollector interface {
	// RecordTaskStart 记录任务开始
	RecordTaskStart(platform string, taskID string)

	// RecordTaskSuccess 记录任务成功
	RecordTaskSuccess(platform string, taskID string, duration time.Duration)

	// RecordTaskFailure 记录任务失败
	RecordTaskFailure(platform string, taskID string, duration time.Duration, err error)

	// GetMetrics 获取指标
	GetMetrics(platform string) *TaskMetrics

	// GetAllMetrics 获取所有平台指标
	GetAllMetrics() map[string]*TaskMetrics
}
