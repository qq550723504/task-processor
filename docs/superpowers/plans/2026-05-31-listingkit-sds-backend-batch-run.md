# ListingKit SDS Backend Batch Run Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace frontend-driven Shein Studio bulk `批量继续生成` with a durable backend batch-run engine that can continue without the browser and expose progress, failure details, and cancellation.

**Architecture:** Add a new backend `studio batch run` persistence layer, service facade, executor, coordinator, and HTTP API in the existing `listingkit` service, reusing Studio session/batch storage and extracted single-batch generation logic. Keep single-batch workbench generation unchanged, but switch homepage bulk generation to start a backend run and render run progress from a dedicated progress view.

**Tech Stack:** Go, Gin, GORM, in-memory repositories for tests, Next.js App Router, React, Vitest, existing ListingKit proxy layer

---

## File Structure

### Backend domain and persistence

- Create: `internal/listingkit/studio_batch_run_repository.go`
  - record structs, status constants, mem/gorm repositories, auto-migrate
- Create: `internal/listingkit/studio_batch_run_repository_test.go`
  - mem/gorm repository behavior, scope enforcement, ordering
- Modify: `internal/listingkit/service.go`
  - add batch-run repository dependency and collaborator field
- Modify: `internal/listingkit/service_collaborators.go`
  - initialize new collaborator(s)
- Modify: `internal/listingkit/httpapi/builders.go`
  - database/fallback builders and auto-migrate wiring

### Backend execution and service facade

- Create: `internal/listingkit/studio_batch_run_service.go`
  - public service facade methods and request/response types
- Create: `internal/listingkit/task_studio_batch_run_service.go`
  - create/query/cancel behavior
- Create: `internal/listingkit/task_studio_batch_run_executor.go`
  - sequential item execution, aggregate counters, cancellation checks
- Create: `internal/listingkit/studio_batch_run_coordinator.go`
  - launch/resume unfinished runs on startup
- Create: `internal/listingkit/studio_batch_run_service_test.go`
  - create/query/cancel semantics
- Create: `internal/listingkit/task_studio_batch_run_executor_test.go`
  - continue-on-error, final statuses, cancellation

### Backend single-batch execution reuse

- Create: `internal/listingkit/studio_batch_generate_execution.go`
  - shared internal entry point for one Studio batch generation
- Modify: `internal/listingkit/api/studio_async_jobs_handler.go`
  - call shared internal execution path instead of owning the logic
- Modify: `internal/listingkit/api/handler.go`
  - wire batch-run APIs and dependencies
- Create: `internal/listingkit/api/studio_batch_runs_handler.go`
  - HTTP handlers for start/query/list-items/cancel
- Create: `internal/listingkit/api/studio_batch_runs_handler_test.go`
  - API binding and status responses
- Modify: `internal/listingkit/httpapi/routes.go`
  - register new batch-run routes

### Frontend API and types

- Create: `web/listingkit-ui/src/lib/types/shein-studio-batch-runs.ts`
  - run/item types for UI and API layer
- Create: `web/listingkit-ui/src/lib/api/shein-studio-batch-runs.ts`
  - start/query/list-items/cancel requests
- Create: `web/listingkit-ui/src/lib/api/shein-studio-batch-runs.test.ts`
  - request payload and response mapping
- Modify: `web/listingkit-ui/src/app/api/listing-kits/proxy-response.ts`
  - ensure new `studio/batch-runs` routes get Studio timeout bucket
- Modify: `web/listingkit-ui/src/app/api/listing-kits/route.test.ts`
  - lock timeout behavior for new routes

### Frontend UI

- Create: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-run-progress.tsx`
  - run status panel, progress counters, failed item list, cancel button
- Modify: `web/listingkit-ui/src/components/listingkit/sds/sds-homepage-entry.tsx`
  - launch backend run instead of opening frontend generate queue
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.tsx`
  - bulk action callback shape and text for backend run launch
- Modify: `web/listingkit-ui/src/app/listing-kits/sds/page.tsx`
  - support run-progress route state if needed
