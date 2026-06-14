package submission

import (
	"context"
	"fmt"
	"strings"
)

type RequeueRequest struct {
	TaskIDs []string
}

type RequeueTask struct {
	ID     string
	Status string
}

type RequeueSkip struct {
	TaskID string
	Status string
	Reason string
}

type RequeueFailure struct {
	TaskID string
	Status string
	Error  string
}

type RequeueResult struct {
	RequeuedTaskIDs []string
	Skipped         []RequeueSkip
	Failed          []RequeueFailure
}

type RequeueSubmitFunc func(string) error

type RequeueServiceConfig struct {
	LoadTask          func(context.Context, string) (*RequeueTask, error)
	CurrentSubmitter  func() RequeueSubmitFunc
	IsTaskNotFound    func(error) bool
	CanRequeue        func(*RequeueTask) (bool, string)
	SubmitTask        func(RequeueSubmitFunc, string) error
	ErrUnavailable    error
	ErrInvalidRequest error
}

type RequeueService struct {
	loadTask          func(context.Context, string) (*RequeueTask, error)
	currentSubmitter  func() RequeueSubmitFunc
	isTaskNotFound    func(error) bool
	canRequeue        func(*RequeueTask) (bool, string)
	submitTask        func(RequeueSubmitFunc, string) error
	errUnavailable    error
	errInvalidRequest error
}

func NewRequeueService(config RequeueServiceConfig) *RequeueService {
	return &RequeueService{
		loadTask:          config.LoadTask,
		currentSubmitter:  config.CurrentSubmitter,
		isTaskNotFound:    config.IsTaskNotFound,
		canRequeue:        config.CanRequeue,
		submitTask:        config.SubmitTask,
		errUnavailable:    config.ErrUnavailable,
		errInvalidRequest: config.ErrInvalidRequest,
	}
}

func (s *RequeueService) RequeueTasks(ctx context.Context, req *RequeueRequest) (*RequeueResult, error) {
	if s == nil || s.loadTask == nil {
		return nil, fmt.Errorf("task requeue loader is not configured")
	}
	submitter := s.currentSubmitterOrNil()
	if submitter == nil {
		return nil, s.errUnavailable
	}

	taskIDs := NormalizeRequeueTaskIDs(req)
	if len(taskIDs) == 0 {
		return nil, s.errInvalidRequest
	}

	result := &RequeueResult{
		RequeuedTaskIDs: make([]string, 0, len(taskIDs)),
		Skipped:         make([]RequeueSkip, 0),
		Failed:          make([]RequeueFailure, 0),
	}

	for _, taskID := range taskIDs {
		task, err := s.loadTask(ctx, taskID)
		if err != nil {
			if s.isTaskNotFound != nil && s.isTaskNotFound(err) {
				result.Skipped = append(result.Skipped, RequeueSkip{
					TaskID: taskID,
					Reason: err.Error(),
				})
				continue
			}
			return nil, err
		}

		if allowed, reason := s.canRequeueTask(task); !allowed {
			result.Skipped = append(result.Skipped, RequeueSkip{
				TaskID: task.ID,
				Status: task.Status,
				Reason: reason,
			})
			continue
		}

		if err := s.submitTask(submitter, task.ID); err != nil {
			result.Failed = append(result.Failed, RequeueFailure{
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

func NormalizeRequeueTaskIDs(req *RequeueRequest) []string {
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

func CanRequeueTaskWithStatus(task *RequeueTask, processableStatus string) (bool, string) {
	if task == nil {
		return false, `task status "" is not processable`
	}
	if task.Status != processableStatus {
		return false, fmt.Sprintf("task status %q is not processable", task.Status)
	}
	return true, ""
}

func (s *RequeueService) currentSubmitterOrNil() RequeueSubmitFunc {
	if s == nil || s.currentSubmitter == nil {
		return nil
	}
	return s.currentSubmitter()
}

func (s *RequeueService) canRequeueTask(task *RequeueTask) (bool, string) {
	if s == nil || s.canRequeue == nil {
		return true, ""
	}
	return s.canRequeue(task)
}
