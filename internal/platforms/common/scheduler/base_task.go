// Package scheduler 提供平台通用的任务基础实现
package scheduler

import (
	"fmt"
	"time"

	appscheduler "task-processor/internal/app/scheduler"
)

// BaseTask 基础任务实现
// 提供所有平台任务的通用功能
type BaseTask struct {
	id       string
	taskType appscheduler.TaskType
	platform string
	tenantID int64
	storeID  int64
	interval time.Duration
	status   appscheduler.TaskStatus
}

// NewBaseTask 创建基础任务
func NewBaseTask(config appscheduler.TaskConfig) *BaseTask {
	taskID := fmt.Sprintf("%s:%s:%d:%d",
		config.Platform,
		config.TaskType,
		config.TenantID,
		config.StoreID,
	)

	return &BaseTask{
		id:       taskID,
		taskType: config.TaskType,
		platform: config.Platform,
		tenantID: config.TenantID,
		storeID:  config.StoreID,
		interval: config.Interval,
		status:   appscheduler.TaskStatusStopped,
	}
}

// GetID 获取任务ID
func (t *BaseTask) GetID() string {
	return t.id
}

// GetType 获取任务类型
func (t *BaseTask) GetType() appscheduler.TaskType {
	return t.taskType
}

// GetPlatform 获取平台名称
func (t *BaseTask) GetPlatform() string {
	return t.platform
}

// GetStoreID 获取店铺ID
func (t *BaseTask) GetStoreID() int64 {
	return t.storeID
}

// GetInterval 获取执行间隔
func (t *BaseTask) GetInterval() time.Duration {
	return t.interval
}

// GetStatus 获取任务状态
func (t *BaseTask) GetStatus() appscheduler.TaskStatus {
	return t.status
}

// SetStatus 设置任务状态
func (t *BaseTask) SetStatus(status appscheduler.TaskStatus) {
	t.status = status
}

// GetTenantID 获取租户ID
func (t *BaseTask) GetTenantID() int64 {
	return t.tenantID
}
