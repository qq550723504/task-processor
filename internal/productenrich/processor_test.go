package productenrich

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// --- mock implementations ---

type mockService struct {
	result *ProductJSON
	err    error
}

func (m *mockService) CreateGenerateTask(_ context.Context, _ *GenerateRequest) (*Task, error) {
	return nil, nil
}
func (m *mockService) GetTaskResult(_ context.Context, _ string) (*TaskResult, error) {
	return nil, nil
}
func (m *mockService) ProcessProduct(_ context.Context, _ *Task) (*ProductJSON, error) {
	return m.result, m.err
}
func (m *mockService) SetTaskSubmitter(_ TaskSubmitter) {}

type mockTaskRepo struct {
	tasks        map[string]*Task
	retryErr     error
	resetErr     error
	incrementErr error
}

func newMockTaskRepo(tasks ...*Task) *mockTaskRepo {
	r := &mockTaskRepo{tasks: make(map[string]*Task)}
	for _, t := range tasks {
		r.tasks[t.ID] = t
	}
	return r
}

func (r *mockTaskRepo) CreateTask(_ context.Context, task *Task) error {
	r.tasks[task.ID] = task
	return nil
}
func (r *mockTaskRepo) GetTask(_ context.Context, id string) (*Task, error) {
	t, ok := r.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return t, nil
}
func (r *mockTaskRepo) UpdateTaskStatus(_ context.Context, id string, status TaskStatus) error {
	if t, ok := r.tasks[id]; ok {
		t.Status = status
	}
	return nil
}
func (r *mockTaskRepo) UpdateTaskError(_ context.Context, id string, msg string) error {
	if t, ok := r.tasks[id]; ok {
		t.Error = msg
		t.Status = TaskStatusFailed
	}
	return nil
}
func (r *mockTaskRepo) SaveTaskResult(_ context.Context, id string, result *ProductJSON) error {
	if t, ok := r.tasks[id]; ok {
		t.Result = result
		t.Status = TaskStatusCompleted
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
func (r *mockTaskRepo) ResetForRetry(_ context.Context, id string) error {
	if r.resetErr != nil {
		return r.resetErr
	}
	if t, ok := r.tasks[id]; ok {
		t.Status = TaskStatusPending
	}
	return nil
}

// --- tests ---

func TestProcessor_ProcessTask_Success(t *testing.T) {
	task := &Task{ID: "t1", Request: &GenerateRequest{}, Status: TaskStatusPending}
	repo := newMockTaskRepo(task)
	svc := &mockService{result: &ProductJSON{Title: "ok"}}
	p, _ := NewProcessor(svc, repo, logrus.New(), 3)

	err := p.ProcessTask(context.Background(), worker.WorkerJob{TaskData: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessor_ProcessTask_EmptyTaskID(t *testing.T) {
	repo := newMockTaskRepo()
	svc := &mockService{}
	p, _ := NewProcessor(svc, repo, logrus.New(), 3)

	err := p.ProcessTask(context.Background(), worker.WorkerJob{TaskData: ""})
	if err == nil {
		t.Fatal("expected error for empty task ID")
	}
}

func TestProcessor_ProcessTask_NoRetryOnRejection(t *testing.T) {
	task := &Task{ID: "t2", Request: &GenerateRequest{}, Status: TaskStatusPending}
	repo := newMockTaskRepo(task)
	svc := &mockService{err: &errNoRetry{cause: errors.New("data quality insufficient")}}
	p, _ := NewProcessor(svc, repo, logrus.New(), 3)

	err := p.ProcessTask(context.Background(), worker.WorkerJob{TaskData: "t2"})
	if err == nil {
		t.Fatal("expected error")
	}
	// 拒绝类错误不应增加 retry_count
	if task.RetryCount != 0 {
		t.Errorf("RetryCount = %d, want 0 (no retry on rejection)", task.RetryCount)
	}
}

func TestProcessor_ProcessTask_RetryOnTransientError(t *testing.T) {
	task := &Task{ID: "t3", Request: &GenerateRequest{}, Status: TaskStatusPending, RetryCount: 0}
	repo := newMockTaskRepo(task)
	svc := &mockService{err: errors.New("transient error")}
	p, _ := NewProcessor(svc, repo, logrus.New(), 3)

	_ = p.ProcessTask(context.Background(), worker.WorkerJob{TaskData: "t3"})

	if task.RetryCount != 1 {
		t.Errorf("RetryCount = %d, want 1", task.RetryCount)
	}
	if task.Status != TaskStatusPending {
		t.Errorf("Status = %q, want pending (reset for retry)", task.Status)
	}
}

func TestProcessor_ProcessTask_ExceedMaxRetries(t *testing.T) {
	task := &Task{ID: "t4", Request: &GenerateRequest{}, Status: TaskStatusFailed, RetryCount: 3}
	repo := newMockTaskRepo(task)
	svc := &mockService{err: errors.New("still failing")}
	p, _ := NewProcessor(svc, repo, logrus.New(), 3)

	_ = p.ProcessTask(context.Background(), worker.WorkerJob{TaskData: "t4"})

	// 超过最大重试次数，不应再 reset 为 pending
	if task.Status == TaskStatusPending {
		t.Error("task should not be reset to pending after exceeding max retries")
	}
}

func TestProcessor_NewProcessor_Validation(t *testing.T) {
	logger := logrus.New()
	repo := newMockTaskRepo()
	svc := &mockService{}

	if _, err := NewProcessor(nil, repo, logger, 3); err == nil {
		t.Error("expected error for nil service")
	}
	if _, err := NewProcessor(svc, nil, logger, 3); err == nil {
		t.Error("expected error for nil repo")
	}
	if _, err := NewProcessor(svc, repo, nil, 3); err == nil {
		t.Error("expected error for nil logger")
	}
}

func TestIsNoRetryError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"errNoRetry", &errNoRetry{cause: errors.New("x")}, true},
		{"wrapped errNoRetry", errors.New("outer"), false},
		{"plain error", errors.New("plain"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isNoRetryError(tc.err)
			if got != tc.want {
				t.Errorf("isNoRetryError = %v, want %v", got, tc.want)
			}
		})
	}
}
