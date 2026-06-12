package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

type stubTaskRecoveryHandlerService struct {
	task           *listingkit.Task
	recoveredCount int64
	requeueResult  *listingkit.RequeuePendingTasksResult
	err            error

	lastRecoveredTaskID string
	lastRecoverQuery    *listingkit.RecoverBlockedTasksQuery
	lastRequeueRequest  *listingkit.RequeuePendingTasksRequest
}

func (s *stubTaskRecoveryHandlerService) RecoverTaskNow(_ context.Context, taskID string) (*listingkit.Task, error) {
	s.lastRecoveredTaskID = taskID
	return s.task, s.err
}

func (s *stubTaskRecoveryHandlerService) RunRecoverySweep(context.Context, time.Time, int) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *stubTaskRecoveryHandlerService) BulkRecoverTasks(_ context.Context, query *listingkit.RecoverBlockedTasksQuery) (int64, error) {
	s.lastRecoverQuery = query
	return s.recoveredCount, s.err
}

func (s *stubTaskRecoveryHandlerService) RequeuePendingTasks(_ context.Context, req *listingkit.RequeuePendingTasksRequest) (*listingkit.RequeuePendingTasksResult, error) {
	s.lastRequeueRequest = req
	return s.requeueResult, s.err
}

func TestRecoverTaskNowHandlerBindsTaskIDAndReturnsTask(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubTaskRecoveryHandlerService{
		task: &listingkit.Task{
			ID:       "task-123",
			TenantID: "tenant-1",
			Status:   listingkit.TaskStatusPending,
		},
	}
	h, err := NewHandler(&stubGenerationTaskService{}, WithTaskRecoveryService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/recover", h.RecoverTaskNow)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-123/recover", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.lastRecoveredTaskID != "task-123" {
		t.Fatalf("task id = %q, want task-123", svc.lastRecoveredTaskID)
	}

	var body struct {
		Task *listingkit.Task `json:"task"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Task == nil || body.Task.ID != "task-123" || body.Task.Status != listingkit.TaskStatusPending {
		t.Fatalf("body = %+v, want recovered task payload", body)
	}
}

func TestBulkRecoverTasksHandlerBindsQueryAndReturnsCount(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	dueBefore := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	recoverAt := time.Date(2026, 6, 6, 13, 30, 0, 0, time.UTC)
	svc := &stubTaskRecoveryHandlerService{recoveredCount: 4}
	h, err := NewHandler(&stubGenerationTaskService{}, WithTaskRecoveryService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/recovery/recover", h.BulkRecoverTasks)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/listing-kits/tasks/recovery/recover?due_before="+dueBefore.Format(time.RFC3339)+"&limit=7",
		strings.NewReader(`{"recover_at":"`+recoverAt.Format(time.RFC3339)+`"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.lastRecoverQuery == nil {
		t.Fatal("expected recover query")
	}
	if !svc.lastRecoverQuery.DueBefore.Equal(dueBefore) {
		t.Fatalf("due_before = %v, want %v", svc.lastRecoverQuery.DueBefore, dueBefore)
	}
	if !svc.lastRecoverQuery.RecoverAt.Equal(recoverAt) {
		t.Fatalf("recover_at = %v, want %v", svc.lastRecoverQuery.RecoverAt, recoverAt)
	}
	if svc.lastRecoverQuery.Limit != 7 {
		t.Fatalf("limit = %d, want 7", svc.lastRecoverQuery.Limit)
	}

	var body struct {
		RecoveredCount int64 `json:"recovered_count"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.RecoveredCount != 4 {
		t.Fatalf("recovered_count = %d, want 4", body.RecoveredCount)
	}
}

func TestRecoverTaskNowHandlerReturnsNotFoundForMissingTask(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubTaskRecoveryHandlerService{err: listingkit.ErrTaskNotFound}
	h, err := NewHandler(&stubGenerationTaskService{}, WithTaskRecoveryService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/recover", h.RecoverTaskNow)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-missing/recover", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.Code)
	}
}

func TestBulkRecoverTasksHandlerReturnsBadRequestForInvalidQuery(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubTaskRecoveryHandlerService{}
	h, err := NewHandler(&stubGenerationTaskService{}, WithTaskRecoveryService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/recovery/recover", h.BulkRecoverTasks)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/recovery/recover?due_before=not-a-time", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}
