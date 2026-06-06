package listingkit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/infra/worker"
	"task-processor/internal/productenrich"
)

func TestCreateGenerateTaskHandlesWorkerQueueFullAsAsyncRetry(t *testing.T) {
	t.Parallel()

	repo := newRetryableLifecycleTestRepo()
	submitter := &retryableLifecycleSequencedSubmitter{
		results: []error{worker.ErrQueueFull, nil},
		calls:   make(chan int, 2),
	}
	svc := newTaskLifecycleService(taskLifecycleServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return submitter
		},
	})

	task, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Text:      "queue backpressure",
		Platforms: []string{"amazon"},
	})
	if err != nil {
		t.Fatalf("CreateGenerateTask() error = %v, want nil when queue is temporarily full", err)
	}
	if task == nil {
		t.Fatal("CreateGenerateTask() task = nil, want persisted task")
	}

	waitForRetryableLifecycleSubmitAttempt(t, submitter.calls, "initial submit attempt")
	waitForRetryableLifecycleSubmitAttempt(t, submitter.calls, "async retry attempt")

	if repo.blockedTaskID != "" {
		t.Fatalf("MarkBlockedRetryable task ID = %q, want empty for worker.ErrQueueFull async retry", repo.blockedTaskID)
	}
	if repo.failedTaskID != "" {
		t.Fatalf("MarkFailed task ID = %q, want empty for worker.ErrQueueFull async retry", repo.failedTaskID)
	}

	stored, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("GetTask() error = %v", getErr)
	}
	if stored.Status != TaskStatusPending {
		t.Fatalf("stored status = %q, want %q while async retry succeeds without terminalization", stored.Status, TaskStatusPending)
	}
	if stored.RetryableBlock != nil {
		t.Fatalf("stored RetryableBlock = %+v, want nil for worker.ErrQueueFull async retry", stored.RetryableBlock)
	}
	if stored.Error != "" {
		t.Fatalf("stored error = %q, want empty while queue-full retry resolves asynchronously", stored.Error)
	}
}

func TestCreateGenerateTaskAsyncRetryMarksRetryableFailureWhenRetryStops(t *testing.T) {
	t.Parallel()

	repo := newRetryableLifecycleTestRepo()
	submitter := &retryableLifecycleSequencedSubmitter{
		results: []error{worker.ErrQueueFull, errors.New("工作队列已满")},
		calls:   make(chan int, 2),
	}
	svc := newTaskLifecycleService(taskLifecycleServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return submitter
		},
	})

	task, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Text:      "queue retry stops on retryable failure",
		Platforms: []string{"amazon"},
	})
	if err != nil {
		t.Fatalf("CreateGenerateTask() error = %v, want nil while async retry owns the queue-full recovery", err)
	}
	if task == nil {
		t.Fatal("CreateGenerateTask() task = nil, want persisted task")
	}

	waitForRetryableLifecycleSubmitAttempt(t, submitter.calls, "initial submit attempt")
	waitForRetryableLifecycleSubmitAttempt(t, submitter.calls, "async retry attempt")

	stored := waitForRetryableLifecycleTaskStatus(t, repo, task.ID, TaskStatusBlockedRetryable)
	if repo.blockedTaskID != task.ID {
		t.Fatalf("MarkBlockedRetryable task ID = %q, want %q", repo.blockedTaskID, task.ID)
	}
	if repo.failedTaskID != "" {
		t.Fatalf("MarkFailed task ID = %q, want empty when retry stops on retryable failure", repo.failedTaskID)
	}
	if stored.RetryableBlock == nil {
		t.Fatal("stored RetryableBlock = nil, want retry metadata")
	}
	if stored.RetryableBlock.ReasonCode != retryableBlockReasonCodeWorkerQueueBackpressure {
		t.Fatalf("ReasonCode = %q, want %q", stored.RetryableBlock.ReasonCode, retryableBlockReasonCodeWorkerQueueBackpressure)
	}
	if stored.Error == "" || !strings.Contains(stored.Error, "工作队列已满") {
		t.Fatalf("stored error = %q, want queue backpressure message", stored.Error)
	}
}

