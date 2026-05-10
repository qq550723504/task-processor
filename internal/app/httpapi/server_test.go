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
	listQueueCalled bool
	getResultCalled bool
	workbenchCalled bool
	reviewCalled    bool
	submitCalled    bool
}

type stubTaskRPCHandler struct {
	healthCalled     bool
	statusCalled     bool
	retryCalled      bool
	cancelCalled     bool
	queueStatsCalled bool
}

func (s *stubTaskRPCHandler) GetHealth(c *gin.Context) {
	s.healthCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *stubAmazonListingHandler) GenerateListing(c *gin.Context) {
	s.generateCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": "listing-task"})
}

func (s *stubAmazonListingHandler) ListTaskQueue(c *gin.Context) {
	s.listQueueCalled = true
	c.JSON(http.StatusOK, gin.H{"count": 1})
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

func (s *stubTaskRPCHandler) GetTaskStatus(c *gin.Context) {
	s.statusCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "canonicalStatus": "processing"})
}

func (s *stubTaskRPCHandler) RetryTask(c *gin.Context) {
	s.retryCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "canonicalStatus": "retried"})
}

func (s *stubTaskRPCHandler) CancelTask(c *gin.Context) {
	s.cancelCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "canonicalStatus": "cancelled"})
}

func (s *stubTaskRPCHandler) GetQueueStats(c *gin.Context) {
	s.queueStatsCalled = true
	c.JSON(http.StatusOK, gin.H{"queueStats": "ok"})
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

type stubListingKitHandler struct {
	generateCalled                     bool
	generateStudioDesignsCalled        bool
	generateStudioProductImagesCalled  bool
	uploadImagesCalled                 bool
	getUploadedImageCalled             bool
	listTasksCalled                    bool
	getResultCalled                    bool
	getPreviewCalled                   bool
	getGenerationCalled                bool
	getGenerationQueueCalled           bool
	getGenerationReviewSessionCalled   bool
	getGenerationReviewPreviewCalled   bool
	dispatchGenerationNavigationCalled bool
	retryGenerationCalled              bool
	executeGenerationActionCalled      bool
	getHistoryCalled                   bool
	getHistoryDetailCalled             bool
	getExportCalled                    bool
	revisionCalled                     bool
	validateCalled                     bool
	searchSheinCategoriesCalled        bool
	submitCalled                       bool
}

func (s *stubListingKitHandler) GenerateListingKit(c *gin.Context) {
	s.generateCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": "listing-kit-task"})
}

func (s *stubListingKitHandler) GenerateStudioDesigns(c *gin.Context) {
	s.generateStudioDesignsCalled = true
	c.JSON(http.StatusOK, gin.H{"images": []any{}})
}

func (s *stubListingKitHandler) GenerateStudioProductImages(c *gin.Context) {
	s.generateStudioProductImagesCalled = true
	c.JSON(http.StatusOK, gin.H{"images": []any{}})
}

func (s *stubListingKitHandler) UploadListingKitImages(c *gin.Context) {
	s.uploadImagesCalled = true
	c.JSON(http.StatusOK, gin.H{"image_urls": []string{"/api/v1/listing-kits/uploads/files/test.jpg"}})
}

func (s *stubListingKitHandler) GetUploadedListingKitImage(c *gin.Context) {
	s.getUploadedImageCalled = true
	c.Data(http.StatusOK, "image/jpeg", []byte{0xFF, 0xD8, 0xFF})
}

func (s *stubListingKitHandler) ListTasks(c *gin.Context) {
	s.listTasksCalled = true
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) GetTaskResult(c *gin.Context) {
	s.getResultCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id")})
}

func (s *stubListingKitHandler) GetTaskPreview(c *gin.Context) {
	s.getPreviewCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "selected_platform": c.Query("platform")})
}

func (s *stubListingKitHandler) GetTaskGenerationTasks(c *gin.Context) {
	s.getGenerationCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "tasks": []any{}})
}

func (s *stubListingKitHandler) GetTaskGenerationQueue(c *gin.Context) {
	s.getGenerationQueueCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "items": []any{}})
}

func (s *stubListingKitHandler) GetTaskGenerationReviewSession(c *gin.Context) {
	s.getGenerationReviewSessionCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "slot": c.Query("slot")})
}

