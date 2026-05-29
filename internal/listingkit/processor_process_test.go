package listingkit

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/infra/worker"
)

type stubProcessorService struct {
	result   *ListingKitResult
	err      error
	calls    int
	lastCtx  context.Context
	lastTask *Task
}

type stubProcessorRepo struct {
	task                 *Task
	getTaskErr           error
	incrementRetryCalls  int
	prepareRetryCalls    int
	incrementRetryTaskID string
	prepareRetryTaskID   string
}

type stubProcessorSubmitter struct {
	calls      int
	lastTaskID string
	err        error
}

func (s *stubProcessorService) ProcessListingKit(ctx context.Context, task *Task) (*ListingKitResult, error) {
	s.calls++
	s.lastCtx = ctx
	s.lastTask = task
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func (r *stubProcessorRepo) CreateTask(context.Context, *Task) error { return nil }

func (r *stubProcessorRepo) GetTask(context.Context, string) (*Task, error) {
	if r.getTaskErr != nil {
		return nil, r.getTaskErr
	}
	if r.task == nil {
		return nil, ErrTaskNotFound
	}
	copied := *r.task
	return &copied, nil
}

func (r *stubProcessorRepo) ListTasks(context.Context, *TaskListQuery) ([]Task, int64, error) {
	return nil, 0, nil
}

func (r *stubProcessorRepo) MarkProcessing(context.Context, string) error { return nil }
func (r *stubProcessorRepo) MarkCompleted(context.Context, string, *ListingKitResult) error {
	return nil
}
func (r *stubProcessorRepo) MarkNeedsReview(context.Context, string, *ListingKitResult, string) error {
	return nil
}
func (r *stubProcessorRepo) MarkFailed(context.Context, string, string) error { return nil }
func (r *stubProcessorRepo) PrepareRetry(_ context.Context, taskID string) error {
	r.prepareRetryCalls++
	r.prepareRetryTaskID = taskID
	return nil
}
func (r *stubProcessorRepo) IncrementRetryCount(_ context.Context, taskID string) error {
	r.incrementRetryCalls++
	r.incrementRetryTaskID = taskID
	return nil
}
func (r *stubProcessorRepo) SaveTaskResult(context.Context, string, *ListingKitResult) error {
	return nil
}

func (s *stubProcessorSubmitter) Submit(taskID string) error {
	s.calls++
	s.lastTaskID = taskID
	return s.err
}

func TestProcessorProcessTaskSkipsNonPendingTasks(t *testing.T) {
	t.Parallel()

	svc := &stubProcessorService{}
	repo := &stubProcessorRepo{task: &Task{ID: "task-1", Status: TaskStatusCompleted}}
	processor, err := NewProcessor(svc, repo, logrus.New(), 2)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	err = processor.ProcessTask(context.Background(), worker.WorkerJob{TaskData: "task-1"})
	if err != nil {
		t.Fatalf("ProcessTask() error = %v, want nil skip", err)
	}
	if svc.calls != 0 {
		t.Fatalf("service calls = %d, want 0", svc.calls)
	}
}

func TestProcessorProcessTaskTreatsErrTaskNotPendingAsSkip(t *testing.T) {
	t.Parallel()

	svc := &stubProcessorService{err: ErrTaskNotPending}
	repo := &stubProcessorRepo{task: &Task{ID: "task-2", Status: TaskStatusPending}}
	processor, err := NewProcessor(svc, repo, logrus.New(), 2)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	err = processor.ProcessTask(context.Background(), worker.WorkerJob{TaskData: "task-2"})
	if err != nil {
		t.Fatalf("ProcessTask() error = %v, want nil skip", err)
	}
	if repo.incrementRetryCalls != 0 || repo.prepareRetryCalls != 0 {
		t.Fatalf("retry calls = increment:%d prepare:%d, want 0", repo.incrementRetryCalls, repo.prepareRetryCalls)
	}
}

func TestProcessorProcessTaskSchedulesRetryForRetryableFailure(t *testing.T) {
	t.Parallel()

	svc := &stubProcessorService{err: errors.New("workflow failed")}
	repo := &stubProcessorRepo{task: &Task{ID: "task-3", Status: TaskStatusPending, RetryCount: 0}}
	submitter := &stubProcessorSubmitter{}
	processor, err := NewProcessor(svc, repo, logrus.New(), 2)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}
	processor.SetTaskSubmitter(submitter)

	err = processor.ProcessTask(context.Background(), worker.WorkerJob{TaskData: "task-3"})
	if err == nil {
		t.Fatal("ProcessTask() error = nil, want retryable failure")
	}
	if repo.incrementRetryCalls != 1 || repo.incrementRetryTaskID != "task-3" {
		t.Fatalf("IncrementRetryCount = %d for %q, want 1 for task-3", repo.incrementRetryCalls, repo.incrementRetryTaskID)
	}
	if repo.prepareRetryCalls != 1 || repo.prepareRetryTaskID != "task-3" {
		t.Fatalf("PrepareRetry = %d for %q, want 1 for task-3", repo.prepareRetryCalls, repo.prepareRetryTaskID)
	}
	if submitter.calls != 1 || submitter.lastTaskID != "task-3" {
		t.Fatalf("Submit = %d for %q, want 1 for task-3", submitter.calls, submitter.lastTaskID)
	}
}

func TestProcessorProcessTaskDoesNotRetryWhenMaxRetriesReached(t *testing.T) {
	t.Parallel()

	svc := &stubProcessorService{err: errors.New("workflow failed")}
	repo := &stubProcessorRepo{task: &Task{ID: "task-4", Status: TaskStatusPending, RetryCount: 2}}
	submitter := &stubProcessorSubmitter{}
	processor, err := NewProcessor(svc, repo, logrus.New(), 2)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}
	processor.SetTaskSubmitter(submitter)

	err = processor.ProcessTask(context.Background(), worker.WorkerJob{TaskData: "task-4"})
	if err == nil {
		t.Fatal("ProcessTask() error = nil, want failure")
	}
	if repo.incrementRetryCalls != 0 || repo.prepareRetryCalls != 0 || submitter.calls != 0 {
		t.Fatalf("retry path = increment:%d prepare:%d submit:%d, want all 0", repo.incrementRetryCalls, repo.prepareRetryCalls, submitter.calls)
	}
}

func TestProcessorProcessTaskInjectsTenantAndIdentityBeforeServiceExecution(t *testing.T) {
	t.Parallel()

	svc := &stubProcessorService{result: &ListingKitResult{}}
	repo := &stubProcessorRepo{task: &Task{
		ID:       "task-5",
		Status:   TaskStatusPending,
		TenantID: "tenant-a",
		Request:  &GenerateRequest{UserID: "user-a"},
	}}
	processor, err := NewProcessor(svc, repo, logrus.New(), 2)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	err = processor.ProcessTask(context.Background(), worker.WorkerJob{TaskData: "task-5"})
	if err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if svc.calls != 1 || svc.lastCtx == nil {
		t.Fatalf("service calls = %d, ctx = %v, want one call with ctx", svc.calls, svc.lastCtx)
	}
	if got := TenantIDFromContext(svc.lastCtx); got != "tenant-a" {
		t.Fatalf("tenant in context = %q, want tenant-a", got)
	}
	identity := openaiclient.IdentityFromContext(svc.lastCtx)
	if identity.TenantID != "tenant-a" || identity.UserID != "user-a" {
		t.Fatalf("identity = %+v, want tenant-a/user-a", identity)
	}
}
