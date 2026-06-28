package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"task-processor/internal/productenrich"
	productapi "task-processor/internal/productenrich/api"
	productpipeline "task-processor/internal/productenrich/pipeline"
	"task-processor/internal/productimage"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
	productimagestore "task-processor/internal/productimage/store"
)

func init() {
	gin.SetMode(gin.TestMode)
	logrus.SetLevel(logrus.FatalLevel)
}

type stubProductHandler struct {
	generateCalled  bool
	getResultCalled bool
}

func (s *stubProductHandler) GenerateProduct(c *gin.Context) {
	s.generateCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": "product-stub-task"})
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
	c.JSON(http.StatusOK, gin.H{"task_id": "image-stub-task"})
}

func (s *stubImageHandler) GetTaskResult(c *gin.Context) {
	s.getResultCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id")})
}

func (s *stubImageHandler) ReviewTask(c *gin.Context) {
	s.reviewCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": productimage.TaskStatusCompleted})
}

func TestRegisterRoutes_HealthCheck(t *testing.T) {
	r := gin.New()
	registerRoutes(r, &stubProductHandler{}, &stubImageHandler{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /health = %d, want 200", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal health response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status = %q, want ok", body["status"])
	}
}

func TestRegisterRoutes_GenerateProduct(t *testing.T) {
	h := &stubProductHandler{}
	r := gin.New()
	registerRoutes(r, h, &stubImageHandler{})

	body, _ := json.Marshal(productenrich.GenerateRequest{Text: "test"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/products/generate = %d, want 200", w.Code)
	}
	if !h.generateCalled {
		t.Fatal("GenerateProduct handler was not called")
	}
}

func TestRegisterRoutes_GetProductTaskResult(t *testing.T) {
	h := &stubProductHandler{}
	r := gin.New()
	registerRoutes(r, h, &stubImageHandler{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/tasks/task-123", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/products/tasks/:task_id = %d, want 200", w.Code)
	}
	if !h.getResultCalled {
		t.Fatal("product GetTaskResult handler was not called")
	}
}

func TestRegisterRoutes_ProcessImages(t *testing.T) {
	h := &stubImageHandler{}
	r := gin.New()
	registerRoutes(r, &stubProductHandler{}, h)

	body, _ := json.Marshal(productimage.ImageProcessRequest{
		ImageURLs:   []string{"https://example.com/hero.jpg"},
		Marketplace: "amazon",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/process", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/images/process = %d, want 200", w.Code)
	}
	if !h.processCalled {
		t.Fatal("ProcessImages handler was not called")
	}
}

func TestRegisterRoutes_GetImageTaskResult(t *testing.T) {
	h := &stubImageHandler{}
	r := gin.New()
	registerRoutes(r, &stubProductHandler{}, h)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/images/tasks/task-123", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/images/tasks/:task_id = %d, want 200", w.Code)
	}
	if !h.getResultCalled {
		t.Fatal("image GetTaskResult handler was not called")
	}
}

func TestRegisterRoutes_ReviewImageTask(t *testing.T) {
	h := &stubImageHandler{}
	r := gin.New()
	registerRoutes(r, &stubProductHandler{}, h)

	body, _ := json.Marshal(productimage.ReviewTaskRequest{Action: "approve"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/tasks/task-123/review", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/images/tasks/:task_id/review = %d, want 200", w.Code)
	}
	if !h.reviewCalled {
		t.Fatal("image ReviewTask handler was not called")
	}
}