func (s *stubListingKitHandler) GetTaskGenerationReviewPreview(c *gin.Context) {
	s.getGenerationReviewPreviewCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "slot": c.Query("slot")})
}

func (s *stubListingKitHandler) DispatchTaskGenerationNavigation(c *gin.Context) {
	s.dispatchGenerationNavigationCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "dispatch_kind": "session"})
}

func (s *stubListingKitHandler) RetryTaskGenerationTasks(c *gin.Context) {
	s.retryGenerationCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "retried"})
}

func (s *stubListingKitHandler) ExecuteTaskGenerationAction(c *gin.Context) {
	s.executeGenerationActionCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "executed"})
}

func (s *stubListingKitHandler) GetTaskRevisionHistory(c *gin.Context) {
	s.getHistoryCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "limit": c.Query("limit")})
}

func (s *stubListingKitHandler) GetTaskRevisionHistoryDetail(c *gin.Context) {
	s.getHistoryDetailCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "revision_id": c.Param("revision_id")})
}

func (s *stubListingKitHandler) GetTaskExport(c *gin.Context) {
	s.getExportCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "selected_platform": c.Query("platform")})
}

func (s *stubListingKitHandler) ApplyTaskRevision(c *gin.Context) {
	s.revisionCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "revised"})
}

func (s *stubListingKitHandler) ValidateTaskRevision(c *gin.Context) {
	s.validateCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "valid": false})
}

func (s *stubListingKitHandler) SubmitTask(c *gin.Context) {
	s.submitCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "submitted"})
}

func (s *stubListingKitHandler) RefreshSubmissionStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "refreshed"})
}

func (s *stubListingKitHandler) GetSheinSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"default_store_id": 869})
}

func (s *stubListingKitHandler) UpdateSheinSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"default_store_id": 869})
}

func (s *stubListingKitHandler) GetAIClientSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"client_name": "default", "api_key_set": false})
}

func (s *stubListingKitHandler) UpdateAIClientSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"client_name": "default", "api_key_set": true})
}

func (s *stubListingKitHandler) PreviewSheinPrice(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ready": true})
}

func (s *stubListingKitHandler) SearchSheinCategories(c *gin.Context) {
	s.searchSheinCategoriesCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "query": c.Query("query"), "items": []any{}})
}

func (s *stubListingKitHandler) UpdateSheinFinalDraft(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id")})
}

func (s *stubListingKitHandler) GetSubmissionEvents(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "items": []any{}})
}

