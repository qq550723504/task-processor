package pipeline_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"

	"task-processor/internal/infra/worker"
	"task-processor/internal/productenrich"
	"task-processor/internal/productenrich/pipeline"
)

type mockService struct {
	result *productenrich.ProductJSON
	err    error
	calls  int
}

func (m *mockService) CreateGenerateTask(_ context.Context, _ *productenrich.GenerateRequest) (*productenrich.Task, error) {
	return nil, nil
}

func (m *mockService) GetTaskResult(_ context.Context, _ string) (*productenrich.TaskResult, error) {
	return nil, nil
}

func (m *mockService) ProcessProduct(_ context.Context, _ *productenrich.Task) (*productenrich.ProductJSON, error) {
	m.calls++
	return m.result, m.err
}

func (m *mockService) SetTaskSubmitter(_ productenrich.TaskSubmitter) {}

type mockTaskRepo struct {
	tasks        map[string]*productenrich.Task
	retryErr     error
	resetErr     error
	incrementErr error
}

func newMockTaskRepo(tasks ...*productenrich.Task) *mockTaskRepo {
	r := &mockTaskRepo{tasks: make(map[string]*productenrich.Task)}
	for _, t := range tasks {
		r.tasks[t.ID] = t
	}
	return r
}

func (r *mockTaskRepo) CreateTask(_ context.Context, task *productenrich.Task) error {
	r.tasks[task.ID] = task
	return nil
}

func (r *mockTaskRepo) GetTask(_ context.Context, id string) (*productenrich.Task, error) {
	t, ok := r.tasks[id]
	if !ok {
		return nil, productenrich.ErrTaskNotFound
	}
	return t, nil
}

func (r *mockTaskRepo) MarkProcessing(_ context.Context, id string) error {
	if t, ok := r.tasks[id]; ok {
		t.Status = productenrich.TaskStatusProcessing
		t.Error = ""
	}
	return nil
}

func (r *mockTaskRepo) UpdateTaskStatus(_ context.Context, id string, status productenrich.TaskStatus) error {
	if t, ok := r.tasks[id]; ok {
		t.Status = status
	}
	return nil
}

func (r *mockTaskRepo) MarkFailed(_ context.Context, id string, msg string) error {
	if t, ok := r.tasks[id]; ok {
		t.Error = msg
		t.Status = productenrich.TaskStatusFailed
	}
	return nil
}

func (r *mockTaskRepo) UpdateTaskError(_ context.Context, id string, msg string) error {
	if t, ok := r.tasks[id]; ok {
		t.Error = msg
		t.Status = productenrich.TaskStatusFailed
	}
	return nil
}

func (r *mockTaskRepo) MarkCompleted(_ context.Context, id string, result *productenrich.ProductJSON) error {
	if t, ok := r.tasks[id]; ok {
		t.Result = result
		t.Status = productenrich.TaskStatusCompleted
		t.Error = ""
	}
	return nil
}

func (r *mockTaskRepo) SaveTaskResult(_ context.Context, id string, result *productenrich.ProductJSON) error {
	if t, ok := r.tasks[id]; ok {
		t.Result = result
		t.Status = productenrich.TaskStatusCompleted
	}
	return nil
}

func (r *mockTaskRepo) IncrementRetryCount(_ context.Context, id string) error {
	if r.incrementErr != nil {
		return r.incrementErr
	}
	if t, ok := r.tasks[id]; ok {
		t.RetryCount++
	}
	return nil
}

func (r *mockTaskRepo) PrepareRetry(_ context.Context, id string) error {
	if r.resetErr != nil {
		return r.resetErr
	}
	if t, ok := r.tasks[id]; ok {
		t.Status = productenrich.TaskStatusPending
		t.Error = ""
	}
	return nil
}

func (r *mockTaskRepo) ResetForRetry(_ context.Context, id string) error {
	if r.resetErr != nil {
		return r.resetErr
	}
	if t, ok := r.tasks[id]; ok {
		t.Status = productenrich.TaskStatusPending
		t.Error = ""
	}
	return nil
}

type mockTaskSubmitter struct {
	submitErr error
}

func (m *mockTaskSubmitter) Submit(_ string) error {
	return m.submitErr
}

