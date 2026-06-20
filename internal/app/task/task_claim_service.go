package task

import (
	"fmt"
	"time"

	"task-processor/internal/core/logger"
	"task-processor/internal/model"
)

type TaskClaimService struct {
	fetcher *TaskFetcher
}

func NewTaskClaimService(fetcher *TaskFetcher) *TaskClaimService {
	return &TaskClaimService{fetcher: fetcher}
}

func (s *TaskClaimService) Claim(task *ImportTaskRecord) (string, bool) {
	if s == nil || s.fetcher == nil || task == nil {
		return "", false
	}

	taskID := fmt.Sprintf("%d", task.ID)
	if s.fetcher.isTaskProcessing(taskID) {
		return taskID, false
	}

	if err := model.ValidateTaskStatusTransitionCode(task.Status, model.TaskStatusProcessing); err != nil {
		logger.GetGlobalLogger("app/task").Warnf(
			"任务状态不允许进入 processing，跳过抢占: TaskID=%s, CurrentStatus=%d, CurrentStatusKey=%s, CurrentCanonicalStatus=%s, Error=%v",
			taskID,
			task.Status,
			task.StatusKey,
			task.CanonicalStatus,
			err,
		)
		return taskID, false
	}

	s.fetcher.tasksMutex.Lock()
	s.fetcher.processingTasks[taskID] = time.Now()
	s.fetcher.tasksMutex.Unlock()

	if err := s.fetcher.updateTaskStatusToProcessing(task.ID, task.Status); err != nil {
		s.fetcher.rollbackProcessingStatus(taskID)
		logger.GetGlobalLogger("app/task").WithError(err).Warnf("任务远端 claim 失败，跳过抢占: TaskID=%s", taskID)
		return taskID, false
	}

	fromStatus, err := model.ParseTaskStatus(task.Status)
	if err != nil {
		s.fetcher.rollbackClaimState(taskID, task, "failed to parse original status for claim journal")
		logger.GetGlobalLogger("app/task").WithError(err).Warnf(
			"解析原始任务状态失败，回滚 claim: TaskID=%s, CurrentStatus=%d, CurrentStatusKey=%s, CurrentCanonicalStatus=%s",
			taskID,
			task.Status,
			task.StatusKey,
			task.CanonicalStatus,
		)
		return taskID, false
	}

	if err := s.fetcher.recordClaimJournalEntry(task.ID, &ClaimJournalEntry{
		TaskID:       task.ID,
		ClaimedAt:    time.Now(),
		FromStatus:   fromStatus,
		ProductID:    task.ProductID,
		StoreID:      task.StoreID,
		Platform:     task.Platform,
		ErrorMessage: task.ErrorMessage,
	}); err != nil {
		s.fetcher.rollbackClaimState(taskID, task, "failed to persist claim journal")
		logger.GetGlobalLogger("app/task").WithError(err).Warnf("claim journal 持久化失败，回滚 claim: TaskID=%s", taskID)
		return taskID, false
	}

	logger.GetGlobalLogger("app/task").Debugf(
		"任务已成功 claim 并标记为 processing: TaskID=%s, FromStatus=%s, FromStatusKey=%s, FromCanonicalStatus=%s",
		taskID,
		fromStatus.String(),
		task.StatusKey,
		task.CanonicalStatus,
	)
	return taskID, true
}