func TestRegisterRoutes_AmazonListingEndpoints(t *testing.T) {
	t.Parallel()

	handler := &stubAmazonListingHandler{}
	router := gin.New()
	RegisterRoutes(router, nil, nil, handler, nil, nil)

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
			name:   "list queue",
			method: http.MethodGet,
			path:   "/api/v1/amazon/listings/tasks",
			assertFn: func(t *testing.T) {
				if !handler.listQueueCalled {
					t.Fatal("ListTaskQueue handler was not called")
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
	RegisterRoutes(router, handler, nil, nil, nil, nil)

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
	RegisterRoutes(router, nil, handler, nil, nil, nil)

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

func TestRegisterRoutes_ListingKitEndpoints(t *testing.T) {
	t.Parallel()

	handler := &stubListingKitHandler{}
	router := gin.New()
	RegisterRoutes(router, nil, nil, nil, handler, nil)

	body, _ := json.Marshal(map[string]any{
		"text":      "test",
		"platforms": []string{"amazon", "shein"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/generate = %d, want 200", resp.Code)
	}
	if !handler.generateCalled {
		t.Fatal("GenerateListingKit handler was not called")
	}

	handler.generateStudioDesignsCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/designs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/studio/designs = %d, want 200", resp.Code)
	}
	if !handler.generateStudioDesignsCalled {
		t.Fatal("listing kit GenerateStudioDesigns handler was not called")
	}

	handler.generateStudioProductImagesCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/product-images", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/studio/product-images = %d, want 200", resp.Code)
	}
	if !handler.generateStudioProductImagesCalled {
		t.Fatal("listing kit GenerateStudioProductImages handler was not called")
	}

	handler.uploadImagesCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/uploads/images", bytes.NewReader([]byte("--x--")))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/uploads/images = %d, want 200", resp.Code)
	}
	if !handler.uploadImagesCalled {
		t.Fatal("listing kit UploadListingKitImages handler was not called")
	}

	handler.getUploadedImageCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/uploads/files/test.jpg", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/uploads/files/test.jpg = %d, want 200", resp.Code)
	}
	if !handler.getUploadedImageCalled {
		t.Fatal("listing kit GetUploadedListingKitImage handler was not called")
	}

	handler.getResultCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123 = %d, want 200", resp.Code)
	}
	if !handler.getResultCalled {
		t.Fatal("listing kit GetTaskResult handler was not called")
	}

	handler.getPreviewCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/preview?platform=shein", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/preview = %d, want 200", resp.Code)
	}
	if !handler.getPreviewCalled {
		t.Fatal("listing kit GetTaskPreview handler was not called")
	}

	handler.getHistoryCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/revision-history?limit=5", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/revision-history = %d, want 200", resp.Code)
	}
	if !handler.getHistoryCalled {
		t.Fatal("listing kit GetTaskRevisionHistory handler was not called")
	}

	handler.getGenerationCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/generation-tasks", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/generation-tasks = %d, want 200", resp.Code)
	}
	if !handler.getGenerationCalled {
		t.Fatal("listing kit GetTaskGenerationTasks handler was not called")
	}

	handler.getGenerationQueueCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/generation-queue", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/generation-queue = %d, want 200", resp.Code)
	}
	if !handler.getGenerationQueueCalled {
		t.Fatal("listing kit GetTaskGenerationQueue handler was not called")
	}

	handler.getGenerationReviewSessionCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/generation-review-session?platform=shein&slot=main", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/generation-review-session = %d, want 200", resp.Code)
	}
	if !handler.getGenerationReviewSessionCalled {
		t.Fatal("listing kit GetTaskGenerationReviewSession handler was not called")
	}

	handler.getGenerationReviewPreviewCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/generation-review-preview?platform=shein&slot=main", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/generation-review-preview = %d, want 200", resp.Code)
	}
	if !handler.getGenerationReviewPreviewCalled {
		t.Fatal("listing kit GetTaskGenerationReviewPreview handler was not called")
	}

	handler.dispatchGenerationNavigationCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-123/generation-navigation/dispatch", bytes.NewReader([]byte(`{"target":{"dispatch_kind":"session","session_query":{"platform":"shein","slot":"main"}}}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/tasks/task-123/generation-navigation/dispatch = %d, want 200", resp.Code)
	}
	if !handler.dispatchGenerationNavigationCalled {
		t.Fatal("listing kit DispatchTaskGenerationNavigation handler was not called")
	}

	handler.retryGenerationCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-123/generation-tasks/retry", bytes.NewReader([]byte(`{"task_ids":["amazon:amazon-lifestyle"]}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/tasks/task-123/generation-tasks/retry = %d, want 200", resp.Code)
	}
	if !handler.retryGenerationCalled {
		t.Fatal("listing kit RetryTaskGenerationTasks handler was not called")
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-123/generation-actions/execute", bytes.NewReader([]byte(`{"action_key":"generate_missing_assets"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/tasks/task-123/generation-actions/execute = %d, want 200", resp.Code)
	}
	if !handler.executeGenerationActionCalled {
		t.Fatal("listing kit ExecuteTaskGenerationAction handler was not called")
	}

	handler.getHistoryDetailCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/revision-history/rev-123", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/revision-history/rev-123 = %d, want 200", resp.Code)
	}
	if !handler.getHistoryDetailCalled {
		t.Fatal("listing kit GetTaskRevisionHistoryDetail handler was not called")
	}

	handler.getExportCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/export?platform=shein", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/export = %d, want 200", resp.Code)
	}
	if !handler.getExportCalled {
		t.Fatal("listing kit GetTaskExport handler was not called")
	}

	handler.revisionCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-123/revision", bytes.NewReader([]byte(`{"platform":"shein","shein":{"spu_name":"updated"}}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/tasks/task-123/revision = %d, want 200", resp.Code)
	}
	if !handler.revisionCalled {
		t.Fatal("listing kit ApplyTaskRevision handler was not called")
	}

	handler.searchSheinCategoriesCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/shein/categories?query=mask", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/shein/categories = %d, want 200", resp.Code)
	}
	if !handler.searchSheinCategoriesCalled {
		t.Fatal("listing kit SearchSheinCategories handler was not called")
	}

	handler.validateCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-123/revision/validate", bytes.NewReader([]byte(`{"platform":"shein","shein":{"category_id":0}}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/tasks/task-123/revision/validate = %d, want 200", resp.Code)
	}
	if !handler.validateCalled {
		t.Fatal("listing kit ValidateTaskRevision handler was not called")
	}
}

