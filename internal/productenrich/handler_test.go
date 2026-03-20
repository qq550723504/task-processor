package productenrich_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"task-processor/internal/productenrich"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// mockHandlerSvc 瀹炵幇 ProductHandlerService锛堥伩鍏嶄笌 integration_handler_test.go 涓殑 mockHandlerSvc 鍐茬獊锛?
type mockHandlerSvc struct {
	createResult *productenrich.Task
	createErr    error
	getResult    *productenrich.TaskResult
	getErr       error
}

func (m *mockHandlerSvc) CreateGenerateTask(_ context.Context, _ *productenrich.GenerateRequest) (*productenrich.Task, error) {
	return m.createResult, m.createErr
}
func (m *mockHandlerSvc) GetTaskResult(_ context.Context, _ string) (*productenrich.TaskResult, error) {
	return m.getResult, m.getErr
}

func newTestRouter(svc productenrich.ProductHandlerService) *gin.Engine {
	h, _ := productenrich.NewProductHandler(svc)
	r := gin.New()
	r.POST("/generate", h.GenerateProduct)
	r.GET("/tasks/:task_id", h.GetTaskResult)
	return r
}

// --- NewProductHandler ---

func TestNewProductHandler_NilService_ReturnsError(t *testing.T) {
	_, err := productenrich.NewProductHandler(nil)
	if err == nil {
		t.Fatal("expected error for nil service")
	}
}

// --- GenerateProduct ---

func TestGenerateProduct_ValidRequest_Returns200(t *testing.T) {
	now := time.Now()
	svc := &mockHandlerSvc{
		createResult: &productenrich.Task{
			ID:        "task-123",
			Status:    productenrich.TaskStatusPending,
			CreatedAt: now,
		},
	}
	r := newTestRouter(svc)

	body := `{"image_urls":["http://example.com/img.jpg"]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/generate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["task_id"] != "task-123" {
		t.Errorf("task_id = %v, want task-123", resp["task_id"])
	}
	if resp["status"] != "pending" {
		t.Errorf("status = %v, want pending", resp["status"])
	}
}

func TestGenerateProduct_InvalidJSON_Returns400(t *testing.T) {
	r := newTestRouter(&mockHandlerSvc{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/generate", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "invalid_request" {
		t.Errorf("error = %v, want invalid_request", resp["error"])
	}
}

func TestGenerateProduct_ServiceError_Returns500(t *testing.T) {
	svc := &mockHandlerSvc{createErr: productenrich.ErrTaskNotFound} // 浠绘剰闈?nil 閿欒
	r := newTestRouter(svc)

	body := `{"text":"a product"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/generate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "task_creation_failed" {
		t.Errorf("error = %v, want task_creation_failed", resp["error"])
	}
}

// --- GetTaskResult ---

func TestGetTaskResult_ExistingTask_Returns200(t *testing.T) {
	now := time.Now()
	svc := &mockHandlerSvc{
		getResult: &productenrich.TaskResult{
			TaskID:    "task-abc",
			Status:    productenrich.TaskStatusCompleted,
			CreatedAt: now,
		},
	}
	r := newTestRouter(svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks/task-abc", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["task_id"] != "task-abc" {
		t.Errorf("task_id = %v, want task-abc", resp["task_id"])
	}
}

func TestGetTaskResult_NotFound_Returns404(t *testing.T) {
	svc := &mockHandlerSvc{getErr: productenrich.ErrTaskNotFound}
	r := newTestRouter(svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks/nonexistent", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "task_not_found" {
		t.Errorf("error = %v, want task_not_found", resp["error"])
	}
}

func TestGetTaskResult_ServiceError_Returns500(t *testing.T) {
	svc := &mockHandlerSvc{getErr: errors.New("internal db error")}
	r := newTestRouter(svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks/task-xyz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "query_failed" {
		t.Errorf("error = %v, want query_failed", resp["error"])
	}
}

