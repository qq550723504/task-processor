//go:build integration

package productenrich_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"task-processor/internal/productenrich"
)

// =============================================================================
// Handler 集成测试（gin + httptest，mock service）
// =============================================================================

// mockHandlerService 实现 ProductHandlerService 接口
type mockHandlerService struct {
	task   *productenrich.Task
	result *productenrich.TaskResult
	err    error
}

func (m *mockHandlerService) CreateGenerateTask(_ context.Context, _ *productenrich.GenerateRequest) (*productenrich.Task, error) {
	return m.task, m.err
}

func (m *mockHandlerService) GetTaskResult(_ context.Context, _ string) (*productenrich.TaskResult, error) {
	return m.result, m.err
}

// setupRouter 创建测试用 gin 路由
func setupRouter(t *testing.T, svc productenrich.ProductHandlerService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler, err := productenrich.NewProductHandler(svc)
	require.NoError(t, err)

	r.POST("/products/generate", handler.GenerateProduct)
	r.GET("/products/tasks/:task_id", handler.GetTaskResult)
	return r
}

func TestHandler_Integration_GenerateProduct(t *testing.T) {
	t.Run("success_returns_200", func(t *testing.T) {
		svc := &mockHandlerService{
			task: &productenrich.Task{
				ID:     "task-001",
				Status: productenrich.TaskStatusPending,
			},
		}
		r := setupRouter(t, svc)

		body, _ := json.Marshal(productenrich.GenerateRequest{Text: "高品质蓝牙耳机"})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/products/generate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp productenrich.TaskResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "task-001", resp.TaskID)
		assert.Equal(t, string(productenrich.TaskStatusPending), resp.Status)
	})

	t.Run("invalid_json_returns_400", func(t *testing.T) {
		svc := &mockHandlerService{}
		r := setupRouter(t, svc)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/products/generate", bytes.NewReader([]byte("not-json")))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var resp productenrich.ErrorResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "invalid_request", resp.Error)
	})

	t.Run("service_error_returns_500", func(t *testing.T) {
		svc := &mockHandlerService{err: fmt.Errorf("db connection failed")}
		r := setupRouter(t, svc)

		body, _ := json.Marshal(productenrich.GenerateRequest{Text: "test"})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/products/generate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var resp productenrich.ErrorResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "task_creation_failed", resp.Error)
	})
}

func TestHandler_Integration_GetTaskResult(t *testing.T) {
	t.Run("found_returns_200", func(t *testing.T) {
		status := productenrich.TaskStatusCompleted
		svc := &mockHandlerService{
			result: &productenrich.TaskResult{
				TaskID: "task-001",
				Status: status,
			},
		}
		r := setupRouter(t, svc)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/products/tasks/task-001", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp productenrich.TaskResult
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "task-001", resp.TaskID)
	})

	t.Run("not_found_returns_404", func(t *testing.T) {
		svc := &mockHandlerService{err: productenrich.ErrTaskNotFound}
		r := setupRouter(t, svc)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/products/tasks/nonexistent", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var resp productenrich.ErrorResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "task_not_found", resp.Error)
	})

	t.Run("service_error_returns_500", func(t *testing.T) {
		svc := &mockHandlerService{err: fmt.Errorf("internal error")}
		r := setupRouter(t, svc)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/products/tasks/task-x", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