func TestCreateGenerateTaskAsyncRetryPreservesRequestScopeOnFinalFailure(t *testing.T) {
	t.Parallel()

	repo := newRetryableLifecycleTestRepo()
	submitter := &retryableLifecycleSequencedSubmitter{
		results: []error{worker.ErrQueueFull, errors.New("工作队列已满")},
		calls:   make(chan int, 2),
	}
	svc := newTaskLifecycleService(taskLifecycleServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return submitter
		},
	})
	ctx := openaiclient.WithIdentity(WithTenantID(context.Background(), "tenant-async"), openaiclient.Identity{
		TenantID: "tenant-async",
		UserID:   "user-async",
	})

	task, err := svc.CreateGenerateTask(ctx, &GenerateRequest{
		Text:      "queue retry preserves scope",
		Platforms: []string{"amazon"},
	})
	if err != nil {
		t.Fatalf("CreateGenerateTask() error = %v, want nil while async retry owns the queue-full recovery", err)
	}

	waitForRetryableLifecycleSubmitAttempt(t, submitter.calls, "initial submit attempt")
	waitForRetryableLifecycleSubmitAttempt(t, submitter.calls, "async retry attempt")
	waitForRetryableLifecycleTaskStatus(t, repo, task.ID, TaskStatusBlockedRetryable)

	if got := TenantIDFromContext(repo.blockedCtx); got != "tenant-async" {
		t.Fatalf("blocked tenant context = %q, want tenant-async", got)
	}
	if got := RequestUserIDFromContext(repo.blockedCtx); got != "user-async" {
		t.Fatalf("blocked user context = %q, want user-async", got)
	}
}

func TestProcessFlowMarksOpenAICreditFailureAsBlockedRetryable(t *testing.T) {
	t.Parallel()

	repo := newRetryableLifecycleTestRepo()
	svc, err := NewService(&ServiceConfig{
		Core: ServiceCoreDependencies{
			Repository:     repo,
			ProductService: retryableLifecycleTestProductService{processErr: errors.New("OpenAI API error: insufficient credits in account balance")},
		},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	task := &Task{
		ID:        "retryable-openai-credits-1",
		Status:    TaskStatusPending,
		Request:   &GenerateRequest{ProductURL: "https://example.com/product", Platforms: []string{"amazon"}},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	result, runErr := svc.ProcessListingKit(context.Background(), task)
	if runErr == nil {
		t.Fatal("ProcessListingKit() error = nil, want workflow failure")
	}
	if result != nil {
		t.Fatalf("ProcessListingKit() result = %+v, want nil on failure", result)
	}
	if repo.blockedTaskID != task.ID {
		t.Fatalf("MarkBlockedRetryable task ID = %q, want %q", repo.blockedTaskID, task.ID)
	}
	if repo.failedTaskID != "" {
		t.Fatalf("MarkFailed task ID = %q, want empty when workflow failure is retryable", repo.failedTaskID)
	}

	stored, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("GetTask() error = %v", getErr)
	}
	if stored.Status != TaskStatusBlockedRetryable {
		t.Fatalf("stored status = %q, want %q", stored.Status, TaskStatusBlockedRetryable)
	}
	if stored.Result == nil {
		t.Fatal("stored result = nil, want partial workflow result persisted")
	}
	if stored.RetryableBlock == nil {
		t.Fatal("stored RetryableBlock = nil, want retry metadata")
	}
	if stored.RetryableBlock.ReasonCode != retryableBlockReasonCodeOpenAIInsufficientCredits {
		t.Fatalf("ReasonCode = %q, want %q", stored.RetryableBlock.ReasonCode, retryableBlockReasonCodeOpenAIInsufficientCredits)
	}
}

func TestProcessFlowReturnsPersistenceErrorWhenFailureStateCannotBeStored(t *testing.T) {
	t.Parallel()

	repo := newRetryableLifecycleTestRepo()
	repo.blockedErr = errors.New("persist blocked state failed")
	svc, err := NewService(&ServiceConfig{
		Core: ServiceCoreDependencies{
			Repository:     repo,
			ProductService: retryableLifecycleTestProductService{processErr: errors.New("OpenAI API error: insufficient credits in account balance")},
		},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	task := &Task{
		ID:        "retryable-openai-persist-failure-1",
		Status:    TaskStatusPending,
		Request:   &GenerateRequest{ProductURL: "https://example.com/product", Platforms: []string{"amazon"}},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	result, runErr := svc.ProcessListingKit(context.Background(), task)
	if runErr == nil {
		t.Fatal("ProcessListingKit() error = nil, want persistence failure")
	}
	if result != nil {
		t.Fatalf("ProcessListingKit() result = %+v, want nil on failure", result)
	}
	if !strings.Contains(runErr.Error(), "persist blocked state failed") {
		t.Fatalf("ProcessListingKit() error = %v, want persistence failure included", runErr)
	}
}

func TestProcessFlowReturnsPartialResultSaveErrorWhenFailureResultCannotBeStored(t *testing.T) {
	t.Parallel()

	repo := newRetryableLifecycleTestRepo()
	repo.saveResultErr = errors.New("persist partial result failed")
	svc, err := NewService(&ServiceConfig{
		Core: ServiceCoreDependencies{
			Repository:     repo,
			ProductService: retryableLifecycleTestProductService{processErr: errors.New("OpenAI API error: insufficient credits in account balance")},
		},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	task := &Task{
		ID:        "retryable-openai-save-failure-1",
		Status:    TaskStatusPending,
		Request:   &GenerateRequest{ProductURL: "https://example.com/product", Platforms: []string{"amazon"}},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	result, runErr := svc.ProcessListingKit(context.Background(), task)
	if runErr == nil {
		t.Fatal("ProcessListingKit() error = nil, want partial result persistence failure")
	}
	if result != nil {
		t.Fatalf("ProcessListingKit() result = %+v, want nil on failure", result)
	}
	if !strings.Contains(runErr.Error(), "persist partial result failed") {
		t.Fatalf("ProcessListingKit() error = %v, want partial result persistence failure included", runErr)
	}
	if repo.blockedTaskID != task.ID {
		t.Fatalf("MarkBlockedRetryable task ID = %q, want %q even when partial result save fails", repo.blockedTaskID, task.ID)
	}
	stored, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("GetTask() error = %v", getErr)
	}
	if stored.Status != TaskStatusBlockedRetryable {
		t.Fatalf("stored status = %q, want %q so task does not remain processing", stored.Status, TaskStatusBlockedRetryable)
	}
}

func TestCreateGenerateTaskMarksNonRetryableSubmitFailureAsFailed(t *testing.T) {
	t.Parallel()

	repo := newRetryableLifecycleTestRepo()
	svc := newTaskLifecycleService(taskLifecycleServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return retryableLifecycleTestSubmitter{err: errors.New("submit exploded permanently")}
		},
	})

	_, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Text:      "non retryable submit failure",
		Platforms: []string{"amazon"},
	})
	if err == nil {
		t.Fatal("CreateGenerateTask() error = nil, want submit failure")
	}
	taskID := repo.onlyTaskID(t)
	if repo.failedTaskID != taskID {
		t.Fatalf("MarkFailed task ID = %q, want %q", repo.failedTaskID, taskID)
	}
	if repo.blockedTaskID != "" {
		t.Fatalf("MarkBlockedRetryable task ID = %q, want empty for non-retryable failure", repo.blockedTaskID)
	}

	stored, getErr := repo.GetTask(context.Background(), taskID)
	if getErr != nil {
		t.Fatalf("GetTask() error = %v", getErr)
	}
	if stored.Status != TaskStatusFailed {
		t.Fatalf("stored status = %q, want %q", stored.Status, TaskStatusFailed)
	}
	if stored.RetryableBlock != nil {
		t.Fatalf("stored RetryableBlock = %+v, want nil for non-retryable failure", stored.RetryableBlock)
	}
}

func TestCreateGenerateTaskReturnsPersistenceErrorWhenSubmitFailureCannotBeStored(t *testing.T) {
	t.Parallel()

	repo := newRetryableLifecycleTestRepo()
	repo.failedErr = errors.New("persist failed state failed")
	svc := newTaskLifecycleService(taskLifecycleServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return retryableLifecycleTestSubmitter{err: errors.New("submit exploded permanently")}
		},
	})

	_, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Text:      "non retryable submit persistence failure",
		Platforms: []string{"amazon"},
	})
	if err == nil {
		t.Fatal("CreateGenerateTask() error = nil, want persistence failure")
	}
	if !strings.Contains(err.Error(), "persist failed state failed") {
		t.Fatalf("CreateGenerateTask() error = %v, want persistence failure included", err)
	}
}

