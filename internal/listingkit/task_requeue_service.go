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
	repo             Repository
	taskSubmitter    func() TaskSubmitter
	standardWorkflow func() (StandardProductWorkflowClient, bool)
}

type taskRequeueService struct {
	repo             Repository
	taskSubmitter    func() TaskSubmitter
	standardWorkflow func() (StandardProductWorkflowClient, bool)
	runner           *submissiondomain.RequeueService
}

func newTaskRequeueService(config taskRequeueServiceConfig) *taskRequeueService {
	svc := &taskRequeueService{
		repo:             config.repo,
		taskSubmitter:    config.taskSubmitter,
		standardWorkflow: config.standardWorkflow,
	}
	wiring := buildTaskRequeueRunnerWiring(svc)
	svc.runner = submissiondomain.NewRequeueService(submissiondomain.RequeueServiceConfig{
		LoadTask:          wiring.loadTask,
		CurrentSubmitter:  wiring.currentSubmitter,
		IsTaskNotFound:    wiring.isTaskNotFound,
		CanRequeue:        wiring.canRequeue,
		SubmitTask:        wiring.submitTask,
		ErrUnavailable:    ErrTaskRequeueUnavailable,
		ErrInvalidRequest: ErrTaskRequeueInvalidRequest,
	})
	return svc
}

type taskRequeueRunnerWiring struct {
	svc *taskRequeueService
}

func buildTaskRequeueRunnerWiring(svc *taskRequeueService) taskRequeueRunnerWiring {
	return taskRequeueRunnerWiring{svc: svc}
}

func (w taskRequeueRunnerWiring) loadTask(ctx context.Context, taskID string) (*submissiondomain.RequeueTask, error) {
	task, err := w.svc.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return &submissiondomain.RequeueTask{ID: task.ID, Status: string(task.Status)}, nil
}

func (w taskRequeueRunnerWiring) currentSubmitter() submissiondomain.RequeueSubmitFunc {
	if client, enabled := w.svc.currentStandardWorkflow(); enabled && client != nil {
		return func(string) error { return nil }
	}
	submitter := w.svc.currentSubmitter()
	if submitter == nil {
		return nil
	}
	return submitter.Submit
}

func (w taskRequeueRunnerWiring) isTaskNotFound(err error) bool {
	return errors.Is(err, ErrTaskNotFound)
}

func (w taskRequeueRunnerWiring) canRequeue(task *submissiondomain.RequeueTask) (bool, string) {
	return submissiondomain.CanRequeueTaskWithStatus(task, string(TaskStatusPending))
}

func (w taskRequeueRunnerWiring) submitTask(ctx context.Context, submit submissiondomain.RequeueSubmitFunc, taskID string) error {
	if client, enabled := w.svc.currentStandardWorkflow(); enabled && client != nil {
		return client.StartStandardProduct(ctx, StandardProductWorkflowStartInput{
			TaskID:      taskID,
			RequestedAt: time.Now().UTC(),
		})
	}
	if submit == nil {
		return ErrTaskRequeueUnavailable
	}
	return submissiondomain.RetryEnqueueSubmit(taskID, taskRequeueMaxWait, submit)
}

func (s *taskRequeueService) RequeuePendingTasks(ctx context.Context, req *RequeuePendingTasksRequest) (*RequeuePendingTasksResult, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("task requeue repository is not configured")
	}
	if s.runner == nil {
		return nil, fmt.Errorf("task requeue runner is not configured")
	}
	result, err := s.runner.RequeueTasks(ctx, &submissiondomain.RequeueRequest{
		TaskIDs: submissiondomain.NormalizeRequeueTaskIDs(&submissiondomain.RequeueRequest{
			TaskIDs: req.TaskIDs,
		}),
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

func (s *taskRequeueService) currentStandardWorkflow() (StandardProductWorkflowClient, bool) {
	if s == nil || s.standardWorkflow == nil {
		return nil, false
	}
	return s.standardWorkflow()
}
