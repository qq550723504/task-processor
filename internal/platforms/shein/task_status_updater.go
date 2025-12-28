// Package shein 提供SHEIN平台的任务状态更新功能
package shein

import (
	"fmt"
	management_api "task-processor/internal/common/management/api"
	"task-processor/internal/model"
	"time"

	"github.com/sirupsen/logrus"
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
		logrus.Errorf("解析任务ID失败: %v", err)
		return fmt.Errorf("解析任务ID失败: %w", err)
	}

	importTaskClient := u.processor.GetManagementClient().GetImportTaskClient()
	if importTaskClient == nil {
		err := fmt.Errorf("导入任务客户端未初始化")
		logrus.Warn(err.Error())
		return err
	}

	req := &management_api.ProductImportTaskUpdateReqDTO{
		ID:           id,
		Status:       status.Int16(),
		ErrorMessage: errorMsg,
	}

	updateFunc := func() error {
		maxRetries := 5
		var lastErr error

		for i := 0; i < maxRetries; i++ {
			if err := importTaskClient.UpdateTaskStatus(req); err != nil {
				lastErr = err
				if i < maxRetries-1 {
					retryDelay := time.Second * time.Duration(i+1)
					logrus.Warnf("更新任务状态到API失败 (TaskID: %s, Status: %s, 重试 %d/%d): %v, %v后重试",
						taskID, status.String(), i+1, maxRetries, err, retryDelay)
					time.Sleep(retryDelay)
					continue
				}
				logrus.Errorf("⚠️ 更新任务状态到API失败，已达最大重试次数 (TaskID: %s, Status: %s): %v",
					taskID, status.String(), err)
				return fmt.Errorf("更新任务状态失败: %w", err)
			} else {
				logrus.Infof("✅ 成功更新任务状态到API (TaskID: %s, Status: %s)", taskID, status.String())
				return nil
			}
		}
		return lastErr
	}

	if sync {
		return updateFunc()
	} else {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("异步更新任务状态goroutine panic (TaskID: %s, Status: %s): %v", taskID, status.String(), r)
				}
			}()
			if err := updateFunc(); err != nil {
				logrus.Errorf("异步更新任务状态最终失败 (TaskID: %s, Status: %s): %v", taskID, status.String(), err)
			}
		}()
		return nil
	}
}
