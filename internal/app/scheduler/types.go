// Package scheduler 提供统一的任务调度功能
package scheduler

import (
	"context"
	"time"
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypePricing     TaskType = "pricing"     // 核价任务
	TaskTypeProductSync TaskType = "productSync" // 同步任务
	TaskTypeInventory   TaskType = "inventory"   // 库存任务
	TaskTypeActivity    TaskType = "activity"    // 活动任务
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusRunning TaskStatus = "running"
	TaskStatusStopped TaskStatus = "stopped"
	TaskStatusError   TaskStatus = "error"
)

// Task 任务接口
type Task interface {
	// GetID 获取任务ID
	GetID() string

	// GetType 获取任务类型
	GetType() TaskType

	// GetPlatform 获取平台名称
	GetPlatform() string

	// GetStoreID 获取店铺ID
	GetStoreID() int64

	// Execute 执行任务
	Execute(ctx context.Context) error

	// GetInterval 获取执行间隔
	GetInterval() time.Duration

	// GetStatus 获取任务状态
	GetStatus() TaskStatus
}

// TaskConfig 任务配置
type TaskConfig struct {
	TaskType  TaskType      // 任务类型
	Platform  string        // 平台名称
	TenantID  int64         // 租户ID
	StoreID   int64         // 店铺ID
	Interval  time.Duration // 执行间隔
	Enabled   bool          // 是否启用
	AutoStart bool          // 是否自动启动
}

// TaskFactory 任务工厂接口
type TaskFactory interface {
	// CreateTask 创建任务
	CreateTask(ctx context.Context, config TaskConfig) (Task, error)

	// SupportedPlatform 支持的平台
	SupportedPlatform() string

	// SupportedTaskTypes 支持的任务类型
	SupportedTaskTypes() []TaskType
}

// TaskResult 任务执行结果
type TaskResult struct {
	TaskID       string        // 任务ID
	TaskType     TaskType      // 任务类型
	Platform     string        // 平台名称
	StartTime    time.Time     // 开始时间
	EndTime      time.Time     // 结束时间
	Duration     time.Duration // 执行时长
	Success      bool          // 是否成功
	ErrorMessage string        // 错误信息
	Stats        any   // 统计信息
}
