package listingkit

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const taskRequeueMaxWait = 5 * time.Second

type taskRequeueServiceConfig struct {
	repo          Repository
	taskSubmitter func() TaskSubmitter
}

type taskRequeueService struct {
	repo          Repository
	taskSubmitter func() TaskSubmitter
}

func newTaskRequeueService(config taskRequeueServiceConfig) *taskRequeueService {
	return &taskRequeueService{
		repo:          config.repo,
		taskSubmitter: config.taskSubmitter,
	}
}

func (s *taskRequeueService) RequeuePendingTasks(ctx context.Context, req *RequeuePendingTasksRequest) (*RequeuePendingTasksResult, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("task requeue repository is not configured")
	}
	submitter := s.currentSubmitter()
	if submitter == nil {
		return nil, ErrTaskRequeueUnavailable
	}

	taskIDs := normalizeRequeueTaskIDs(req)
	if len(taskIDs) == 0 {
		return nil, ErrTaskRequeueInvalidRequest
	}

	result := &RequeuePendingTasksResult{
		RequeuedTaskIDs: make([]string, 0, len(taskIDs)),
		Skipped:         make([]TaskRequeueSkip, 0),
		Failed:          make([]TaskRequeueFailure, 0),
	}

	for _, taskID := range taskIDs {
		task, err := s.repo.GetTask(ctx, taskID)
		if err != nil {
			if errors.Is(err, ErrTaskNotFound) {
				result.Skipped = append(result.Skipped, TaskRequeueSkip{
					TaskID: taskID,
					Reason: ErrTaskNotFound.Error(),
				})
				continue
			}
			return nil, err
		}
		if task.Status != TaskStatusPending {
			result.Skipped = append(result.Skipped, TaskRequeueSkip{
				TaskID: task.ID,
				Status: task.Status,
				Reason: fmt.Sprintf("task status %q is not processable", task.Status),
			})
			continue
		}
		if err := submitTaskWithRetry(submitter, task.ID, taskRequeueMaxWait); err != nil {
			result.Failed = append(result.Failed, TaskRequeueFailure{
				TaskID: task.ID,
				Status: task.Status,
				Error:  err.Error(),
			})
			continue
		}
		result.RequeuedTaskIDs = append(result.RequeuedTaskIDs, task.ID)
	}

	return result, nil
}

func (s *taskRequeueService) currentSubmitter() TaskSubmitter {
	if s == nil || s.taskSubmitter == nil {
		return nil
	}
	return s.taskSubmitter()
}
