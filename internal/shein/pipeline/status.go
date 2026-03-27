// Package pipeline 提供SHEIN平台的任务状态更新功能
package pipeline

import (
	"fmt"
	"task-processor/internal/app/taskstatus"
	"task-processor/internal/core/logger"
	"task-processor/internal/model"
)

// TaskStatusUpdater 任务状态更新器
type TaskStatusUpdater struct {
	processor *SheinProcessor
}

// NewTaskStatusUpdater 创建任务状态更新器
func NewTaskStatusUpdater(processor *SheinProcessor) *TaskStatusUpdater {
	return &TaskStatusUpdater{
		processor: processor,
	}
}

// UpdateTaskStatusAsync 异步更新任务状态
func (u *TaskStatusUpdater) UpdateTaskStatusAsync(taskID string, status model.TaskStatus, errorMsg string) {
	u.updateTaskStatusWithMode(taskID, status, errorMsg, false)
}

// UpdateTaskStatusSync 同步更新任务状态
func (u *TaskStatusUpdater) UpdateTaskStatusSync(taskID string, status model.TaskStatus, errorMsg string) error {
	return u.updateTaskStatusWithMode(taskID, status, errorMsg, true)
}

// updateTaskStatusWithMode 更新任务状态（支持同步/异步模式）
func (u *TaskStatusUpdater) updateTaskStatusWithMode(taskID string, status model.TaskStatus, errorMsg string, sync bool) error {
	var id int64
	if _, err := fmt.Sscanf(taskID, "%d", &id); err != nil {
		logger.GetGlobalLogger("shein/pipeline").Errorf("解析任务ID失败: %v", err)
		return fmt.Errorf("解析任务ID失败: %w", err)
	}

	if !status.IsValid() {
		err := fmt.Errorf("非法任务状态: %d", status)
		logger.GetGlobalLogger("shein/pipeline").Warn(err.Error())
		return err
	}

	if err := model.ValidateTaskStatusTransition(model.TaskStatusProcessing, status); err != nil {
		return err
	}

	statusService := taskstatus.NewService("shein/pipeline", func() taskstatus.ImportTaskStatusClient {
		managementClient := u.processor.GetManagementClient()
		if managementClient == nil {
			return nil
		}
		return managementClient.GetImportTaskClient()
	})

	if sync {
		return statusService.TransitionSync(id, model.TaskStatusProcessing, status, errorMsg)
	}

	return statusService.TransitionAsync(id, model.TaskStatusProcessing, status, errorMsg)
}