- Create: `web/listingkit-ui/src/components/listingkit/sds/sds-homepage-entry.test.tsx`
  - bulk generate launch + progress rendering

## Task 1: Add Batch Run Persistence

**Files:**
- Create: `internal/listingkit/studio_batch_run_repository.go`
- Create: `internal/listingkit/studio_batch_run_repository_test.go`
- Modify: `internal/listingkit/httpapi/builders.go`
- Test: `internal/listingkit/studio_batch_run_repository_test.go`

- [ ] **Step 1: Write the failing repository tests**

```go
func TestMemStudioBatchRunRepositoryCreateAndListItemsInOrder(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")

	run := &StudioBatchRunRecord{
		ID:            "run-1",
		Status:        StudioBatchRunStatusPending,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		TotalBatches:  2,
	}
	items := []StudioBatchRunItemRecord{
		{ID: "run-1:1", RunID: "run-1", BatchID: "batch-1", Position: 1, Status: StudioBatchRunItemStatusPending},
		{ID: "run-1:2", RunID: "run-1", BatchID: "batch-2", Position: 2, Status: StudioBatchRunItemStatusPending},
	}

	if err := repo.CreateStudioBatchRun(ctx, run, items); err != nil {
		t.Fatalf("CreateStudioBatchRun() error = %v", err)
	}
	gotItems, err := repo.ListStudioBatchRunItems(ctx, "run-1")
	if err != nil {
		t.Fatalf("ListStudioBatchRunItems() error = %v", err)
	}
	if len(gotItems) != 2 || gotItems[0].BatchID != "batch-1" || gotItems[1].BatchID != "batch-2" {
		t.Fatalf("got items = %+v, want ordered batch ids", gotItems)
	}
}

func TestGormStudioBatchRunRepositoryScopesByTenant(t *testing.T) {
	db := openSQLiteForTest(t)
	if err := AutoMigrateStudioBatchRunRepository(db); err != nil {
		t.Fatalf("AutoMigrateStudioBatchRunRepository() error = %v", err)
	}
	repo := NewGormStudioBatchRunRepository(db)
	ctxA := WithTenantID(context.Background(), "tenant-a")
	ctxB := WithTenantID(context.Background(), "tenant-b")

	run := &StudioBatchRunRecord{ID: "run-1", Status: StudioBatchRunStatusPending, TotalBatches: 1}
	item := StudioBatchRunItemRecord{ID: "run-1:1", RunID: "run-1", BatchID: "batch-1", Position: 1, Status: StudioBatchRunItemStatusPending}
	if err := repo.CreateStudioBatchRun(ctxA, run, []StudioBatchRunItemRecord{item}); err != nil {
		t.Fatalf("CreateStudioBatchRun() error = %v", err)
	}
	if _, err := repo.GetStudioBatchRun(ctxB, "run-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetStudioBatchRun() error = %v, want record not found", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit -run TestMemStudioBatchRunRepository`

Expected: FAIL with missing repository types and constructors such as `NewMemStudioBatchRunRepository` and `AutoMigrateStudioBatchRunRepository`

- [ ] **Step 3: Write minimal repository implementation**

