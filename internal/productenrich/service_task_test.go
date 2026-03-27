package productenrich

import (
	"context"
	"errors"
	"testing"
)

// mockTaskSubmitter 捕获 Submit 调用，实现 TaskSubmitter 接口
type mockTaskSubmitter struct {
	submitted []string
	submitErr error
}

func (m *mockTaskSubmitter) Submit(taskID string) error {
	if m.submitErr != nil {
		return m.submitErr
	}
	m.submitted = append(m.submitted, taskID)
	return nil
}

func newSvcWithSubmitter(t *testing.T, submitter TaskSubmitter) (*productService, *mockTaskRepo) {
	t.Helper()
	repo := newMockTaskRepo()
	svc, err := NewProductService(&ProductServiceConfig{
		QueueName:     "q",
		TaskRepo:      repo,
		RedisClient:   &mockRedisClient{},
		TaskSubmitter: submitter,
	})
	if err != nil {
		t.Fatalf("NewProductService: %v", err)
	}
	return svc.(*productService), repo
}

// --- validateRequest ---

func TestValidateRequest_NoInput_ReturnsError(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	err := svc.validateRequest(&GenerateRequest{})
	if err == nil {
		t.Fatal("expected error when no input provided")
	}
}

func TestValidateRequest_TooManyImages_ReturnsError(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	urls := make([]string, 11)
	for i := range urls {
		urls[i] = "http://example.com/img.jpg"
	}
	err := svc.validateRequest(&GenerateRequest{ImageURLs: urls})
	if err == nil {
		t.Fatal("expected error for >10 image URLs")
	}
}

func TestValidateRequest_TextTooLong_ReturnsError(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	longText := make([]byte, 10001)
	err := svc.validateRequest(&GenerateRequest{Text: string(longText)})
	if err == nil {
		t.Fatal("expected error for text > 10000 chars")
	}
}

func TestValidateRequest_ValidInputs_NoError(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	cases := []struct {
		name string
		req  *GenerateRequest
	}{
		{"image url", &GenerateRequest{ImageURLs: []string{"http://example.com/img.jpg"}}},
		{"text", &GenerateRequest{Text: "a product"}},
		{"product url", &GenerateRequest{ProductURL: "https://detail.1688.com/offer/123.html"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := svc.validateRequest(tc.req); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateRequest_UnsupportedProductURL_ReturnsError(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	err := svc.validateRequest(&GenerateRequest{ProductURL: "https://example.com/product/123"})
	if err == nil {
		t.Fatal("expected error for unsupported product_url host")
	}
}

func TestValidateRequest_NonDetail1688ProductURL_ReturnsError(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	err := svc.validateRequest(&GenerateRequest{ProductURL: "https://detail.1688.com/product/123.html"})
	if err == nil {
		t.Fatal("expected error for non-detail 1688 product_url path")
	}
}

// --- CreateGenerateTask ---

func TestCreateGenerateTask_NilRequest_ReturnsError(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	_, err := svc.CreateGenerateTask(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestCreateGenerateTask_InvalidRequest_ReturnsError(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	_, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{})
	if err == nil {
		t.Fatal("expected error for invalid request")
	}
}

func TestCreateGenerateTask_WithSubmitter_SubmitsJob(t *testing.T) {
	submitter := &mockTaskSubmitter{}
	svc, repo := newSvcWithSubmitter(t, submitter)

	task, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{Text: "a product"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.ID == "" {
		t.Error("expected non-empty task ID")
	}
	if task.Status != TaskStatusPending {
		t.Errorf("Status = %q, want pending", task.Status)
	}
	if _, ok := repo.tasks[task.ID]; !ok {
		t.Error("task not saved to repo")
	}
	if len(submitter.submitted) != 1 {
		t.Errorf("submitter.submitted len = %d, want 1", len(submitter.submitted))
	}
	if submitter.submitted[0] != task.ID {
		t.Errorf("submitted taskID = %q, want %q", submitter.submitted[0], task.ID)
	}
}

func TestCreateGenerateTask_SubmitFail_ReturnsError(t *testing.T) {
	submitter := &mockTaskSubmitter{submitErr: errors.New("pool full")}
	svc, _ := newSvcWithSubmitter(t, submitter)

	_, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{Text: "a product"})
	if err == nil {
		t.Fatal("expected error when submitter.Submit fails")
	}
}

func TestCreateGenerateTask_NoSubmitter_FallsBackToRedis(t *testing.T) {
	repo := newMockTaskRepo()
	rc := newMockRedisForCache()
	svc, err := NewProductService(&ProductServiceConfig{
		QueueName:   "myqueue",
		TaskRepo:    repo,
		RedisClient: rc,
	})
	if err != nil {
		t.Fatalf("NewProductService: %v", err)
	}

	task, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{Text: "a product"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rc.store["myqueue"] != task.ID {
		t.Errorf("redis store[myqueue] = %q, want %q", rc.store["myqueue"], task.ID)
	}
}

// --- GetTaskResult ---

func TestGetTaskResult_EmptyID_ReturnsError(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	_, err := svc.GetTaskResult(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty task ID")
	}
}

func TestGetTaskResult_NotFound_ReturnsErrTaskNotFound(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	_, err := svc.GetTaskResult(context.Background(), "nonexistent")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestGetTaskResult_CompletedTask_SetsCompletedAt(t *testing.T) {
	svc, repo := newSvcWithSubmitter(t, nil)
	task := &Task{
		ID:      "t1",
		Request: &GenerateRequest{},
		Status:  TaskStatusCompleted,
		Result:  &ProductJSON{Title: "done"},
	}
	repo.tasks[task.ID] = task

	result, err := svc.GetTaskResult(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != TaskStatusCompleted {
		t.Errorf("Status = %q, want completed", result.Status)
	}
	if result.CompletedAt == nil {
		t.Error("expected CompletedAt to be set for completed task")
	}
	if result.ProductJSON == nil || result.ProductJSON.Title != "done" {
		t.Error("expected ProductJSON to be populated")
	}
}

func TestGetTaskResult_PendingTask_NoCompletedAt(t *testing.T) {
	svc, repo := newSvcWithSubmitter(t, nil)
	task := &Task{
		ID:      "t2",
		Request: &GenerateRequest{},
		Status:  TaskStatusPending,
	}
	repo.tasks[task.ID] = task

	result, err := svc.GetTaskResult(context.Background(), "t2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CompletedAt != nil {
		t.Error("expected CompletedAt to be nil for pending task")
	}
}

func TestGetTaskResult_PendingTask_DoesNotExposeStaleErrorAfterRetryReset(t *testing.T) {
	svc, repo := newSvcWithSubmitter(t, nil)
	task := &Task{
		ID:      "t3",
		Request: &GenerateRequest{},
		Status:  TaskStatusFailed,
		Error:   "old failure",
	}
	repo.tasks[task.ID] = task

	if err := repo.ResetForRetry(context.Background(), task.ID); err != nil {
		t.Fatalf("ResetForRetry: %v", err)
	}

	result, err := svc.GetTaskResult(context.Background(), "t3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != TaskStatusPending {
		t.Fatalf("Status = %q, want pending", result.Status)
	}
	if result.Error != "" {
		t.Errorf("Error = %q, want empty after retry reset", result.Error)
	}
}