type retryableLifecycleTestRepo struct {
	tasks         map[string]*Task
	failedTaskID  string
	failedError   string
	failedCtx     context.Context
	failedErr     error
	blockedTaskID string
	blockedError  string
	blockedBlock  *RetryableBlock
	blockedCtx    context.Context
	blockedErr    error
	saveResultErr error
}

func newRetryableLifecycleTestRepo() *retryableLifecycleTestRepo {
	return &retryableLifecycleTestRepo{tasks: map[string]*Task{}}
}

func (r *retryableLifecycleTestRepo) onlyTaskID(t *testing.T) string {
	t.Helper()
	if len(r.tasks) != 1 {
		t.Fatalf("task count = %d, want 1", len(r.tasks))
	}
	for taskID := range r.tasks {
		return taskID
	}
	t.Fatal("task ID = empty, want stored task")
	return ""
}

func (r *retryableLifecycleTestRepo) CreateTask(_ context.Context, task *Task) error {
	copied := *task
	r.tasks[task.ID] = &copied
	return nil
}

func (r *retryableLifecycleTestRepo) GetTask(_ context.Context, taskID string) (*Task, error) {
	task, ok := r.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	copied := *task
	return &copied, nil
}

func (r *retryableLifecycleTestRepo) ListTasks(context.Context, *TaskListQuery) ([]Task, int64, error) {
	return nil, 0, nil
}