```go
type StudioBatchRunStatus string

const (
	StudioBatchRunStatusPending            StudioBatchRunStatus = "pending"
	StudioBatchRunStatusRunning            StudioBatchRunStatus = "running"
	StudioBatchRunStatusSucceeded          StudioBatchRunStatus = "succeeded"
	StudioBatchRunStatusPartiallySucceeded StudioBatchRunStatus = "partially_succeeded"
	StudioBatchRunStatusFailed             StudioBatchRunStatus = "failed"
	StudioBatchRunStatusCancelled          StudioBatchRunStatus = "cancelled"
)

type StudioBatchRunRecord struct {
	ID               string               `gorm:"primaryKey;type:varchar(64)"`
	TenantID         string               `gorm:"type:varchar(64);index"`
	UserID           string               `gorm:"type:varchar(128);index"`
	Mode             string               `gorm:"type:varchar(32);not null"`
	FailurePolicy    string               `gorm:"type:varchar(32);not null"`
	Status           StudioBatchRunStatus `gorm:"type:varchar(32);index;not null"`
	CurrentBatchID   string               `gorm:"type:varchar(64);index"`
	CurrentIndex     int                  `gorm:"not null;default:0"`
	TotalBatches     int                  `gorm:"not null;default:0"`
	CompletedBatches int                  `gorm:"not null;default:0"`
	SucceededBatches int                  `gorm:"not null;default:0"`
	FailedBatches    int                  `gorm:"not null;default:0"`
	LastError        string               `gorm:"type:text"`
	CancelRequested  bool                 `gorm:"not null;default:false"`
	StartedAt        *time.Time
	FinishedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type StudioBatchRunItemRecord struct {
	ID           string `gorm:"primaryKey;type:varchar(96)"`
	TenantID     string `gorm:"type:varchar(64);index"`
	RunID        string `gorm:"type:varchar(64);index:idx_listingkit_studio_batch_run_items_run_position,priority:1"`
	BatchID      string `gorm:"type:varchar(64);index"`
	Position     int    `gorm:"index:idx_listingkit_studio_batch_run_items_run_position,priority:2"`
	Status       string `gorm:"type:varchar(32);index;not null"`
	SessionID    string `gorm:"type:varchar(64);index"`
	AsyncJobID   string `gorm:"type:varchar(64);index"`
	ErrorMessage string `gorm:"type:text"`
	StartedAt    *time.Time
	FinishedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type StudioBatchRunRepository interface {
	CreateStudioBatchRun(ctx context.Context, run *StudioBatchRunRecord, items []StudioBatchRunItemRecord) error
	GetStudioBatchRun(ctx context.Context, runID string) (*StudioBatchRunRecord, error)
	ListStudioBatchRunItems(ctx context.Context, runID string) ([]StudioBatchRunItemRecord, error)
	UpdateStudioBatchRun(ctx context.Context, run *StudioBatchRunRecord) error
	UpdateStudioBatchRunItem(ctx context.Context, item *StudioBatchRunItemRecord) error
}
```

- [ ] **Step 4: Run tests to verify repository behavior passes**

