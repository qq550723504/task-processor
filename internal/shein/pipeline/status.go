package pipeline

import (
	"fmt"

	"task-processor/internal/app/taskstatus"
	"task-processor/internal/core/logger"
	"task-processor/internal/model"
)

type TaskStatusUpdater struct {
	processor *SheinProcessor
}

func NewTaskStatusUpdater(processor *SheinProcessor) *TaskStatusUpdater {
	return &TaskStatusUpdater{processor: processor}
}

func (u *TaskStatusUpdater) UpdateTaskStatusAsync(taskID string, status model.TaskStatus, errorMsg string) {
	u.updateTaskStatusWithMode(taskID, nil, status, errorMsg, false)
}

func (u *TaskStatusUpdater) UpdateTaskStatusAsyncWithTask(task *model.Task, status model.TaskStatus, errorMsg string) {
	if task == nil {
		return
	}
	u.updateTaskStatusWithMode(fmt.Sprintf("%d", task.ID), task, status, errorMsg, false)
}

func (u *TaskStatusUpdater) UpdateTaskStatusSync(taskID string, status model.TaskStatus, errorMsg string) error {
	return u.updateTaskStatusWithMode(taskID, nil, status, errorMsg, true)
}

func (u *TaskStatusUpdater) UpdateTaskStatusSyncWithTask(task *model.Task, status model.TaskStatus, errorMsg string) error {
	if task == nil {
		return fmt.Errorf("task is nil")
	}
	return u.updateTaskStatusWithMode(fmt.Sprintf("%d", task.ID), task, status, errorMsg, true)
}

func (u *TaskStatusUpdater) updateTaskStatusWithMode(taskID string, task *model.Task, status model.TaskStatus, errorMsg string, sync bool) error {
	if u == nil || u.processor == nil {
		return fmt.Errorf("task status updater is not initialized")
	}

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

	input := taskstatus.UpdateInput{
		TaskID:       id,
		Status:       status,
		ErrorMessage: errorMsg,
	}
	if task != nil {
		input.RetryCount = &task.RetryCount
		input.Priority = &task.Priority
	}

	statusService := taskstatus.NewService("shein/pipeline", func() taskstatus.ImportTaskStatusClient {
		managementClient := u.processor.GetManagementClient()
		if managementClient == nil {
			return nil
		}
		return managementClient.GetImportTaskClient()
	})

	if sync {
		return statusService.TransitionSyncWithInput(model.TaskStatusProcessing, input)
	}
	return statusService.TransitionAsyncWithInput(model.TaskStatusProcessing, input)
}
