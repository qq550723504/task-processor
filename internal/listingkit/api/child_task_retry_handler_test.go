package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func TestRetryTaskChildTaskReturnsBadRequestForEmptyKind(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{}
	h, err := NewHandler(svc)
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
	svc := &stubGenerationTaskService{err: listingkit.ErrChildTaskRetryConflict}
	h, err := NewHandler(svc)
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

func TestRetryTaskChildTaskReturnsTaskResultPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		childRetryResult: &listingkit.TaskResult{
			TaskIdentityFields: listingkit.TaskIdentityFields{TaskID: "task-1"},
			TaskResultLifecycleFields: listingkit.TaskResultLifecycleFields{
				Status: listingkit.TaskStatusCompleted,
			},
			Result: &listingkit.ListingKitResult{
				TaskID: "task-1",
				Status: string(listingkit.TaskStatusCompleted),
			},
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/child-tasks/retry", h.RetryTaskChildTask)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/child-tasks/retry", strings.NewReader(`{"kind":"sds_design_sync"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	var body listingkit.TaskResult
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.TaskID != "task-1" || body.Status != listingkit.TaskStatusCompleted {
		t.Fatalf("body = %+v, want completed task result", body)
	}
}

func TestRetryTaskChildTaskBindsKind(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/child-tasks/retry", h.RetryTaskChildTask)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/child-tasks/retry", strings.NewReader(`{"kind":"sds_catalog_product"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.childRetryReq == nil || svc.childRetryReq.Kind != "sds_catalog_product" {
		t.Fatalf("child retry req = %+v, want kind bound", svc.childRetryReq)
	}
}
