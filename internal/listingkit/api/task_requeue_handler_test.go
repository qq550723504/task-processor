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
)

func TestRequeuePendingTasksHandlerBindsBodyAndReturnsResult(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubTaskRecoveryHandlerService{
		requeueResult: &listingkit.RequeuePendingTasksResult{
			RequeuedTaskIDs: []string{"task-pending"},
			Skipped: []listingkit.TaskRequeueSkip{
				{TaskID: "task-review", Status: listingkit.TaskStatusNeedsReview, Reason: "task status \"needs_review\" is not processable"},
			},
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/requeue", h.RequeuePendingTasks)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/requeue", strings.NewReader(`{"task_ids":["task-pending","task-review"]}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.lastRequeueRequest == nil || len(svc.lastRequeueRequest.TaskIDs) != 2 {
		t.Fatalf("request = %+v, want bound task ids", svc.lastRequeueRequest)
	}

	var body listingkit.RequeuePendingTasksResult
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(body.RequeuedTaskIDs) != 1 || body.RequeuedTaskIDs[0] != "task-pending" {
		t.Fatalf("body = %+v, want requeued result", body)
	}
}

func TestRequeuePendingTasksHandlerRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubTaskRecoveryHandlerService{}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/requeue", h.RequeuePendingTasks)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/requeue", strings.NewReader(`{"task_ids":[]}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func (s *stubTaskRecoveryHandlerService) RequeuePendingTasks(_ context.Context, req *listingkit.RequeuePendingTasksRequest) (*listingkit.RequeuePendingTasksResult, error) {
	s.lastRequeueRequest = req
	return s.requeueResult, s.err
}
