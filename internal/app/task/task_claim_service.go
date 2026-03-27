package task

import (
	"fmt"
	"time"

	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
)

type TaskClaimService struct {
	fetcher *TaskFetcher
}

func NewTaskClaimService(fetcher *TaskFetcher) *TaskClaimService {
	return &TaskClaimService{fetcher: fetcher}
}

func (s *TaskClaimService) Claim(apiTask *api.ProductImportTaskRespDTO) (string, bool) {
	if s == nil || s.fetcher == nil || apiTask == nil {
		return "", false
	}

	taskID := fmt.Sprintf("%d", apiTask.ID)
	if s.fetcher.isTaskProcessing(taskID) {
		return taskID, false
	}

	if err := model.ValidateTaskStatusTransitionCode(apiTask.Status, model.TaskStatusProcessing); err != nil {
		logger.GetGlobalLogger("app/task").Warnf("任务状态不允许进入处理中，跳过抢占: TaskID=%s, CurrentStatus=%d, Error=%v", taskID, apiTask.Status, err)
		return taskID, false
	}

	s.fetcher.tasksMutex.Lock()
	s.fetcher.processingTasks[taskID] = time.Now()
	s.fetcher.tasksMutex.Unlock()

	logger.GetGlobalLogger("app/task").Debugf("🔒 任务已立即标记为处理中: TaskID=%s", taskID)
	s.fetcher.updateTaskStatusToProcessing(apiTask.ID)
	return taskID, true
}
