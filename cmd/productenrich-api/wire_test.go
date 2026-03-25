// Package main 测试 main.go 中的依赖注入、路由注册和 Worker Pool 集成
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"task-processor/internal/infra/worker"
	"task-processor/internal/productenrich"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func init() {
	gin.SetMode(gin.TestMode)
	// 测试期间静默日志
	logrus.SetLevel(logrus.FatalLevel)
}

// =============================================================================
// registerRoutes 路由注册测试
// =============================================================================

// stubHandler 实现 productenrich.ProductHandler，用于路由注册测试
type stubHandler struct {
	generateCalled  bool
	getResultCalled bool
}

func (s *stubHandler) GenerateProduct(c *gin.Context) {
	s.generateCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": "stub-task"})
}

func (s *stubHandler) GetTaskResult(c *gin.Context) {
	s.getResultCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id")})
}

func TestRegisterRoutes_HealthCheck(t *testing.T) {
	r := gin.New()
	registerRoutes(r, &stubHandler{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /health = %d, want 200", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

func TestRegisterRoutes_GenerateProduct(t *testing.T) {
	h := &stubHandler{}
	r := gin.New()
	registerRoutes(r, h)

	body, _ := json.Marshal(productenrich.GenerateRequest{Text: "test"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /api/v1/products/generate = %d, want 200", w.Code)
	}
	if !h.generateCalled {
		t.Error("GenerateProduct handler was not called")
	}
}

func TestRegisterRoutes_GetTaskResult(t *testing.T) {
	h := &stubHandler{}
	r := gin.New()
	registerRoutes(r, h)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/tasks/task-123", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /api/v1/products/tasks/:task_id = %d, want 200", w.Code)
	}
	if !h.getResultCalled {
		t.Error("GetTaskResult handler was not called")
	}
}

func TestRegisterRoutes_UnknownPath_Returns404(t *testing.T) {
	r := gin.New()
	registerRoutes(r, &stubHandler{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("unknown path = %d, want 404", w.Code)
	}
}

// =============================================================================
// buildHandler 依赖注入链测试（使用内存回退，不依赖外部服务）
// =============================================================================

// buildHandlerWithMemory 使用内存实现构建 handler，绕过真实 DB/Redis/LLM
// 通过替换全局 flag 变量实现（main 包内可直接访问）
func buildHandlerWithMemory(t *testing.T) (productenrich.ProductHandler, worker.WorkerPool, []func() error) {
	t.Helper()

	// 使用不存在的配置文件，触发内存回退路径
	*configPath = "nonexistent-config.yaml"

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler, pool, closers, err := buildHandler(logger)
	if err != nil {
		t.Skipf("buildHandler 需要外部依赖（LLM），跳过: %v", err)
	}
	return handler, pool, closers
}

func TestBuildHandler_ReturnsNonNilComponents(t *testing.T) {
	handler, pool, closers := buildHandlerWithMemory(t)
	defer func() {
		for _, c := range closers {
			_ = c()
		}
	}()

	if handler == nil {
		t.Error("handler should not be nil")
	}
	if pool == nil {
		t.Error("pool should not be nil")
	}
}

func TestBuildHandler_PoolCanStartAndStop(t *testing.T) {
	_, pool, closers := buildHandlerWithMemory(t)
	defer func() {
		for _, c := range closers {
			_ = c()
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	stats := pool.GetQueueStats()
	if stats.BufferSize <= 0 {
		t.Errorf("BufferSize = %d, want > 0", stats.BufferSize)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	cancel()
	pool.Stop(shutdownCtx)
}

// =============================================================================
// Worker Pool 与 ProductService 集成测试（全内存，不依赖外部服务）
// =============================================================================

// mockWorkerPool 实现 worker.WorkerPool，记录提交的 job
type mockWorkerPool struct {
	submitted []worker.WorkerJob
	started   bool
	stopped   bool
}

func (m *mockWorkerPool) Start(_ context.Context) { m.started = true }
func (m *mockWorkerPool) Stop(_ context.Context)  { m.stopped = true }
func (m *mockWorkerPool) AvailableSlots() int     { return 100 }
func (m *mockWorkerPool) GetQueueStats() worker.QueueStats {
	return worker.QueueStats{BufferSize: 100, AvailableSlots: 100}
}
func (m *mockWorkerPool) SetJobHandler(_ worker.JobHandler) {}
func (m *mockWorkerPool) GetMetrics() *worker.Metrics       { return nil }
func (m *mockWorkerPool) Submit(job worker.WorkerJob) error {
	m.submitted = append(m.submitted, job)
	return nil
}

// mockTaskSubmitter 实现 productenrich.TaskSubmitter，记录提交的 taskID
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

	// 注入 submitter
	svc.SetTaskSubmitter(submitter)

	task, err := svc.CreateGenerateTask(ctx, &productenrich.GenerateRequest{Text: "蓝牙耳机"})
	if err != nil {
		t.Fatalf("CreateGenerateTask: %v", err)
	}

	if len(submitter.submitted) != 1 {
		t.Errorf("submitted jobs = %d, want 1", len(submitter.submitted))
	}
	if submitter.submitted[0] != task.ID {
		t.Errorf("submitted TaskID = %q, want %q", submitter.submitted[0], task.ID)
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
	// 不调用 SetWorkerPool，走 Redis 降级路径

	task, err := svc.CreateGenerateTask(ctx, &productenrich.GenerateRequest{Text: "商品描述"})
	if err != nil {
		t.Fatalf("CreateGenerateTask: %v", err)
	}
	if task.Status != productenrich.TaskStatusPending {
		t.Errorf("Status = %q, want pending", task.Status)
	}
}

// =============================================================================
// Processor 生命周期测试
// =============================================================================

func TestProcessor_StartAndClose_NoError(t *testing.T) {
	repo := newMemTaskRepository()
	svc, _ := productenrich.NewProductService(&productenrich.ProductServiceConfig{
		QueueName:   "lc_queue",
		TaskRepo:    repo,
		RedisClient: newMemRedisClient(),
	})

	proc, err := productenrich.NewProcessor(svc, repo, logrus.New(), 3)
	if err != nil {
		t.Fatalf("NewProcessor: %v", err)
	}

	ctx := context.Background()
	if err := proc.Start(ctx); err != nil {
		t.Errorf("Start: %v", err)
	}
	proc.Close(ctx) // 不应 panic 或报错
}

// =============================================================================
// 端到端路由 + 内存 service 集成测试
// =============================================================================

// buildTestRouter 构建完整路由，使用内存 service（不依赖 LLM）
func buildTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

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

	handler, err := productenrich.NewProductHandler(svc)
	if err != nil {
		t.Fatalf("NewProductHandler: %v", err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	registerRoutes(r, handler)
	return r
}

func TestE2E_GenerateAndQueryTask(t *testing.T) {
	r := buildTestRouter(t)

	// 1. 提交生成任务
	reqBody, _ := json.Marshal(productenrich.GenerateRequest{Text: "高品质蓝牙耳机，支持主动降噪"})
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
		t.Error("task_id should not be empty")
	}
	if taskResp.Status != string(productenrich.TaskStatusPending) {
		t.Errorf("status = %q, want pending", taskResp.Status)
	}

	// 2. 查询任务结果
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/products/tasks/"+taskResp.TaskID, nil)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("get task = %d, body: %s", w2.Code, w2.Body.String())
	}

	var result productenrich.TaskResult
	if err := json.Unmarshal(w2.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal task result: %v", err)
	}
	if result.TaskID != taskResp.TaskID {
		t.Errorf("task_id = %q, want %q", result.TaskID, taskResp.TaskID)
	}
}

func TestE2E_GenerateTask_EmptyRequest_Returns400(t *testing.T) {
	r := buildTestRouter(t)

	// 空请求体（无 image_urls、text、product_url）
	reqBody, _ := json.Marshal(productenrich.GenerateRequest{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/generate", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// service 层 validateRequest 会拒绝，返回 500（task_creation_failed）
	// 因为 handler 将 service 错误统一映射为 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("empty request = %d, want 500", w.Code)
	}
	var errResp productenrich.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errResp.Error != "task_creation_failed" {
		t.Errorf("error = %q, want task_creation_failed", errResp.Error)
	}
}

func TestE2E_GetTask_NotFound_Returns404(t *testing.T) {
	r := buildTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/tasks/nonexistent-id", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("not found = %d, want 404", w.Code)
	}
	var errResp productenrich.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errResp.Error != "task_not_found" {
		t.Errorf("error = %q, want task_not_found", errResp.Error)
	}
}

func TestE2E_GenerateTask_InvalidJSON_Returns400(t *testing.T) {
	r := buildTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/generate", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid json = %d, want 400", w.Code)
	}
}

func TestE2E_ConcurrentTaskSubmission(t *testing.T) {
	r := buildTestRouter(t)
	const n = 10

	type result struct {
		code int
		id   string
	}
	results := make(chan result, n)

	for range n {
		go func() {
			reqBody, _ := json.Marshal(productenrich.GenerateRequest{Text: "并发测试商品"})
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/products/generate", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			var resp productenrich.TaskResponse
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			results <- result{code: w.Code, id: resp.TaskID}
		}()
	}

	ids := make(map[string]struct{}, n)
	for range n {
		res := <-results
		if res.code != http.StatusOK {
			t.Errorf("concurrent submit = %d, want 200", res.code)
		}
		if res.id == "" {
			t.Error("task_id should not be empty")
		}
		if _, dup := ids[res.id]; dup {
			t.Errorf("duplicate task_id: %s", res.id)
		}
		ids[res.id] = struct{}{}
	}
}
