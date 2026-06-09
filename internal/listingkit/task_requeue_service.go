package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/listingkit/submission"
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

func normalizeRequeueTaskIDs(req *RequeuePendingTasksRequest) []string {
	if req == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(req.TaskIDs))
	taskIDs := make([]string, 0, len(req.TaskIDs))
	for _, taskID := range req.TaskIDs {
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			continue
		}
		if _, ok := seen[taskID]; ok {
			continue
		}
		seen[taskID] = struct{}{}
		taskIDs = append(taskIDs, taskID)
	}
	return taskIDs
}

func submitTaskWithRetry(submitter TaskSubmitter, taskID string, maxWait time.Duration) error {
	if submitter == nil {
		return ErrTaskRequeueUnavailable
	}
	return submission.RetryEnqueueSubmit(taskID, maxWait, submitter.Submit)
}

func (s *taskRequeueService) currentSubmitter() TaskSubmitter {
	if s == nil || s.taskSubmitter == nil {
		return nil
	}
	return s.taskSubmitter()
}

func (s *service) RequeuePendingTasks(ctx context.Context, req *RequeuePendingTasksRequest) (*RequeuePendingTasksResult, error) {
	return s.taskRequeueOrDefault().RequeuePendingTasks(ctx, req)
}

func (s *service) taskRequeueOrDefault() *taskRequeueService {
	if s == nil {
		return nil
	}
	if s.submission.taskRequeue != nil {
		return s.submission.taskRequeue
	}
	s.submission.taskRequeue = newTaskRequeueService(buildTaskRequeueServiceConfig(s))
	return s.submission.taskRequeue
}