Run: `go test ./internal/listingkit -run "TestMemStudioBatchRunRepository|TestGormStudioBatchRunRepository"`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/studio_batch_run_repository.go internal/listingkit/studio_batch_run_repository_test.go internal/listingkit/httpapi/builders.go
git commit -m "feat: add studio batch run repository"
```

## Task 2: Add Service Facade and Batch Run Domain APIs

**Files:**
- Create: `internal/listingkit/studio_batch_run_service.go`
- Create: `internal/listingkit/task_studio_batch_run_service.go`
- Create: `internal/listingkit/studio_batch_run_service_test.go`
- Modify: `internal/listingkit/service.go`
- Modify: `internal/listingkit/service_collaborators.go`
- Test: `internal/listingkit/studio_batch_run_service_test.go`

- [ ] **Step 1: Write the failing service tests**

```go
func TestStudioBatchRunServiceCreateNormalizesOrderedItems(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	sessionRepo := newStudioSessionMemRepoForTest()
	svc := newTaskStudioBatchRunService(taskStudioBatchRunServiceConfig{
		repo:            repo,
		studioSessionRepo: sessionRepo,
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedSavedBatch(t, sessionRepo, ctx, "batch-1")
	seedSavedBatch(t, sessionRepo, ctx, "batch-2")

	run, items, err := svc.CreateStudioBatchRun(ctx, &CreateStudioBatchRunRequest{
		BatchIDs: []string{"batch-1", "batch-2"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchRun() error = %v", err)
	}
	if run.TotalBatches != 2 || len(items) != 2 || items[0].Position != 1 || items[1].Position != 2 {
		t.Fatalf("run=%+v items=%+v, want ordered items", run, items)
	}
}

func TestStudioBatchRunServiceCancelMarksRunAsCancelRequested(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	svc := newTaskStudioBatchRunService(taskStudioBatchRunServiceConfig{repo: repo})
	ctx := WithTenantID(context.Background(), "tenant-a")
	mustCreateRunForTest(t, repo, ctx, "run-1")

	if err := svc.CancelStudioBatchRun(ctx, "run-1"); err != nil {
		t.Fatalf("CancelStudioBatchRun() error = %v", err)
	}
	run, err := repo.GetStudioBatchRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("GetStudioBatchRun() error = %v", err)
	}
	if !run.CancelRequested {
		t.Fatalf("run.CancelRequested = false, want true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit -run TestStudioBatchRunService`

Expected: FAIL with missing request types, service methods, or constructor

- [ ] **Step 3: Add the service facade and request/response types**

```go
type CreateStudioBatchRunRequest struct {
	BatchIDs []string `json:"batch_ids"`
}

type StudioBatchRunService interface {
	CreateStudioBatchRun(ctx context.Context, req *CreateStudioBatchRunRequest) (*StudioBatchRunRecord, []StudioBatchRunItemRecord, error)
	GetStudioBatchRun(ctx context.Context, runID string) (*StudioBatchRunRecord, error)
	ListStudioBatchRunItems(ctx context.Context, runID string) ([]StudioBatchRunItemRecord, error)
	CancelStudioBatchRun(ctx context.Context, runID string) error
}

func (s *service) CreateStudioBatchRun(ctx context.Context, req *CreateStudioBatchRunRequest) (*StudioBatchRunRecord, []StudioBatchRunItemRecord, error) {
	return s.taskStudioBatchRunOrDefault().CreateStudioBatchRun(ctx, req)
}

func (s *service) CancelStudioBatchRun(ctx context.Context, runID string) error {
	return s.taskStudioBatchRunOrDefault().CancelStudioBatchRun(ctx, runID)
}
```

- [ ] **Step 4: Run tests to verify create/query/cancel behavior passes**

Run: `go test ./internal/listingkit -run TestStudioBatchRunService`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/studio_batch_run_service.go internal/listingkit/task_studio_batch_run_service.go internal/listingkit/studio_batch_run_service_test.go internal/listingkit/service.go internal/listingkit/service_collaborators.go
git commit -m "feat: add studio batch run service facade"
```

## Task 3: Extract Shared Single-Batch Studio Generation Execution

**Files:**
- Create: `internal/listingkit/studio_batch_generate_execution.go`
- Modify: `internal/listingkit/api/studio_async_jobs_handler.go`
- Modify: `internal/listingkit/api/studio_async_jobs_handler_test.go`
- Test: `internal/listingkit/api/studio_async_jobs_handler_test.go`

- [ ] **Step 1: Write the failing extraction test**

```go
func TestStartStudioAsyncJobUsesSharedStudioBatchExecution(t *testing.T) {
	t.Parallel()

	svc := &stubGenerationTaskService{
		studioDesigns: &listingkit.StudioDesignResponse{
			Prompt: "retro cherries",
			Images: []listingkit.StudioGeneratedImage{{ID: "design-1", ImageURL: "https://example.com/design.png"}},
		},
	}
	h, err := NewHandler(svc, WithSubscriptionService(activeStudioSubscriptionService(t)))
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/async-jobs", h.StartStudioAsyncJob)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/async-jobs", strings.NewReader(`{"path":"/studio/designs","body":{"prompt":"retro cherries","count":1}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.Code)
	}
	if svc.studioDesignReq == nil || svc.studioDesignReq.Prompt != "retro cherries" {
		t.Fatalf("studioDesignReq = %+v, want shared execution to call service", svc.studioDesignReq)
	}
}
```

- [ ] **Step 2: Run test to verify it fails for the new extraction seam**

Run: `go test ./internal/listingkit/api -run TestStartStudioAsyncJobUsesSharedStudioBatchExecution`

Expected: FAIL after introducing a reference to a not-yet-created helper such as `executeStudioDesignBatch`

- [ ] **Step 3: Extract the reusable execution entry point**

```go
type StudioBatchGenerateExecutor interface {
	ExecuteStudioDesignBatch(ctx context.Context, input StudioBatchGenerateInput) (*StudioBatchGenerateOutput, error)
}

type StudioBatchGenerateInput struct {
	Request   *StudioDesignRequest
	SessionID string
}

type StudioBatchGenerateOutput struct {
	Response  *StudioDesignResponse
	SessionID string
}

func (s *service) ExecuteStudioDesignBatch(ctx context.Context, input StudioBatchGenerateInput) (*StudioBatchGenerateOutput, error) {
	response, err := s.GenerateStudioDesigns(ctx, input.Request)
	if err != nil {
		return nil, err
	}
	return &StudioBatchGenerateOutput{
		Response:  response,
		SessionID: strings.TrimSpace(input.SessionID),
	}, nil
}
```

- [ ] **Step 4: Run async-job tests to verify the handler still works after extraction**

Run: `go test ./internal/listingkit/api -run TestStudioAsyncJob`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/studio_batch_generate_execution.go internal/listingkit/api/studio_async_jobs_handler.go internal/listingkit/api/studio_async_jobs_handler_test.go
git commit -m "refactor: extract studio batch generation execution"
```

## Task 4: Implement Batch Run Executor and Recovery Coordinator

**Files:**
- Create: `internal/listingkit/task_studio_batch_run_executor.go`
- Create: `internal/listingkit/studio_batch_run_coordinator.go`
- Create: `internal/listingkit/task_studio_batch_run_executor_test.go`
- Modify: `internal/listingkit/service_collaborators.go`
- Test: `internal/listingkit/task_studio_batch_run_executor_test.go`

- [ ] **Step 1: Write the failing executor tests**

```go
func TestStudioBatchRunExecutorContinuesAfterOneItemFailure(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	sessionRepo := newStudioSessionMemRepoForTest()
	ctx := WithTenantID(context.Background(), "tenant-a")
	seedSavedBatch(t, sessionRepo, ctx, "batch-1")
	seedSavedBatch(t, sessionRepo, ctx, "batch-2")
	mustCreateRunWithItems(t, repo, ctx, "run-1", []string{"batch-1", "batch-2"})

	executor := newTaskStudioBatchRunExecutor(taskStudioBatchRunExecutorConfig{
		repo: repo,
		executeOne: func(ctx context.Context, batchID string) error {
			if batchID == "batch-1" {
				return errors.New("upstream failed")
			}
			return nil
		},
	})

	if err := executor.Run(ctx, "run-1"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	run, _ := repo.GetStudioBatchRun(ctx, "run-1")
	if run.Status != StudioBatchRunStatusPartiallySucceeded || run.FailedBatches != 1 || run.SucceededBatches != 1 {
		t.Fatalf("run = %+v, want partially_succeeded with 1 success and 1 failure", run)
	}
}

func TestStudioBatchRunExecutorStopsStartingNewItemsAfterCancelRequested(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	mustCreateRunWithItems(t, repo, ctx, "run-1", []string{"batch-1", "batch-2"})
	mustCancelRunForTest(t, repo, ctx, "run-1")

	executor := newTaskStudioBatchRunExecutor(taskStudioBatchRunExecutorConfig{
		repo: repo,
		executeOne: func(ctx context.Context, batchID string) error {
			t.Fatalf("executeOne should not start when cancellation is already requested")
			return nil
		},
	})
	if err := executor.Run(ctx, "run-1"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	run, _ := repo.GetStudioBatchRun(ctx, "run-1")
	if run.Status != StudioBatchRunStatusCancelled {
		t.Fatalf("run.Status = %q, want cancelled", run.Status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit -run TestStudioBatchRunExecutor`

Expected: FAIL with missing executor/coordinator types

- [ ] **Step 3: Implement the minimal executor/coordinator logic**

```go
type taskStudioBatchRunExecutor struct {
	repo       StudioBatchRunRepository
	executeOne func(ctx context.Context, batchID string) error
}

func (e *taskStudioBatchRunExecutor) Run(ctx context.Context, runID string) error {
	run, err := e.repo.GetStudioBatchRun(ctx, runID)
	if err != nil {
		return err
	}
	items, err := e.repo.ListStudioBatchRunItems(ctx, runID)
	if err != nil {
		return err
	}
	for i := range items {
		if run.CancelRequested {
			run.Status = StudioBatchRunStatusCancelled
			return e.repo.UpdateStudioBatchRun(ctx, run)
		}
		item := &items[i]
		if item.Status == StudioBatchRunItemStatusSucceeded || item.Status == StudioBatchRunItemStatusFailed {
			continue
		}
		item.Status = StudioBatchRunItemStatusRunning
		run.Status = StudioBatchRunStatusRunning
		run.CurrentBatchID = item.BatchID
		run.CurrentIndex = item.Position
		_ = e.repo.UpdateStudioBatchRunItem(ctx, item)
		_ = e.repo.UpdateStudioBatchRun(ctx, run)
		err := e.executeOne(ctx, item.BatchID)
		run.CompletedBatches++
		if err != nil {
			item.Status = StudioBatchRunItemStatusFailed
			item.ErrorMessage = err.Error()
			run.FailedBatches++
			run.LastError = err.Error()
		} else {
			item.Status = StudioBatchRunItemStatusSucceeded
			run.SucceededBatches++
		}
		_ = e.repo.UpdateStudioBatchRunItem(ctx, item)
	}
	switch {
	case run.SucceededBatches > 0 && run.FailedBatches > 0:
		run.Status = StudioBatchRunStatusPartiallySucceeded
	case run.SucceededBatches > 0:
		run.Status = StudioBatchRunStatusSucceeded
	default:
		run.Status = StudioBatchRunStatusFailed
	}
	return e.repo.UpdateStudioBatchRun(ctx, run)
}
```

- [ ] **Step 4: Run executor tests and broad listingkit tests**

Run: `go test ./internal/listingkit -run TestStudioBatchRunExecutor`

Run: `go test ./internal/listingkit/...`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/task_studio_batch_run_executor.go internal/listingkit/studio_batch_run_coordinator.go internal/listingkit/task_studio_batch_run_executor_test.go internal/listingkit/service_collaborators.go
git commit -m "feat: add studio batch run executor"
```

## Task 5: Expose Batch Run HTTP APIs and Wire Repositories

**Files:**
- Create: `internal/listingkit/api/studio_batch_runs_handler.go`
- Create: `internal/listingkit/api/studio_batch_runs_handler_test.go`
- Modify: `internal/listingkit/api/handler.go`
- Modify: `internal/listingkit/httpapi/routes.go`
- Modify: `internal/listingkit/httpapi/builders.go`
- Test: `internal/listingkit/api/studio_batch_runs_handler_test.go`

- [ ] **Step 1: Write the failing handler tests**

```go
func TestCreateStudioBatchRunReturnsAcceptedRunPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &stubStudioBatchRunService{
		run: &listingkit.StudioBatchRunRecord{ID: "run-1", Status: listingkit.StudioBatchRunStatusPending, TotalBatches: 2},
		items: []listingkit.StudioBatchRunItemRecord{
			{ID: "run-1:1", RunID: "run-1", BatchID: "batch-1", Position: 1, Status: listingkit.StudioBatchRunItemStatusPending},
		},
	}
	h, err := NewHandler(&stubGenerationTaskService{}, WithStudioBatchRunService(svc))
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/batch-runs", h.CreateStudioBatchRun)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batch-runs", strings.NewReader(`{"batch_ids":["batch-1","batch-2"]}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 body=%s", resp.Code, resp.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit/api -run TestCreateStudioBatchRun`

Expected: FAIL with missing handler methods or missing handler option

- [ ] **Step 3: Implement route handlers and wiring**

```go
type studioBatchRunHandlerService interface {
	CreateStudioBatchRun(ctx context.Context, req *listingkit.CreateStudioBatchRunRequest) (*listingkit.StudioBatchRunRecord, []listingkit.StudioBatchRunItemRecord, error)
	GetStudioBatchRun(ctx context.Context, runID string) (*listingkit.StudioBatchRunRecord, error)
	ListStudioBatchRunItems(ctx context.Context, runID string) ([]listingkit.StudioBatchRunItemRecord, error)
	CancelStudioBatchRun(ctx context.Context, runID string) error
}

func (h *handler) CreateStudioBatchRun(c *gin.Context) {
	var req listingkit.CreateStudioBatchRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	run, items, err := h.studioBatchRunService.CreateStudioBatchRun(requestContext(c), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "studio_batch_run_create_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"run": run, "items": items})
}
```

- [ ] **Step 4: Run handler tests and route interface tests**

Run: `go test ./internal/listingkit/api -run TestCreateStudioBatchRun`

Run: `go test ./internal/listingkit/httpapi -run Test`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/api/studio_batch_runs_handler.go internal/listingkit/api/studio_batch_runs_handler_test.go internal/listingkit/api/handler.go internal/listingkit/httpapi/routes.go internal/listingkit/httpapi/builders.go
git commit -m "feat: expose studio batch run APIs"
```

## Task 6: Add Frontend Batch Run API Client and Proxy Coverage

**Files:**
- Create: `web/listingkit-ui/src/lib/types/shein-studio-batch-runs.ts`
- Create: `web/listingkit-ui/src/lib/api/shein-studio-batch-runs.ts`
- Create: `web/listingkit-ui/src/lib/api/shein-studio-batch-runs.test.ts`
- Modify: `web/listingkit-ui/src/app/api/listing-kits/proxy-response.ts`
- Modify: `web/listingkit-ui/src/app/api/listing-kits/route.test.ts`
- Test: `web/listingkit-ui/src/lib/api/shein-studio-batch-runs.test.ts`

- [ ] **Step 1: Write the failing frontend API tests**

```ts
it("starts a studio batch run with ordered batch ids", async () => {
  mockedApiRequest.mockResolvedValueOnce({
    run: { id: "run-1", status: "pending", total_batches: 2 },
    items: [{ id: "run-1:1", run_id: "run-1", batch_id: "batch-1", position: 1, status: "pending" }],
  });

  const response = await startSheinStudioBatchRun(["batch-1", "batch-2"]);

  expect(mockedApiRequest).toHaveBeenCalledWith(
    "/studio/batch-runs",
    expect.objectContaining({
      method: "POST",
      body: { batch_ids: ["batch-1", "batch-2"] },
    }),
  );
  expect(response.run.id).toBe("run-1");
});

it("uses the studio proxy timeout bucket for batch-runs", () => {
  expect(resolveListingKitProxyTimeoutMs("POST", ["studio", "batch-runs"])).toBe(PROXY_STUDIO_UPSTREAM_TIMEOUT_MS);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- src/lib/api/shein-studio-batch-runs.test.ts src/app/api/listing-kits/route.test.ts`

Expected: FAIL with missing batch-run client exports and timeout case

- [ ] **Step 3: Implement the client and type mapping**

```ts
export type SheinStudioBatchRunStatus =
  | "pending"
  | "running"
  | "succeeded"
  | "partially_succeeded"
  | "failed"
  | "cancelled";

export async function startSheinStudioBatchRun(batchIds: string[]) {
  const payload = await apiRequest<{
    run: Record<string, unknown>;
    items: Array<Record<string, unknown>>;
  }>("/studio/batch-runs", {
    method: "POST",
    body: { batch_ids: batchIds },
  });
  return {
    run: mapStudioBatchRun(payload.run),
    items: (payload.items ?? []).map(mapStudioBatchRunItem),
  };
}

export async function cancelSheinStudioBatchRun(runId: string) {
  await apiRequest(`/studio/batch-runs/${runId}/cancel`, { method: "POST" });
}
```

- [ ] **Step 4: Run frontend API and proxy tests**

Run: `npm test -- src/lib/api/shein-studio-batch-runs.test.ts src/app/api/listing-kits/route.test.ts`

Run: `npm run typecheck`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/lib/types/shein-studio-batch-runs.ts web/listingkit-ui/src/lib/api/shein-studio-batch-runs.ts web/listingkit-ui/src/lib/api/shein-studio-batch-runs.test.ts web/listingkit-ui/src/app/api/listing-kits/proxy-response.ts web/listingkit-ui/src/app/api/listing-kits/route.test.ts
git commit -m "feat: add shein studio batch run client"
```

## Task 7: Switch Homepage Bulk Generate to Backend Runs and Render Progress

**Files:**
- Create: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-run-progress.tsx`
- Create: `web/listingkit-ui/src/components/listingkit/sds/sds-homepage-entry.test.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/sds/sds-homepage-entry.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/sds/sds-homepage-entry.test.tsx`

- [ ] **Step 1: Write the failing homepage test**

```tsx
it("starts a backend batch run when the user launches bulk continue generate", async () => {
  vi.mocked(startSheinStudioBatchRun).mockResolvedValueOnce({
    run: {
      id: "run-1",
      status: "pending",
      totalBatches: 2,
      completedBatches: 0,
      succeededBatches: 0,
      failedBatches: 0,
    },
    items: [],
  });

  render(<SdsHomepageEntry />);

  await userEvent.click(await screen.findByRole("button", { name: /查看全部批次/i }));
  await userEvent.click(await screen.findByLabelText(/select batch:batch-1/i));
  await userEvent.click(await screen.findByRole("button", { name: /批量继续生成/i }));

  expect(startSheinStudioBatchRun).toHaveBeenCalledWith(["batch-1"]);
  expect(await screen.findByText(/运行中批量生成/i)).toBeInTheDocument();
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- src/components/listingkit/sds/sds-homepage-entry.test.tsx`

Expected: FAIL with missing progress component or old queue callback still being used

- [ ] **Step 3: Replace frontend queue launch with backend run progress**

```tsx
const [activeBatchRunId, setActiveBatchRunId] = useState("");

async function handleOpenBatchQueue(input: { batchIds: string[]; mode: "generate" | "create_tasks" }) {
  if (input.mode !== "generate") {
    return;
  }
  const response = await startSheinStudioBatchRun(input.batchIds);
  setActiveBatchRunId(response.run.id);
}

{activeBatchRunId ? (
  <SheinStudioBatchRunProgress
    runId={activeBatchRunId}
    onBack={() => setActiveBatchRunId("")}
  />
) : (
  <SheinStudioRecentBatchesDashboard
    onOpenBatchQueue={handleOpenBatchQueue}
    ...
  />
)}
```

- [ ] **Step 4: Run homepage tests and related frontend coverage**

Run: `npm test -- src/components/listingkit/sds/sds-homepage-entry.test.tsx src/lib/api/shein-studio-batch-runs.test.ts`

Run: `npm run typecheck`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-run-progress.tsx web/listingkit-ui/src/components/listingkit/sds/sds-homepage-entry.tsx web/listingkit-ui/src/components/listingkit/sds/sds-homepage-entry.test.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.tsx
git commit -m "feat: switch bulk generate to backend batch runs"
```

## Self-Review

### Spec Coverage

- Persistence model for run + items is covered by Task 1.
- Service facade, create/query/cancel, and request types are covered by Task 2.
- Shared single-batch execution reuse is covered by Task 3.
- Sequential execution, continue-on-error, and cancellation are covered by Task 4.
- HTTP APIs and wiring are covered by Task 5.
- Frontend API, proxy timeout coverage, and type mapping are covered by Task 6.
- Homepage launch and progress UI are covered by Task 7.

No spec section is currently left without a corresponding task.

### Placeholder Scan

- No `TODO`, `TBD`, or “implement later” placeholders were left in tasks.
- Every code-changing step includes a concrete snippet.
- Every verification step includes explicit commands and expected outcomes.

### Type Consistency

- Backend naming uses `StudioBatchRunRecord`, `StudioBatchRunItemRecord`, `CreateStudioBatchRunRequest`, and `CancelStudioBatchRun`.
- Frontend naming uses `startSheinStudioBatchRun`, `cancelSheinStudioBatchRun`, and `SheinStudioBatchRunStatus`.
- The run status values and item status values match the approved spec.

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-31-listingkit-sds-backend-batch-run.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
