package listingkit

import (
	"context"
	"errors"
	"fmt"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
)

const taskRequeueMaxWait = 5 * time.Second

type taskRequeueServiceConfig struct {
	repo          Repository
	taskSubmitter func() TaskSubmitter
}

type taskRequeueService struct {
	repo          Repository
	taskSubmitter func() TaskSubmitter
	runner        *submissiondomain.RequeueService
}

func newTaskRequeueService(config taskRequeueServiceConfig) *taskRequeueService {
	svc := &taskRequeueService{
		repo:          config.repo,
		taskSubmitter: config.taskSubmitter,
	}
	svc.runner = submissiondomain.NewRequeueService(submissiondomain.RequeueServiceConfig{
		LoadTask: func(ctx context.Context, taskID string) (*submissiondomain.RequeueTask, error) {
			task, err := svc.repo.GetTask(ctx, taskID)
			if err != nil {
				return nil, err
			}
			return &submissiondomain.RequeueTask{ID: task.ID, Status: string(task.Status)}, nil
		},
		CurrentSubmitter: func() submissiondomain.RequeueSubmitFunc {
			submitter := svc.currentSubmitter()
			if submitter == nil {
				return nil
			}
			return submitter.Submit
		},
		IsTaskNotFound: func(err error) bool {
			return errors.Is(err, ErrTaskNotFound)
		},
		CanRequeue: func(task *submissiondomain.RequeueTask) (bool, string) {
			if task == nil {
				return false, `task status "" is not processable`
			}
			if TaskStatus(task.Status) != TaskStatusPending {
				return false, fmt.Sprintf("task status %q is not processable", TaskStatus(task.Status))
			}
			return true, ""
		},
		SubmitTask: func(submit submissiondomain.RequeueSubmitFunc, taskID string) error {
			if submit == nil {
				return ErrTaskRequeueUnavailable
			}
			return submitTaskWithRetry(taskRequeueSubmitterFunc(submit), taskID, taskRequeueMaxWait)
		},
		ErrUnavailable:    ErrTaskRequeueUnavailable,
		ErrInvalidRequest: ErrTaskRequeueInvalidRequest,
	})
	return svc
}

func (s *taskRequeueService) RequeuePendingTasks(ctx context.Context, req *RequeuePendingTasksRequest) (*RequeuePendingTasksResult, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("task requeue repository is not configured")
	}
	if s.runner == nil {
		return nil, fmt.Errorf("task requeue runner is not configured")
	}
	result, err := s.runner.RequeueTasks(ctx, &submissiondomain.RequeueRequest{
		TaskIDs: normalizeRequeueTaskIDs(req),
	})
	if err != nil {
		return nil, err
	}
	return adaptSubmissionDomainRequeueResult(result), nil
}

func (s *taskRequeueService) currentSubmitter() TaskSubmitter {
	if s == nil || s.taskSubmitter == nil {
		return nil
	}
	return s.taskSubmitter()
}