func TestRegisterRoutes_NilHandlersDoNotExposeModuleRoutes(t *testing.T) {
	t.Parallel()

	router := gin.New()
	RegisterRoutes(router, nil, nil, nil, nil, nil)

	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/v1/products/generate"},
		{method: http.MethodPost, path: "/api/v1/images/process"},
		{method: http.MethodPost, path: "/api/v1/amazon/listings/generate"},
		{method: http.MethodPost, path: "/api/v1/listing-kits/generate"},
		{method: http.MethodGet, path: "/api/v1/management/tasks/health"},
		{method: http.MethodGet, path: "/api/v1/management/tasks/123/status"},
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

func TestRegisterRoutes_TaskRPCEndpoints(t *testing.T) {
	t.Parallel()

	handler := &stubTaskRPCHandler{}
	router := gin.New()
	RegisterRoutes(router, nil, nil, nil, nil, handler)

	tests := []struct {
		name     string
		method   string
		path     string
		assertFn func(*testing.T)
	}{
		{
			name:   "health",
			method: http.MethodGet,
			path:   "/api/v1/management/tasks/health",
			assertFn: func(t *testing.T) {
				if !handler.healthCalled {
					t.Fatal("GetHealth handler was not called")
				}
			},
		},
		{
			name:   "status",
			method: http.MethodGet,
			path:   "/api/v1/management/tasks/123/status",
			assertFn: func(t *testing.T) {
				if !handler.statusCalled {
					t.Fatal("GetTaskStatus handler was not called")
				}
			},
		},
		{
			name:   "retry",
			method: http.MethodPost,
			path:   "/api/v1/management/tasks/123/retry",
			assertFn: func(t *testing.T) {
				if !handler.retryCalled {
					t.Fatal("RetryTask handler was not called")
				}
			},
		},
		{
			name:   "cancel",
			method: http.MethodPost,
			path:   "/api/v1/management/tasks/123/cancel",
			assertFn: func(t *testing.T) {
				if !handler.cancelCalled {
					t.Fatal("CancelTask handler was not called")
				}
			},
		},
		{
			name:   "queue-stats",
			method: http.MethodGet,
			path:   "/api/v1/management/tasks/queue-stats",
			assertFn: func(t *testing.T) {
				if !handler.queueStatsCalled {
					t.Fatal("GetQueueStats handler was not called")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("%s %s = %d, want 200", tt.method, tt.path, resp.Code)
			}
			tt.assertFn(t)
		})
	}
}

func TestBuildRouteDescriptorsMatchMountedRoutes(t *testing.T) {
	t.Parallel()

	router := gin.New()
	productHandler := &stubProductHandler{}
	imageHandler := &stubImageHandler{}
	amazonHandler := &stubAmazonListingHandler{}
	listingKitHandler := &stubListingKitHandler{}
	taskRPCHandler := &stubTaskRPCHandler{}

	routes := buildRouteDescriptors(productHandler, imageHandler, amazonHandler, listingKitHandler, nil, taskRPCHandler)
	mountRoutes(router, routes)

	registered := router.Routes()
	if len(registered) != len(routes) {
		t.Fatalf("registered routes = %d, want %d", len(registered), len(routes))
	}

	index := make(map[string]struct{}, len(registered))
	for _, route := range registered {
		index[route.Method+" "+route.Path] = struct{}{}
	}

	for _, route := range routes {
		key := route.Method + " " + route.Path
		if _, ok := index[key]; !ok {
			t.Fatalf("expected mounted route %s", key)
		}
	}
}
