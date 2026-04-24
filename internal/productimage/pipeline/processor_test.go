package pipeline

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/infra/worker"
	productimage "task-processor/internal/productimage"

	"github.com/sirupsen/logrus"
)

type stubImageService struct {
	err error
}

func (s *stubImageService) ProcessImages(_ context.Context, _ *productimage.Task) (*productimage.ImageProcessResult, error) {
	return nil, s.err
}

func (s *stubImageService) SetTaskSubmitter(_ productimage.TaskSubmitter) {}

type stubTaskRepo struct {
	task                 *productimage.Task
	incrementRetryCalled bool
	prepareRetryCalled   bool
	markFailedCalled     bool
	lastFailedMessage    string
}

func (r *stubTaskRepo) CreateTask(context.Context, *productimage.Task) error { return nil }
func (r *stubTaskRepo) GetTask(context.Context, string) (*productimage.Task, error) {
	if r.task == nil {
		return nil, productimage.ErrTaskNotFound
	}
	return r.task, nil
}
func (r *stubTaskRepo) MarkProcessing(context.Context, string) error { return nil }
func (r *stubTaskRepo) MarkCompleted(context.Context, string, *productimage.ImageProcessResult) error {
	return nil
}
func (r *stubTaskRepo) MarkNeedsReview(context.Context, string, *productimage.ImageProcessResult, string) error {
	return nil
}
func (r *stubTaskRepo) MarkRejected(context.Context, string, string) error { return nil }
func (r *stubTaskRepo) MarkFailed(_ context.Context, _ string, errorMsg string) error {
	r.markFailedCalled = true
	r.lastFailedMessage = errorMsg
	return nil
}
func (r *stubTaskRepo) PrepareRetry(context.Context, string) error {
	r.prepareRetryCalled = true
	return nil
}
func (r *stubTaskRepo) UpdateTaskStatus(context.Context, string, productimage.TaskStatus) error {
	return nil
}
func (r *stubTaskRepo) UpdateTaskError(context.Context, string, string) error { return nil }
func (r *stubTaskRepo) SaveTaskResult(context.Context, string, *productimage.ImageProcessResult) error {
	return nil
}
func (r *stubTaskRepo) IncrementRetryCount(context.Context, string) error {
	r.incrementRetryCalled = true
	return nil
}
func (r *stubTaskRepo) ResetForRetry(context.Context, string) error { return nil }

type stubSubmitter struct {
	submittedTaskID string
	err             error
}

func (s *stubSubmitter) Submit(taskID string) error {
	s.submittedTaskID = taskID
	return s.err
}

func TestProcessorProcessTaskReturnsNilWhenRetryWasScheduled(t *testing.T) {
	t.Parallel()

	task := &productimage.Task{
		ID:         "task-retry",
		Status:     productimage.TaskStatusPending,
		RetryCount: 0,
		Request:    &productimage.ImageProcessRequest{Marketplace: "shein"},
	}
	repo := &stubTaskRepo{task: task}
	service := &stubImageService{err: errors.New("transient provider timeout")}
	submitter := &stubSubmitter{}

	processor, err := NewProcessor(service, repo, logrus.New(), 3)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}
	processor.SetTaskSubmitter(submitter)

	err = processor.ProcessTask(context.Background(), worker.WorkerJob{TaskData: task.ID})
	if err != nil {
		t.Fatalf("ProcessTask() error = %v, want nil after retry scheduled", err)
	}
	if !repo.incrementRetryCalled {
		t.Fatalf("expected retry count increment")
	}
	if !repo.prepareRetryCalled {
		t.Fatalf("expected prepare retry to be called")
	}
	if submitter.submittedTaskID != task.ID {
		t.Fatalf("Submit() task ID = %q, want %q", submitter.submittedTaskID, task.ID)
	}
	if repo.markFailedCalled {
		t.Fatalf("did not expect MarkFailed() after successful resubmit")
	}
}

func TestProcessorProcessTaskReturnsErrorWhenRetrySubmitFails(t *testing.T) {
	t.Parallel()

	task := &productimage.Task{
		ID:         "task-submit-fail",
		Status:     productimage.TaskStatusPending,
		RetryCount: 0,
		Request:    &productimage.ImageProcessRequest{Marketplace: "shein"},
	}
	repo := &stubTaskRepo{task: task}
	service := &stubImageService{err: errors.New("transient provider timeout")}
	submitter := &stubSubmitter{err: errors.New("queue unavailable")}

	processor, err := NewProcessor(service, repo, logrus.New(), 3)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}
	processor.SetTaskSubmitter(submitter)

	err = processor.ProcessTask(context.Background(), worker.WorkerJob{TaskData: task.ID})
	if err == nil {
		t.Fatalf("ProcessTask() error = nil, want submit failure")
	}
	if !repo.markFailedCalled {
		t.Fatalf("expected MarkFailed() after submit failure")
	}
}