func TestRegisterRoutes_UnknownPath_Returns404(t *testing.T) {
	r := gin.New()
	registerRoutes(r, &stubProductHandler{}, &stubImageHandler{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown path = %d, want 404", w.Code)
	}
}

type mockTaskSubmitter struct {
	submitted []string
	err       error
}

func (m *mockTaskSubmitter) Submit(taskID string) error {
	if m.err != nil {
		return m.err
	}
	m.submitted = append(m.submitted, taskID)
	return nil
}

func TestProductService_SetWorkerPool_SubmitsJobOnCreate(t *testing.T) {
	ctx := context.Background()
	repo := newMemTaskRepository()
	redis := newMemRedisClient()
	submitter := &mockTaskSubmitter{}

	svc, err := productenrich.NewProductService(&productenrich.ProductServiceConfig{
		QueueName:            "test_queue",
		TaskRepo:             repo,
		RedisClient:          redis,
		InputValidator:       productenrich.NewInputValidator(nil),
		QualityScorer:        productenrich.NewQualityScorer(nil),
		StrategySelector:     productenrich.NewStrategySelector(nil),
		EnhancementSuggester: productenrich.NewEnhancementSuggester(),
		ResultValidator:      productenrich.NewResultValidator(),
	})
	if err != nil {
		t.Fatalf("NewProductService: %v", err)
	}

	svc.SetTaskSubmitter(submitter)
	task, err := svc.CreateGenerateTask(ctx, &productenrich.GenerateRequest{Text: "bluetooth earphones"})
	if err != nil {
		t.Fatalf("CreateGenerateTask: %v", err)
	}

	if len(submitter.submitted) != 1 {
		t.Fatalf("submitted jobs = %d, want 1", len(submitter.submitted))
	}
	if submitter.submitted[0] != task.ID {
		t.Fatalf("submitted task ID = %q, want %q", submitter.submitted[0], task.ID)
	}
}

func TestProductService_WithoutWorkerPool_FallsBackToRedis(t *testing.T) {
	ctx := context.Background()
	repo := newMemTaskRepository()
	redis := newMemRedisClient()

	svc, err := productenrich.NewProductService(&productenrich.ProductServiceConfig{
		QueueName:            "fallback_queue",
		TaskRepo:             repo,
		RedisClient:          redis,
		InputValidator:       productenrich.NewInputValidator(nil),
		QualityScorer:        productenrich.NewQualityScorer(nil),
		StrategySelector:     productenrich.NewStrategySelector(nil),
		EnhancementSuggester: productenrich.NewEnhancementSuggester(),
		ResultValidator:      productenrich.NewResultValidator(),
	})
	if err != nil {
		t.Fatalf("NewProductService: %v", err)
	}

	task, err := svc.CreateGenerateTask(ctx, &productenrich.GenerateRequest{Text: "product description"})
	if err != nil {
		t.Fatalf("CreateGenerateTask: %v", err)
	}
	if task.Status != productenrich.TaskStatusPending {
		t.Fatalf("Status = %q, want pending", task.Status)
	}
}

func TestProcessor_StartAndClose_NoError(t *testing.T) {
	repo := newMemTaskRepository()
	svc, err := productenrich.NewProductService(&productenrich.ProductServiceConfig{
		QueueName:   "lc_queue",
		TaskRepo:    repo,
		RedisClient: newMemRedisClient(),
	})
	if err != nil {
		t.Fatalf("NewProductService: %v", err)
	}

	proc, err := productpipeline.NewProcessor(svc, repo, logrus.New(), 3)
	if err != nil {
		t.Fatalf("NewProcessor: %v", err)
	}

	ctx := context.Background()
	if err := proc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	proc.Close(ctx)
}

func buildTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	repo := newMemTaskRepository()
	redis := newMemRedisClient()
	submitter := &mockTaskSubmitter{}

	productSvc, err := productenrich.NewProductService(&productenrich.ProductServiceConfig{
		QueueName:            "test_queue",
		TaskRepo:             repo,
		RedisClient:          redis,
		InputValidator:       productenrich.NewInputValidator(nil),
		QualityScorer:        productenrich.NewQualityScorer(nil),
		StrategySelector:     productenrich.NewStrategySelector(nil),
		EnhancementSuggester: productenrich.NewEnhancementSuggester(),
		ResultValidator:      productenrich.NewResultValidator(),
	})
	if err != nil {
		t.Fatalf("NewProductService: %v", err)
	}
	productSvc.SetTaskSubmitter(submitter)

	productHandler, err := productapi.NewProductHandler(productSvc)
	if err != nil {
		t.Fatalf("NewProductHandler: %v", err)
	}

	imageSvc, err := productimage.NewService(&productimage.ServiceConfig{
		QueueName:        "image_test_queue",
		TaskRepo:         productimagestore.NewMemTaskRepository(),
		ImageInspector:   productimage.NewDefaultImageInspector(),
		ImageRanker:      productimage.NewDefaultImageRanker(),
		SubjectExtractor: productimage.NewDefaultSubjectExtractor(),
		ImageCleaner:     productimage.NewDefaultImageCleaner(),
		WhiteBgRenderer:  productimage.NewDefaultWhiteBackgroundRenderer(),
	})
	if err != nil {
		t.Fatalf("NewImageService: %v", err)
	}
	imageSvc.SetTaskSubmitter(submitter)

	imageHandler, err := productimagehttpapi.NewHandler(imageSvc)
	if err != nil {
		t.Fatalf("NewImageHandler: %v", err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	registerRoutes(r, productHandler, imageHandler)
	return r
}

func TestE2E_GenerateAndQueryTask(t *testing.T) {
	r := buildTestRouter(t)

	reqBody, _ := json.Marshal(productenrich.GenerateRequest{Text: "high quality bluetooth earphones"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/generate", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("generate = %d, body: %s", w.Code, w.Body.String())
	}

	var taskResp productenrich.TaskResponse
	if err := json.Unmarshal(w.Body.Bytes(), &taskResp); err != nil {
		t.Fatalf("unmarshal task response: %v", err)
	}
	if taskResp.TaskID == "" {
		t.Fatal("task_id should not be empty")
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/products/tasks/"+taskResp.TaskID, nil)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("get task = %d, body: %s", w2.Code, w2.Body.String())
	}
}

func TestE2E_ProcessAndQueryImageTask(t *testing.T) {
	r := buildTestRouter(t)

	reqBody, _ := json.Marshal(productimage.ImageProcessRequest{
		ImageURLs:   []string{"https://example.com/hero_white.jpg", "https://example.com/detail.jpg"},
		Marketplace: "amazon",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/process", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("process images = %d, body: %s", w.Code, w.Body.String())
	}

	var taskResp struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &taskResp); err != nil {
		t.Fatalf("unmarshal image task response: %v", err)
	}
	if taskResp.TaskID == "" {
		t.Fatal("image task_id should not be empty")
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/images/tasks/"+taskResp.TaskID, nil)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("get image task = %d, body: %s", w2.Code, w2.Body.String())
	}
}

func TestE2E_GenerateTask_EmptyRequest_Returns400(t *testing.T) {
	r := buildTestRouter(t)

	reqBody, _ := json.Marshal(productenrich.GenerateRequest{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/generate", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty request = %d, want 400", w.Code)
	}
}

func TestE2E_ProcessImages_EmptyRequest_Returns400(t *testing.T) {
	r := buildTestRouter(t)

	reqBody, _ := json.Marshal(productimage.ImageProcessRequest{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/process", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty image request = %d, want 400", w.Code)
	}
}

func TestE2E_GetTask_NotFound_Returns404(t *testing.T) {
	r := buildTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/tasks/nonexistent-id", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("not found = %d, want 404", w.Code)
	}
}

func TestE2E_GetImageTask_NotFound_Returns404(t *testing.T) {
	r := buildTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/images/tasks/nonexistent-id", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("image not found = %d, want 404", w.Code)
	}
}

func TestE2E_InvalidJSON_Returns400(t *testing.T) {
	r := buildTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/generate", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid product json = %d, want 400", w.Code)
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/images/process", bytes.NewReader([]byte("not-json")))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Fatalf("invalid image json = %d, want 400", w2.Code)
	}
}
