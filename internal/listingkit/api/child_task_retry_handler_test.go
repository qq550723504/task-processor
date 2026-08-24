package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
)

type stubChildTaskRetryService struct {
	result   *listingkit.TaskResult
	accepted *listingkit.TaskChildRetryAccepted
	req      *listingkit.RetryChildTaskRequest
	err      error
}

func (s *stubChildTaskRetryService) RetryTaskChildTask(_ context.Context, _ string, req *listingkit.RetryChildTaskRequest) (*listingkit.TaskResult, error) {
	s.req = req
	return s.result, s.err
}

func (s *stubChildTaskRetryService) ScheduleTaskChildRetry(_ context.Context, _ string, req *listingkit.RetryChildTaskRequest) (*listingkit.TaskChildRetryAccepted, error) {
	s.req = req
	return s.accepted, s.err
}

func TestRetryTaskChildTaskReturnsBadRequestForEmptyKind(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubChildTaskRetryService{}
	h, err := NewHandler(&stubHandlerCoreService{}, WithChildTaskRetryService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/child-tasks/retry", h.RetryTaskChildTask)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/child-tasks/retry", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestRetryTaskChildTaskReturnsConflictWhenRetryBlocked(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubChildTaskRetryService{err: core.ErrChildTaskRetryConflict}
	h, err := NewHandler(&stubHandlerCoreService{}, WithChildTaskRetryService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/child-tasks/retry", h.RetryTaskChildTask)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/child-tasks/retry", strings.NewReader(`{"kind":"sds_design_sync"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.Code)
	}
}

func TestRetryTaskChildTaskReturnsConflictWhenRepairIsActive(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubChildTaskRetryService{err: listingkit.ErrSDSRepairRetryInProgress}
	h, err := NewHandler(&stubHandlerCoreService{}, WithChildTaskRetryService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/child-tasks/retry", h.RetryTaskChildTask)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/child-tasks/retry", strings.NewReader(`{"kind":"sds_design_sync"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.Code)
	}
}

func TestRetryTaskChildTaskReturnsQueuedPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubChildTaskRetryService{
		accepted: &listingkit.TaskChildRetryAccepted{
			TaskID: "task-1", Kind: "sds_design_sync", Status: "queued",
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithChildTaskRetryService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/child-tasks/retry", h.RetryTaskChildTask)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/child-tasks/retry", strings.NewReader(`{"kind":"sds_design_sync"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.Code)
	}
	var body listingkit.TaskChildRetryAccepted
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.TaskID != "task-1" || body.Kind != "sds_design_sync" || body.Status != "queued" {
		t.Fatalf("body = %+v, want queued retry acknowledgement", body)
	}
}

func TestRetryTaskChildTaskReturnsAcceptedForQueuedRetry(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubChildTaskRetryService{accepted: &listingkit.TaskChildRetryAccepted{
		TaskID: "task-1", Kind: "sds_design_sync", Status: "queued",
	}}
	h, err := NewHandler(&stubHandlerCoreService{}, WithChildTaskRetryService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/child-tasks/retry", h.RetryTaskChildTask)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/child-tasks/retry", strings.NewReader(`{"kind":"sds_design_sync"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.Code)
	}
	var body listingkit.TaskChildRetryAccepted
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.TaskID != "task-1" || body.Kind != "sds_design_sync" || body.Status != "queued" {
		t.Fatalf("body = %+v, want queued acknowledgement", body)
	}
}

func TestRetryTaskChildTaskBindsKind(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubChildTaskRetryService{}
	h, err := NewHandler(&stubHandlerCoreService{}, WithChildTaskRetryService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/child-tasks/retry", h.RetryTaskChildTask)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/child-tasks/retry", strings.NewReader(`{"kind":"sds_catalog_product"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.Code)
	}
	if svc.req == nil || svc.req.Kind != "sds_catalog_product" {
		t.Fatalf("child retry req = %+v, want kind bound", svc.req)
	}
}