func TestProcessor_ProcessTask_Success(t *testing.T) {
	task := &productenrich.Task{ID: "t1", Request: &productenrich.GenerateRequest{}, Status: productenrich.TaskStatusPending}
	repo := newMockTaskRepo(task)
	svc := &mockService{result: &productenrich.ProductJSON{Title: "ok"}}
	p, _ := pipeline.NewProcessor(svc, repo, logrus.New(), 3)

	err := p.ProcessTask(context.Background(), worker.WorkerJob{TaskData: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessor_ProcessTask_SkipsCompletedTask(t *testing.T) {
	task := &productenrich.Task{ID: "done-1", Request: &productenrich.GenerateRequest{}, Status: productenrich.TaskStatusCompleted}
	repo := newMockTaskRepo(task)
	svc := &mockService{result: &productenrich.ProductJSON{Title: "ok"}}
	p, _ := pipeline.NewProcessor(svc, repo, logrus.New(), 3)

	err := p.ProcessTask(context.Background(), worker.WorkerJob{TaskData: "done-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.calls != 0 {
		t.Fatalf("ProcessProduct called %d times, want 0", svc.calls)
	}
}

func TestProcessor_ProcessTask_EmptyTaskID(t *testing.T) {
	repo := newMockTaskRepo()
	svc := &mockService{}
	p, _ := pipeline.NewProcessor(svc, repo, logrus.New(), 3)

	err := p.ProcessTask(context.Background(), worker.WorkerJob{TaskData: ""})
	if err == nil {
		t.Fatal("expected error for empty task ID")
	}
}

func TestProcessor_ProcessTask_NoRetryOnRejection(t *testing.T) {
	task := &productenrich.Task{ID: "t2", Request: &productenrich.GenerateRequest{}, Status: productenrich.TaskStatusPending}
	repo := newMockTaskRepo(task)
	svc := &mockService{err: productenrich.NewNoRetryError(errors.New("data quality insufficient"))}
	p, _ := pipeline.NewProcessor(svc, repo, logrus.New(), 3)

	err := p.ProcessTask(context.Background(), worker.WorkerJob{TaskData: "t2"})
	if err == nil {
		t.Fatal("expected error")
	}
	if task.RetryCount != 0 {
		t.Errorf("RetryCount = %d, want 0 (no retry on rejection)", task.RetryCount)
	}
}

func TestProcessor_ProcessTask_RetryOnTransientError(t *testing.T) {
	task := &productenrich.Task{ID: "t3", Request: &productenrich.GenerateRequest{}, Status: productenrich.TaskStatusPending, RetryCount: 0, Error: "previous failure"}
	repo := newMockTaskRepo(task)
	svc := &mockService{err: errors.New("transient error")}
	p, _ := pipeline.NewProcessor(svc, repo, logrus.New(), 3)
	p.SetTaskSubmitter(&mockTaskSubmitter{})

	_ = p.ProcessTask(context.Background(), worker.WorkerJob{TaskData: "t3"})

	if task.RetryCount != 1 {
		t.Errorf("RetryCount = %d, want 1", task.RetryCount)
	}
	if task.Status != productenrich.TaskStatusPending {
		t.Errorf("Status = %q, want pending (reset for retry)", task.Status)
	}
	if task.Error != "" {
		t.Errorf("Error = %q, want empty after retry reset", task.Error)
	}
}

func TestProcessor_ProcessTask_ExceedMaxRetries(t *testing.T) {
	task := &productenrich.Task{ID: "t4", Request: &productenrich.GenerateRequest{}, Status: productenrich.TaskStatusFailed, RetryCount: 3}
	repo := newMockTaskRepo(task)
	svc := &mockService{err: errors.New("still failing")}
	p, _ := pipeline.NewProcessor(svc, repo, logrus.New(), 3)

	_ = p.ProcessTask(context.Background(), worker.WorkerJob{TaskData: "t4"})

	if task.Status == productenrich.TaskStatusPending {
		t.Error("task should not be reset to pending after exceeding max retries")
	}
}

func TestProcessor_ProcessTask_ResubmitFailure_MarksTaskFailed(t *testing.T) {
	task := &productenrich.Task{ID: "t5", Request: &productenrich.GenerateRequest{}, Status: productenrich.TaskStatusPending, RetryCount: 0, Error: "old error"}
	repo := newMockTaskRepo(task)
	svc := &mockService{err: errors.New("transient error")}
	p, _ := pipeline.NewProcessor(svc, repo, logrus.New(), 3)
	p.SetTaskSubmitter(&mockTaskSubmitter{submitErr: errors.New("queue unavailable")})

	_ = p.ProcessTask(context.Background(), worker.WorkerJob{TaskData: "t5"})

	if task.Status != productenrich.TaskStatusFailed {
		t.Errorf("Status = %q, want failed", task.Status)
	}
	if task.Error == "" {
		t.Error("expected task error to be set after resubmit failure")
	}
}

func TestProcessor_NewProcessor_Validation(t *testing.T) {
	logger := logrus.New()
	repo := newMockTaskRepo()
	svc := &mockService{}

	if _, err := pipeline.NewProcessor(nil, repo, logger, 3); err == nil {
		t.Error("expected error for nil service")
	}
	if _, err := pipeline.NewProcessor(svc, nil, logger, 3); err == nil {
		t.Error("expected error for nil repo")
	}
	if _, err := pipeline.NewProcessor(svc, repo, nil, 3); err == nil {
		t.Error("expected error for nil logger")
	}
}

func TestIsNoRetryError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"errNoRetry", productenrich.NewNoRetryError(errors.New("x")), true},
		{"wrapped errNoRetry", errors.New("outer"), false},
		{"plain error", errors.New("plain"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := productenrich.IsNoRetryError(tc.err)
			if got != tc.want {
				t.Errorf("IsNoRetryError = %v, want %v", got, tc.want)
			}
		})
	}
}
