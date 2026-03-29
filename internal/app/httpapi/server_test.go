package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type stubAmazonListingHandler struct {
	generateCalled  bool
	getResultCalled bool
	workbenchCalled bool
	reviewCalled    bool
	submitCalled    bool
}

func (s *stubAmazonListingHandler) GenerateListing(c *gin.Context) {
	s.generateCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": "listing-task"})
}

func (s *stubAmazonListingHandler) GetTaskResult(c *gin.Context) {
	s.getResultCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id")})
}

func (s *stubAmazonListingHandler) GetTaskWorkbench(c *gin.Context) {
	s.workbenchCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "ready": true})
}

func (s *stubAmazonListingHandler) ReviewTask(c *gin.Context) {
	s.reviewCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "needs_review"})
}

func (s *stubAmazonListingHandler) SubmitTask(c *gin.Context) {
	s.submitCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "submitted"})
}

type stubProductHandler struct {
	generateCalled  bool
	getResultCalled bool
}

func (s *stubProductHandler) GenerateProduct(c *gin.Context) {
	s.generateCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": "product-task"})
}

func (s *stubProductHandler) GetTaskResult(c *gin.Context) {
	s.getResultCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id")})
}

type stubImageHandler struct {
	processCalled   bool
	getResultCalled bool
	reviewCalled    bool
}

func (s *stubImageHandler) ProcessImages(c *gin.Context) {
	s.processCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": "image-task"})
}

func (s *stubImageHandler) GetTaskResult(c *gin.Context) {
	s.getResultCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id")})
}

func (s *stubImageHandler) ReviewTask(c *gin.Context) {
	s.reviewCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "reviewed"})
}

func TestRegisterRoutes_AmazonListingEndpoints(t *testing.T) {
	t.Parallel()

	handler := &stubAmazonListingHandler{}
	router := gin.New()
	RegisterRoutes(router, nil, nil, handler)

	tests := []struct {
		name     string
		method   string
		path     string
		body     any
		assertFn func(*testing.T)
	}{
		{
			name:   "generate",
			method: http.MethodPost,
			path:   "/api/v1/amazon/listings/generate",
			body: map[string]any{
				"marketplace": "amazon",
			},
			assertFn: func(t *testing.T) {
				if !handler.generateCalled {
					t.Fatal("GenerateListing handler was not called")
				}
			},
		},
		{
			name:   "get result",
			method: http.MethodGet,
			path:   "/api/v1/amazon/listings/tasks/task-123",
			assertFn: func(t *testing.T) {
				if !handler.getResultCalled {
					t.Fatal("GetTaskResult handler was not called")
				}
			},
		},
		{
			name:   "get workbench",
			method: http.MethodGet,
			path:   "/api/v1/amazon/listings/tasks/task-123/workbench",
			assertFn: func(t *testing.T) {
				if !handler.workbenchCalled {
					t.Fatal("GetTaskWorkbench handler was not called")
				}
			},
		},
		{
			name:   "review",
			method: http.MethodPost,
			path:   "/api/v1/amazon/listings/tasks/task-123/review",
			body: map[string]any{
				"action": "approve",
			},
			assertFn: func(t *testing.T) {
				if !handler.reviewCalled {
					t.Fatal("ReviewTask handler was not called")
				}
			},
		},
		{
			name:   "submit",
			method: http.MethodPost,
			path:   "/api/v1/amazon/listings/tasks/task-123/submit",
			body: map[string]any{
				"action": "preview",
			},
			assertFn: func(t *testing.T) {
				if !handler.submitCalled {
					t.Fatal("SubmitTask handler was not called")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyReader *bytes.Reader
			if tt.body != nil {
				payload, err := json.Marshal(tt.body)
				if err != nil {
					t.Fatalf("marshal request: %v", err)
				}
				bodyReader = bytes.NewReader(payload)
			} else {
				bodyReader = bytes.NewReader(nil)
			}

			req := httptest.NewRequest(tt.method, tt.path, bodyReader)
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("%s %s = %d, want 200", tt.method, tt.path, resp.Code)
			}
			tt.assertFn(t)
		})
	}
}

func TestRegisterRoutes_ProductEndpoints(t *testing.T) {
	t.Parallel()

	handler := &stubProductHandler{}
	router := gin.New()
	RegisterRoutes(router, handler, nil, nil)

	// generate endpoint
	generatePayload := map[string]any{"text": "test"}
	body, _ := json.Marshal(generatePayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/products/generate = %d, want 200", resp.Code)
	}
	if !handler.generateCalled {
		t.Fatal("GenerateProduct handler was not called")
	}

	// get task result
	handler.getResultCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/products/tasks/task-123", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/products/tasks/task-123 = %d, want 200", resp.Code)
	}
	if !handler.getResultCalled {
		t.Fatal("GetTaskResult handler was not called")
	}
}

func TestRegisterRoutes_ImageEndpoints(t *testing.T) {
	t.Parallel()

	handler := &stubImageHandler{}
	router := gin.New()
	RegisterRoutes(router, nil, handler, nil)

	// process endpoint
	processPayload := map[string]any{"image_urls": []string{"https://example.com/1.jpg"}, "marketplace": "amazon"}
	body, _ := json.Marshal(processPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/process", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/images/process = %d, want 200", resp.Code)
	}
	if !handler.processCalled {
		t.Fatal("ProcessImages handler was not called")
	}

	// get task result
	handler.getResultCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/images/tasks/task-123", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/images/tasks/task-123 = %d, want 200", resp.Code)
	}
	if !handler.getResultCalled {
		t.Fatal("image GetTaskResult handler was not called")
	}

	// review endpoint
	handler.reviewCalled = false
	reviewPayload := map[string]any{"action": "approve"}
	body, _ = json.Marshal(reviewPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/images/tasks/task-123/review", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/images/tasks/task-123/review = %d, want 200", resp.Code)
	}
	if !handler.reviewCalled {
		t.Fatal("ReviewTask handler was not called")
	}
}

func TestRegisterRoutes_NilHandlersDoNotExposeModuleRoutes(t *testing.T) {
	t.Parallel()

	router := gin.New()
	RegisterRoutes(router, nil, nil, nil)

	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/v1/products/generate"},
		{method: http.MethodPost, path: "/api/v1/images/process"},
		{method: http.MethodPost, path: "/api/v1/amazon/listings/generate"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("%s %s = %d, want 404", tt.method, tt.path, resp.Code)
		}
	}
}