func (r *retryableLifecycleTestRepo) MarkProcessing(_ context.Context, taskID string) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	task.Status = TaskStatusProcessing
	task.UpdatedAt = time.Now()
	return nil
}

func (r *retryableLifecycleTestRepo) MarkCompleted(_ context.Context, taskID string, result *ListingKitResult) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	task.Status = TaskStatusCompleted
	task.Result = result
	task.Error = ""
	task.RetryableBlock = nil
	task.UpdatedAt = time.Now()
	return nil
}

func (r *retryableLifecycleTestRepo) MarkNeedsReview(_ context.Context, taskID string, result *ListingKitResult, reason string) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	task.Status = TaskStatusNeedsReview
	task.Result = result
	task.Error = reason
	task.RetryableBlock = nil
	task.UpdatedAt = time.Now()
	return nil
}

func (r *retryableLifecycleTestRepo) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	r.failedTaskID = taskID
	r.failedError = errorMsg
	r.failedCtx = ctx
	if r.failedErr != nil {
		return r.failedErr
	}
	task.Status = TaskStatusFailed
	task.Error = errorMsg
	task.RetryableBlock = nil
	task.UpdatedAt = time.Now()
	return nil
}

func (r *retryableLifecycleTestRepo) MarkBlockedRetryable(ctx context.Context, taskID string, block *RetryableBlock, errorMsg string) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	r.blockedTaskID = taskID
	r.blockedError = errorMsg
	r.blockedBlock = cloneRetryableBlock(block)
	r.blockedCtx = ctx
	if r.blockedErr != nil {
		return r.blockedErr
	}
	task.Status = TaskStatusBlockedRetryable
	task.Error = errorMsg
	task.RetryableBlock = cloneRetryableBlock(block)
	task.UpdatedAt = time.Now()
	return nil
}

func (r *retryableLifecycleTestRepo) ListRecoverableTasks(context.Context, *RecoverableTaskQuery) ([]Task, error) {
	return nil, nil
}

func (r *retryableLifecycleTestRepo) RecoverBlockedTaskNow(context.Context, string, time.Time) error {
	return nil
}

func (r *retryableLifecycleTestRepo) BulkRecoverBlockedTasks(context.Context, *RecoverBlockedTasksQuery) (int64, error) {
	return 0, nil
}

func (r *retryableLifecycleTestRepo) PrepareRetry(context.Context, string) error {
	return nil
}

func (r *retryableLifecycleTestRepo) IncrementRetryCount(context.Context, string) error {
	return nil
}

func (r *retryableLifecycleTestRepo) SaveTaskResult(_ context.Context, taskID string, result *ListingKitResult) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	if r.saveResultErr != nil {
		return r.saveResultErr
	}
	task.Result = result
	task.UpdatedAt = time.Now()
	return nil
}

type retryableLifecycleTestSubmitter struct {
	err error
}

func (s retryableLifecycleTestSubmitter) Submit(string) error {
	return s.err
}

type retryableLifecycleSequencedSubmitter struct {
	results []error
	calls   chan int
	count   int
}

func (s *retryableLifecycleSequencedSubmitter) Submit(string) error {
	s.count++
	if s.calls != nil {
		s.calls <- s.count
	}
	if len(s.results) == 0 {
		return nil
	}
	result := s.results[0]
	if len(s.results) > 1 {
		s.results = s.results[1:]
	}
	return result
}

func waitForRetryableLifecycleSubmitAttempt(t *testing.T, calls <-chan int, label string) {
	t.Helper()
	select {
	case <-calls:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", label)
	}
}

func waitForRetryableLifecycleTaskStatus(t *testing.T, repo *retryableLifecycleTestRepo, taskID string, want TaskStatus) *Task {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		task, err := repo.GetTask(context.Background(), taskID)
		if err == nil && task.Status == want {
			return task
		}
		time.Sleep(10 * time.Millisecond)
	}
	task, err := repo.GetTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTask(%s) error after waiting for %q: %v", taskID, want, err)
	}
	t.Fatalf("stored status = %q after waiting, want %q", task.Status, want)
	return nil
}

type retryableLifecycleTestProductService struct {
	processErr error
}

func (s retryableLifecycleTestProductService) CreateGenerateTask(_ context.Context, _ *productenrich.GenerateRequest) (*productenrich.Task, error) {
	return &productenrich.Task{
		ID:      "product-task-retryable-1",
		Request: &productenrich.GenerateRequest{ProductURL: "https://example.com/product"},
	}, nil
}

func (s retryableLifecycleTestProductService) GetTaskResult(context.Context, string) (*productenrich.TaskResult, error) {
	return nil, nil
}

func (s retryableLifecycleTestProductService) ProcessProduct(context.Context, *productenrich.Task) (*productenrich.ProductJSON, error) {
	return nil, s.processErr
}
