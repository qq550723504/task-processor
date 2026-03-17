// package scheduler 提供SHEIN平台任务的基础实现
package scheduler

import (
	appscheduler "task-processor/internal/app/scheduler"
	commonscheduler "task-processor/internal/taskbase"
)

// BaseTask SHEIN平台基础任务
// 使用通用基类实现
type BaseTask = commonscheduler.BaseTask

// NewBaseTask 创建基础任务
func NewBaseTask(config appscheduler.TaskConfig) *BaseTask {
	return commonscheduler.NewBaseTask(config)
}
