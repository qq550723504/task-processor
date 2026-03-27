package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	productimage "task-processor/internal/productimage"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type mockImageHandlerSvc struct {
	createResult *productimage.Task
	createErr    error
	getResult    *productimage.TaskResult
	getErr       error
	reviewResult *productimage.TaskResult
	reviewErr    error
}

func (m *mockImageHandlerSvc) CreateProcessTask(_ context.Context, _ *productimage.ImageProcessRequest) (*productimage.Task, error) {
	return m.createResult, m.createErr
}

func (m *mockImageHandlerSvc) GetTaskResult(_ context.Context, _ string) (*productimage.TaskResult, error) {
	return m.getResult, m.getErr
}

func (m *mockImageHandlerSvc) ReviewTask(_ context.Context, _ string, _ *productimage.ReviewTaskRequest) (*productimage.TaskResult, error) {
	return m.reviewResult, m.reviewErr
}

func newTestRouter(svc productimage.HandlerService) *gin.Engine {
	h, _ := NewImageHandler(svc)
	r := gin.New()
	r.POST("/images/process", h.ProcessImages)
	r.GET("/images/tasks/:task_id", h.GetTaskResult)
	r.POST("/images/tasks/:task_id/review", h.ReviewTask)
	return r
}

func TestNewImageHandler_NilService_ReturnsError(t *testing.T) {
	_, err := NewImageHandler(nil)
	if err == nil {
		t.Fatal("expected error for nil service")
	}
}

func TestProcessImages_ValidRequest_Returns200(t *testing.T) {
	now := time.Now()
	svc := &mockImageHandlerSvc{createResult: &productimage.Task{ID: "img-123", Status: productimage.TaskStatusPending, CreatedAt: now}}
	r := newTestRouter(svc)

	body := `{"image_urls":["http://example.com/img.jpg"],"marketplace":"amazon"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/images/process", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["task_id"] != "img-123" {
		t.Errorf("task_id = %v, want img-123", resp["task_id"])
	}
}

func TestProcessImages_InvalidJSON_Returns400(t *testing.T) {
	r := newTestRouter(&mockImageHandlerSvc{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/images/process", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestProcessImages_InvalidRequestError_Returns400(t *testing.T) {
	svc := &mockImageHandlerSvc{createErr: errors.New("invalid request: marketplace is required")}
	r := newTestRouter(svc)
	body := `{"image_urls":["http://example.com/img.jpg"]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/images/process", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestGetTaskResult_NotFound_Returns404(t *testing.T) {
	svc := &mockImageHandlerSvc{getErr: productimage.ErrTaskNotFound}
	r := newTestRouter(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/images/tasks/not-found", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestGetTaskResult_ExistingTask_Returns200(t *testing.T) {
	now := time.Now()
	svc := &mockImageHandlerSvc{getResult: &productimage.TaskResult{TaskID: "img-abc", Status: productimage.TaskStatusCompleted, CreatedAt: now}}
	r := newTestRouter(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/images/tasks/img-abc", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestReviewTask_Approve_Returns200(t *testing.T) {
	now := time.Now()
	svc := &mockImageHandlerSvc{reviewResult: &productimage.TaskResult{TaskID: "img-review", Status: productimage.TaskStatusCompleted, CreatedAt: now}}
	r := newTestRouter(svc)
	body := `{"action":"approve"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/images/tasks/img-review/review", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestReviewTask_InvalidRequest_Returns400(t *testing.T) {
	svc := &mockImageHandlerSvc{reviewErr: errors.New("invalid request: unsupported review action")}
	r := newTestRouter(svc)
	body := `{"action":"noop"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/images/tasks/img-review/review", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
