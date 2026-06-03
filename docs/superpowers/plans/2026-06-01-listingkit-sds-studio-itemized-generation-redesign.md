# ListingKit SDS Studio Itemized Generation Redesign Implementation Plan

## Status

Status as of 2026-06-04: Partially implemented, not fully cut over.

The batch/item/attempt/materialized-design direction described here has been substantially implemented:

- New batch model and repositories:
  - `internal/listingkit/studio_batch_model.go`
  - `internal/listingkit/studio_batch_repository.go`
- Batch services and generation flow:
  - `internal/listingkit/studio_batch_service.go`
  - `internal/listingkit/task_studio_batch_service.go`
  - `internal/listingkit/studio_batch_generation.go`
- New batch HTTP routes and frontend batch client:
  - `internal/listingkit/httpapi/routes.go`
  - `web/listingkit-ui/src/lib/api/shein-studio-batches.ts`

However, this redesign is not fully complete. Legacy session-centered compatibility paths still remain, including examples such as:

- `internal/listingkit/task_studio_session_service.go`
- `internal/listingkit/studio_session_api_model.go`
- frontend state/types that still carry flat `selectedIds` / `generationJobs` style ownership

This document should therefore be treated as a partially completed redesign plan rather than an active step-by-step implementation checklist.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the session-centered Shein Studio generation flow with a backend-owned `batch -> item -> attempt -> materialized design` model so generation, recovery, review, and task creation all use the same durable ownership model.

**Architecture:** Introduce first-class backend records for batch items, generation attempts, and materialized designs; move generation/materialization/recovery completely behind batch APIs; then refactor the frontend workbench and batch detail screens to render item-owned designs and submit approvals/task creation through item ownership instead of flat design/job arrays.

**Tech Stack:** Go, Gin, GORM, in-memory repositories for tests, Next.js App Router, React, TypeScript, Vitest, existing ListingKit proxy layer

---

## File Structure

### Backend domain and persistence

- Create: `internal/listingkit/studio_batch_model.go`
  - batch/item/attempt/materialized-design record types, status constants, API detail structs
- Create: `internal/listingkit/studio_batch_repository.go`
  - repository interfaces plus mem/gorm implementations for the new model
- Create: `internal/listingkit/studio_batch_repository_test.go`
  - repository create/load/update/order/scope tests
- Modify: `internal/listingkit/service.go`
  - add new batch repository/service collaborators alongside existing batch-run collaborators
- Modify: `internal/listingkit/service_collaborators.go`
  - initialize the new batch lifecycle / generation / materialization collaborators
- Modify: `internal/listingkit/httpapi/builders.go`
  - auto-migrate and dependency wiring for the new repository

### Backend batch lifecycle, generation, and materialization

- Create: `internal/listingkit/studio_batch_service.go`
  - public service facade for batch query/start/retry/approval/task creation
- Create: `internal/listingkit/task_studio_batch_service.go`
  - batch create/update/detail projection logic and approval mutation logic
- Create: `internal/listingkit/studio_batch_generation.go`
  - item expansion, attempt creation, recovery scan, materialization orchestration
- Create: `internal/listingkit/studio_batch_generation_test.go`
  - expansion, attempt state, recovery, retry, no-cross-item-image tests
- Create: `internal/listingkit/studio_batch_service_test.go`
  - service façade tests for batch detail, start generation, retry, approvals
- Modify: `internal/listingkit/task_studio_batch_run_executor.go`
  - call new batch generation entrypoint instead of `ReplaceStudioSessionDesigns`
- Modify: `internal/listingkit/studio_batch_generate_execution.go`
  - return attempt-oriented execution output suitable for materialization
  - emit item/attempt/materialization logs at each state boundary

### Backend HTTP API cutover

- Modify: `internal/listingkit/api/handler.go`
  - extend handler interfaces with new batch actions and remove session-design append ownership
- Modify: `internal/listingkit/api/studio_sessions_handler.go`
  - keep only batch CRUD that still applies, then route generation/task actions through the new batch service
- Modify: `internal/listingkit/api/studio_async_jobs_handler.go`
  - stop treating async-job success as persisted review state; forward through batch generation/materialization entrypoints
- Modify: `internal/listingkit/httpapi/routes.go`
  - add `/studio/batches/:batch_id/generate`, `/items/retry`, `/design-approvals`, `/tasks`; remove `/studio/sessions/:session_id/designs*`
- Modify: `internal/listingkit/studio_session_api_model.go`
  - remove append/replace session-design request types and add new batch action request types
- Create: `internal/listingkit/api/studio_batches_handler_test.go`
  - HTTP binding, validation, and response tests for new routes

### Frontend types and API layer

- Modify: `web/listingkit-ui/src/lib/types/shein-studio.ts`
  - replace flat `designs/selectedIds/generationJobs` ownership with itemized batch types
- Create: `web/listingkit-ui/src/lib/api/shein-studio-batches.ts`
  - batch detail, start generation, retry items, approve designs, create tasks
- Create: `web/listingkit-ui/src/lib/api/shein-studio-batches.test.ts`
  - response mapping and request payload tests
- Delete: `web/listingkit-ui/src/lib/api/shein-studio-sessions.ts`
  - remove old session-centered persistence client after callers move
- Modify: `web/listingkit-ui/src/lib/utils/shein-studio-batches.ts`
  - batch/local-draft helpers should call only batch APIs for durable state
- Modify: `web/listingkit-ui/src/lib/shein-studio/create-review-tasks.ts`
  - stop deriving ownership from flat designs; consume item/design ownership from backend payloads

### Frontend workbench and review/task UI

- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.ts`
  - replace flat workbench state with `items`, `approvedDesignIds`, item progress, selected retry targets
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-actions.ts`
  - generation action should call batch generation APIs instead of appending session designs
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-workspace.ts`
  - loader should hydrate batch/items/designs from backend detail only
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.ts`
  - item-aware projection helpers
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-preview-grid.tsx`
  - render item-owned design cards and approval state
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-task-creation-actions.ts`
  - submit approved materialized design ids through backend task endpoint
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-detail.tsx`
  - render itemized batch detail and task creation progress
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`
  - wire new state/actions/selectors
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`
  - cover itemized generation/review/task behavior
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-detail.test.tsx`
  - cover itemized batch detail rendering and task action

## Task 1: Add Itemized Batch Persistence

**Files:**
- Create: `internal/listingkit/studio_batch_model.go`
- Create: `internal/listingkit/studio_batch_repository.go`
- Create: `internal/listingkit/studio_batch_repository_test.go`
- Modify: `internal/listingkit/service.go`
- Modify: `internal/listingkit/httpapi/builders.go`
- Test: `internal/listingkit/studio_batch_repository_test.go`

- [ ] **Step 1: Write the failing repository tests**

```go
func TestMemStudioBatchRepositoryCreatesDetailGraph(t *testing.T) {
	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")

	batch := &StudioBatchRecord{
		ID:               "batch-1",
		Status:           StudioBatchStatusDraft,
		GroupedImageMode: "shared_by_size",
		Prompt:           "botanical summer",
	}
	items := []StudioBatchItemRecord{{
		ID:               "item-1",
		BatchID:          "batch-1",
		TargetGroupKey:   "size:1200x1200",
		TargetGroupLabel: "1200 x 1200",
		Status:           StudioBatchItemStatusPending,
		SelectionCount:   3,
	}}
	attempts := []StudioGenerationAttemptRecord{{
		ID:       "attempt-1",
		ItemID:   "item-1",
		AttemptNo: 1,
		Status:   StudioGenerationAttemptStatusQueued,
	}}
	designs := []StudioMaterializedDesignRecord{{
		ID:             "design-1",
		BatchID:        "batch-1",
		ItemID:         "item-1",
		SourceAttemptID: "attempt-1",
		TargetGroupKey: "size:1200x1200",
		ImageURL:       "https://cdn.example.com/design-1.png",
	}}

	if err := repo.CreateStudioBatchGraph(ctx, batch, items, attempts, designs); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}
	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if len(detail.Items) != 1 || len(detail.AttemptsByItem["item-1"]) != 1 || len(detail.DesignsByItem["item-1"]) != 1 {
		t.Fatalf("detail = %+v, want full item graph", detail)
	}
}

func TestGormStudioBatchRepositoryScopesByTenant(t *testing.T) {
	db := openSQLiteForTest(t)
	if err := AutoMigrateStudioBatchRepository(db); err != nil {
		t.Fatalf("AutoMigrateStudioBatchRepository() error = %v", err)
	}
	repo := NewGormStudioBatchRepository(db)
	ctxA := WithTenantID(context.Background(), "tenant-a")
	ctxB := WithTenantID(context.Background(), "tenant-b")

	if err := repo.CreateStudioBatchGraph(ctxA, &StudioBatchRecord{ID: "batch-1", Status: StudioBatchStatusDraft}, nil, nil, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}
	if _, err := repo.GetStudioBatchDetail(ctxB, "batch-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetStudioBatchDetail() error = %v, want record not found", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit -run "TestMemStudioBatchRepository|TestGormStudioBatchRepository"`

Expected: FAIL with missing itemized batch repository types such as `StudioBatchRecord`, `StudioBatchItemRecord`, and `NewMemStudioBatchRepository`

- [ ] **Step 3: Write the minimal itemized record and repository implementation**

```go
type StudioBatchStatus string

const (
	StudioBatchStatusDraft               StudioBatchStatus = "draft"
	StudioBatchStatusGenerating          StudioBatchStatus = "generating"
	StudioBatchStatusPartiallyMaterialized StudioBatchStatus = "partially_materialized"
	StudioBatchStatusReviewReady         StudioBatchStatus = "review_ready"
	StudioBatchStatusPartiallyFailed     StudioBatchStatus = "partially_failed"
	StudioBatchStatusFailed              StudioBatchStatus = "failed"
	StudioBatchStatusTasksCreated        StudioBatchStatus = "tasks_created"
)

type StudioBatchItemStatus string

const (
	StudioBatchItemStatusPending               StudioBatchItemStatus = "pending"
	StudioBatchItemStatusGenerating            StudioBatchItemStatus = "generating"
	StudioBatchItemStatusAwaitingMaterialization StudioBatchItemStatus = "awaiting_materialization"
	StudioBatchItemStatusReviewReady           StudioBatchItemStatus = "review_ready"
	StudioBatchItemStatusFailed                StudioBatchItemStatus = "failed"
)

type StudioBatchRecord struct {
	ID               string `gorm:"primaryKey;type:varchar(64)"`
	TenantID         string `gorm:"type:varchar(64);index"`
	UserID           string `gorm:"type:varchar(128);index"`
	Status           StudioBatchStatus `gorm:"type:varchar(32);index;not null"`
	Prompt           string `gorm:"type:text"`
	GroupedImageMode string `gorm:"type:varchar(32)"`
	SheinStoreID     string `gorm:"type:varchar(64)"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type StudioBatchItemRecord struct {
	ID               string `gorm:"primaryKey;type:varchar(96)"`
	BatchID          string `gorm:"type:varchar(64);index"`
	TargetGroupKey   string `gorm:"type:varchar(255);index"`
	TargetGroupLabel string `gorm:"type:varchar(255)"`
	GroupMode        string `gorm:"type:varchar(32)"`
	Status           StudioBatchItemStatus `gorm:"type:varchar(32);index;not null"`
	SelectionCount   int `gorm:"not null;default:0"`
	LastError        string `gorm:"type:text"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type StudioGenerationAttemptRecord struct {
	ID            string `gorm:"primaryKey;type:varchar(96)"`
	ItemID        string `gorm:"type:varchar(96);index"`
	AttemptNo     int    `gorm:"not null"`
	Status        string `gorm:"type:varchar(32);index;not null"`
	UpstreamJobID string `gorm:"type:varchar(64);index"`
	RequestPayload string `gorm:"type:text"`
	ErrorMessage  string `gorm:"type:text"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type StudioMaterializedDesignRecord struct {
	ID              string `gorm:"primaryKey;type:varchar(96)"`
	BatchID         string `gorm:"type:varchar(64);index"`
	ItemID          string `gorm:"type:varchar(96);index"`
	SourceAttemptID string `gorm:"type:varchar(96);index"`
	TargetGroupKey  string `gorm:"type:varchar(255);index"`
	TargetGroupLabel string `gorm:"type:varchar(255)"`
	ImageURL        string `gorm:"type:text"`
	Approved        bool   `gorm:"not null;default:false"`
	SortOrder       int    `gorm:"not null;default:0"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
```

- [ ] **Step 4: Run tests to verify repository behavior passes**

Run: `go test ./internal/listingkit -run "TestMemStudioBatchRepository|TestGormStudioBatchRepository"`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/studio_batch_model.go internal/listingkit/studio_batch_repository.go internal/listingkit/studio_batch_repository_test.go internal/listingkit/service.go internal/listingkit/httpapi/builders.go
git commit -m "feat: add itemized studio batch repository"
```

## Task 2: Add Batch Lifecycle and Detail Projection Service

**Files:**
- Create: `internal/listingkit/studio_batch_service.go`
- Create: `internal/listingkit/task_studio_batch_service.go`
- Create: `internal/listingkit/studio_batch_service_test.go`
- Modify: `internal/listingkit/service.go`
- Modify: `internal/listingkit/service_collaborators.go`
- Modify: `internal/listingkit/studio_session_service.go`
- Modify: `internal/listingkit/studio_batch_repository.go`
- Modify: `internal/listingkit/studio_batch_repository_test.go`
- Test: `internal/listingkit/studio_batch_service_test.go`

- [ ] **Step 1: Write the failing lifecycle tests**

```go
func TestStudioBatchServiceReturnsItemizedDetailProjection(t *testing.T) {
	repo := NewMemStudioBatchRepository()
	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{repo: repo})
	ctx := WithTenantID(context.Background(), "tenant-a")

	if err := repo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{
		ID:     "batch-1",
		Status: StudioBatchStatusReviewReady,
		Prompt: "botanical summer",
	}, []StudioBatchItemRecord{{
		ID:             "item-1",
		BatchID:        "batch-1",
		TargetGroupKey: "size:1200x1200",
		Status:         StudioBatchItemStatusReviewReady,
	}}, nil, []StudioMaterializedDesignRecord{{
		ID:              "design-1",
		BatchID:         "batch-1",
		ItemID:          "item-1",
		SourceAttemptID: "attempt-1",
		TargetGroupKey:  "size:1200x1200",
		ImageURL:        "https://cdn.example.com/design-1.png",
	}}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	detail, err := svc.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if detail.Batch == nil || len(detail.Items) != 1 || len(detail.Items[0].Designs) != 1 {
		t.Fatalf("detail = %+v, want projected itemized detail", detail)
	}
}

func TestStudioBatchServiceApprovesDesignsByID(t *testing.T) {
	repo := NewMemStudioBatchRepository()
	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{repo: repo})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchReviewReady(t, repo, ctx, "batch-1", "item-1", "design-1")

	detail, err := svc.ApproveStudioBatchDesigns(ctx, "batch-1", &ApproveStudioBatchDesignsRequest{
		DesignIDs: []string{"design-1"},
	})
	if err != nil {
		t.Fatalf("ApproveStudioBatchDesigns() error = %v", err)
	}
	if !detail.Items[0].Designs[0].Approved {
		t.Fatalf("detail = %+v, want approved design", detail)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit -run "TestStudioBatchServiceReturnsItemizedDetailProjection|TestStudioBatchServiceApprovesDesignsByID"`

Expected: FAIL with missing service façade methods such as `GetStudioBatchDetail` and `ApproveStudioBatchDesigns`

- [ ] **Step 3: Write the lifecycle service and projection types**

```go
type StudioBatchDetail struct {
	Batch *StudioBatchRecord         `json:"batch,omitempty"`
	Items []StudioBatchItemDetail    `json:"items,omitempty"`
}

type StudioBatchItemDetail struct {
	Item     StudioBatchItemRecord          `json:"item"`
	Attempts []StudioGenerationAttemptRecord `json:"attempts,omitempty"`
	Designs  []StudioMaterializedDesignRecord `json:"designs,omitempty"`
}

type ApproveStudioBatchDesignsRequest struct {
	DesignIDs []string `json:"design_ids,omitempty"`
}

func (s *service) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().GetStudioBatchDetail(ctx, batchID)
}

func (s *taskStudioBatchService) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	graph, err := s.repo.GetStudioBatchDetail(ctx, strings.TrimSpace(batchID))
	if err != nil {
		return nil, err
	}
	return projectStudioBatchDetail(graph), nil
}

func (s *service) taskStudioBatchOrDefault() *taskStudioBatchService {
	if s.taskStudioBatch != nil {
		return s.taskStudioBatch
	}
	s.taskStudioBatch = newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: s.studioBatchRepo,
	})
	return s.taskStudioBatch
}
```

- [ ] **Step 4: Run tests to verify the detail projection passes**

Run: `go test ./internal/listingkit -run "TestStudioBatchServiceReturnsItemizedDetailProjection|TestStudioBatchServiceApprovesDesignsByID"`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/studio_batch_service.go internal/listingkit/task_studio_batch_service.go internal/listingkit/studio_batch_service_test.go internal/listingkit/service.go internal/listingkit/service_collaborators.go internal/listingkit/studio_session_service.go internal/listingkit/studio_batch_repository.go internal/listingkit/studio_batch_repository_test.go
git commit -m "feat: add itemized studio batch detail service"
```

## Task 3: Implement Item Expansion, Generation Attempts, and Materialization

**Files:**
- Create: `internal/listingkit/studio_batch_generation.go`
- Create: `internal/listingkit/studio_batch_generation_test.go`
- Modify: `internal/listingkit/studio_batch_generate_execution.go`
- Modify: `internal/listingkit/task_studio_batch_run_executor.go`
- Test: `internal/listingkit/studio_batch_generation_test.go`

- [ ] **Step 1: Write the failing generation tests**

```go
func TestStartStudioBatchGenerationExpandsPerProductIntoSeparateItems(t *testing.T) {
	repo := NewMemStudioBatchRepository()
	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{repo: repo})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchDraft(t, repo, ctx, &StudioBatchRecord{
		ID:               "batch-1",
		Status:           StudioBatchStatusDraft,
		GroupedImageMode: "per_product",
	}, []SheinStudioSelection{
		{VariantID: 101, ProductID: 101},
		{VariantID: 102, ProductID: 102},
	})

	detail, err := svc.StartStudioBatchGeneration(ctx, "batch-1")
	if err != nil {
		t.Fatalf("StartStudioBatchGeneration() error = %v", err)
	}
	if len(detail.Items) != 2 {
		t.Fatalf("detail items = %d, want 2", len(detail.Items))
	}
}

func TestMaterializeAttemptDoesNotBorrowImagesAcrossItems(t *testing.T) {
	repo := NewMemStudioBatchRepository()
	engine := newStudioBatchGenerationServiceForTest(repo, stubStudioGenerator{
		ImagesByItem: map[string][]StudioGeneratedImage{
			"item-1": {{ID: "design-1", ImageURL: "https://cdn.example.com/design-1.png"}},
			"item-2": {},
		},
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	batchID := seedStudioBatchWithTwoItems(t, repo, ctx)
	if err := engine.RunPendingStudioBatchItems(ctx, batchID); err != nil {
		t.Fatalf("RunPendingStudioBatchItems() error = %v", err)
	}
	detail, err := repo.GetStudioBatchDetail(ctx, batchID)
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if len(detail.DesignsByItem["item-2"]) != 0 {
		t.Fatalf("designs for item-2 = %+v, want none", detail.DesignsByItem["item-2"])
	}
}

func TestRecoverAwaitingMaterializationReusesAttemptResult(t *testing.T) {
	repo := NewMemStudioBatchRepository()
	engine := newStudioBatchGenerationServiceForTest(repo, stubStudioGenerator{})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedAwaitingMaterializationItem(t, repo, ctx, "batch-1", "item-1", "attempt-1")
	if err := engine.RecoverStudioBatchMaterialization(ctx, "batch-1"); err != nil {
		t.Fatalf("RecoverStudioBatchMaterialization() error = %v", err)
	}
	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if detail.Items[0].Status != StudioBatchItemStatusReviewReady {
		t.Fatalf("item status = %q, want review_ready", detail.Items[0].Status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit -run "TestStartStudioBatchGenerationExpandsPerProductIntoSeparateItems|TestMaterializeAttemptDoesNotBorrowImagesAcrossItems|TestRecoverAwaitingMaterializationReusesAttemptResult"`

Expected: FAIL with missing generation/materialization entrypoints such as `StartStudioBatchGeneration` and `RecoverStudioBatchMaterialization`

- [ ] **Step 3: Implement expansion, attempt orchestration, and materialization**

```go
func (s *taskStudioBatchService) StartStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	batch, err := s.repo.GetStudioBatch(ctx, strings.TrimSpace(batchID))
	if err != nil {
		return nil, err
	}
	items := expandStudioBatchSelections(batch)
	if err := s.repo.ReplaceStudioBatchItems(ctx, batch.ID, items); err != nil {
		return nil, err
	}
	if err := s.generator.RunPendingStudioBatchItems(ctx, batch.ID); err != nil {
		return nil, err
	}
	return s.GetStudioBatchDetail(ctx, batch.ID)
}

func (g *studioBatchGenerationService) materializeAttempt(ctx context.Context, item StudioBatchItemRecord, attempt *StudioGenerationAttemptRecord, response *StudioDesignResponse) error {
	designs := make([]StudioMaterializedDesignRecord, 0, len(response.Images))
	for index, image := range response.Images {
		designs = append(designs, StudioMaterializedDesignRecord{
			ID:               strings.TrimSpace(image.ID),
			BatchID:          item.BatchID,
			ItemID:           item.ID,
			SourceAttemptID:  attempt.ID,
			TargetGroupKey:   item.TargetGroupKey,
			TargetGroupLabel: item.TargetGroupLabel,
			ImageURL:         strings.TrimSpace(image.ImageURL),
			SortOrder:        index,
		})
	}
	if err := g.repo.ReplaceStudioItemMaterializedDesigns(ctx, item.ID, designs); err != nil {
		return err
	}
	studioBatchLogger.WithFields(logrus.Fields{
		"batch_id": item.BatchID,
		"item_id": item.ID,
		"attempt_id": attempt.ID,
		"design_count": len(designs),
	}).Info("studio batch item materialized")
	item.Status = StudioBatchItemStatusReviewReady
	return g.repo.UpdateStudioBatchItem(ctx, &item)
}
```

- [ ] **Step 4: Run tests to verify generation/materialization behavior passes**

Run: `go test ./internal/listingkit -run "TestStartStudioBatchGenerationExpandsPerProductIntoSeparateItems|TestMaterializeAttemptDoesNotBorrowImagesAcrossItems|TestRecoverAwaitingMaterializationReusesAttemptResult"`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/studio_batch_generation.go internal/listingkit/studio_batch_generation_test.go internal/listingkit/studio_batch_generate_execution.go internal/listingkit/task_studio_batch_run_executor.go
git commit -m "feat: add itemized studio batch generation flow"
```

## Task 4: Cut Over Backend APIs for Generate, Retry, Approvals, and Tasks

**Files:**
- Modify: `internal/listingkit/task_studio_batch_service.go`
- Modify: `internal/listingkit/studio_batch_service_test.go`
- Modify: `internal/listingkit/studio_session_api_model.go`
- Modify: `internal/listingkit/api/handler.go`
- Modify: `internal/listingkit/api/studio_sessions_handler.go`
- Modify: `internal/listingkit/httpapi/routes.go`
- Create: `internal/listingkit/api/studio_batches_handler_test.go`
- Modify: `internal/listingkit/studio_session_handler.go`
- Test: `internal/listingkit/api/studio_batches_handler_test.go`

- [ ] **Step 1: Write the failing HTTP tests**

```go
func TestStudioBatchGenerateHandlerStartsItemizedGeneration(t *testing.T) {
	svc := &stubStudioBatchService{
		startResult: &listingkit.StudioBatchDetail{
			Batch: &listingkit.StudioBatchRecord{ID: "batch-1", Status: listingkit.StudioBatchStatusGenerating},
			Items: []listingkit.StudioBatchItemDetail{{Item: listingkit.StudioBatchItemRecord{ID: "item-1"}}},
		},
	}
	h := newStudioSessionHandler(svc)
	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/batches/:batch_id/generate", h.StartStudioBatchGeneration)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batches/batch-1/generate", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "\"items\"") {
		t.Fatalf("body = %s, want itemized detail", rec.Body.String())
	}
}

func TestStudioBatchApproveDesignsHandlerBindsIDs(t *testing.T) {
	svc := &stubStudioBatchService{
		approveResult: &listingkit.StudioBatchDetail{
			Batch: &listingkit.StudioBatchRecord{ID: "batch-1"},
		},
	}
	h := newStudioSessionHandler(svc)
	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/batches/:batch_id/design-approvals", h.ApproveStudioBatchDesigns)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batches/batch-1/design-approvals", strings.NewReader(`{"design_ids":["design-1"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if svc.approveReq == nil || len(svc.approveReq.DesignIDs) != 1 || svc.approveReq.DesignIDs[0] != "design-1" {
		t.Fatalf("approveReq = %+v, want design id bound", svc.approveReq)
	}
}

func TestStudioBatchTasksHandlerUsesApprovedDesignOwnership(t *testing.T) {
	svc := &stubStudioBatchService{
		createTasksResult: &listingkit.CreateStudioBatchTasksResult{
			Batch: &listingkit.StudioBatchRecord{ID: "batch-1", Status: listingkit.StudioBatchStatusTasksCreated},
			CreatedTasks: []listingkit.SheinStudioCreatedTask{{ID: "task-1", DesignID: "design-1", Title: "Style 1"}},
		},
	}
	h := newStudioSessionHandler(svc)
	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/batches/:batch_id/tasks", h.CreateStudioBatchTasks)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batches/batch-1/tasks", strings.NewReader(`{"design_ids":["design-1"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if svc.createTasksReq == nil || len(svc.createTasksReq.DesignIDs) != 1 || svc.createTasksReq.DesignIDs[0] != "design-1" {
		t.Fatalf("createTasksReq = %+v, want design id bound", svc.createTasksReq)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit/api -run "TestStudioBatchGenerateHandlerStartsItemizedGeneration|TestStudioBatchApproveDesignsHandlerBindsIDs|TestStudioBatchTasksHandlerUsesApprovedDesignOwnership"`

Expected: FAIL with missing handler methods and request types for batch generation, design approvals, and item-owned task creation

- [ ] **Step 3: Add new batch action request types and routes**

```go
type RetryStudioBatchItemsRequest struct {
	ItemIDs []string `json:"item_ids,omitempty"`
}

type CreateStudioBatchTasksRequest struct {
	DesignIDs []string `json:"design_ids,omitempty"`
}

func (h *studioSessionHandler) StartStudioBatchGeneration(c *gin.Context) {
	detail, err := h.service.StartStudioBatchGeneration(requestContext(c), c.Param("batch_id"))
	if err != nil {
		writeStudioSessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *studioSessionHandler) RetryStudioBatchItems(c *gin.Context) {
	var req listingkit.RetryStudioBatchItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	detail, err := h.service.RetryStudioBatchItems(requestContext(c), c.Param("batch_id"), &req)
	if err != nil {
		writeStudioSessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (s *taskStudioBatchService) CreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {
	designs, err := s.repo.ListStudioMaterializedDesignsByIDs(ctx, strings.TrimSpace(batchID), req.DesignIDs)
	if err != nil {
		return nil, err
	}
	created := make([]SheinStudioCreatedTask, 0, len(designs))
	for _, design := range designs {
		item, err := s.repo.GetStudioBatchItem(ctx, design.ItemID)
		if err != nil {
			return nil, err
		}
		createdForItem, err := s.createTasksForItemDesign(ctx, item, design)
		if err != nil {
			return nil, err
		}
		created = append(created, createdForItem...)
	}
	return &CreateStudioBatchTasksResult{CreatedTasks: created}, nil
}
```

- [ ] **Step 4: Run handler tests to verify the new API cutover passes**

Run: `go test ./internal/listingkit/api -run "TestStudioBatchGenerateHandlerStartsItemizedGeneration|TestStudioBatchApproveDesignsHandlerBindsIDs|TestStudioBatchTasksHandlerUsesApprovedDesignOwnership"`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/studio_session_api_model.go internal/listingkit/api/handler.go internal/listingkit/api/studio_sessions_handler.go internal/listingkit/httpapi/routes.go internal/listingkit/api/studio_batches_handler_test.go internal/listingkit/studio_session_handler.go
git commit -m "feat: add itemized studio batch action routes"
```

## Task 5: Replace Frontend Types and Batch API Client

**Files:**
- Modify: `web/listingkit-ui/src/lib/types/shein-studio.ts`
- Create: `web/listingkit-ui/src/lib/api/shein-studio-batches.ts`
- Create: `web/listingkit-ui/src/lib/api/shein-studio-batches.test.ts`
- Modify: `web/listingkit-ui/src/lib/utils/shein-studio-batches.ts`
- Delete: `web/listingkit-ui/src/lib/api/shein-studio-sessions.ts`
- Test: `web/listingkit-ui/src/lib/api/shein-studio-batches.test.ts`

- [ ] **Step 1: Write the failing frontend API tests**

```ts
it("maps itemized batch detail responses", async () => {
  server.use(
    http.get("*/api/listing-kits/studio/batches/batch-1", () =>
      HttpResponse.json({
        batch: { id: "batch-1", status: "review_ready", prompt: "botanical" },
        items: [
          {
            item: {
              id: "item-1",
              target_group_key: "size:1200x1200",
              status: "review_ready",
            },
            designs: [
              {
                id: "design-1",
                item_id: "item-1",
                target_group_key: "size:1200x1200",
                image_url: "https://cdn.example.com/design-1.png",
                approved: true,
              },
            ],
          },
        ],
      }),
    ),
  );

  await expect(getSheinStudioBatchDetail("batch-1")).resolves.toMatchObject({
    batch: { id: "batch-1", status: "review_ready" },
    items: [{ item: { id: "item-1" }, designs: [{ id: "design-1", approved: true }] }],
  });
});

it("posts approval requests by design ids", async () => {
  const requests: unknown[] = [];
  server.use(
    http.post("*/api/listing-kits/studio/batches/batch-1/design-approvals", async ({ request }) => {
      requests.push(await request.json());
      return HttpResponse.json({ batch: { id: "batch-1" }, items: [] });
    }),
  );

  await approveSheinStudioBatchDesigns("batch-1", ["design-1", "design-2"]);
  expect(requests).toEqual([{ design_ids: ["design-1", "design-2"] }]);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- --runInBand web/listingkit-ui/src/lib/api/shein-studio-batches.test.ts`

Expected: FAIL with missing client functions such as `getSheinStudioBatchDetail` and `approveSheinStudioBatchDesigns`

- [ ] **Step 3: Replace flat batch types with itemized types and API client**

```ts
export type SheinStudioMaterializedDesign = {
  id: string;
  batchId: string;
  itemId: string;
  sourceAttemptId: string;
  targetGroupKey: string;
  targetGroupLabel?: string;
  imageUrl: string;
  approved: boolean;
  reviewNote?: string;
  role?: string;
  roleLabel?: string;
  productImageUrls?: string[];
};

export type SheinStudioBatchItem = {
  id: string;
  targetGroupKey: string;
  targetGroupLabel?: string;
  status: "pending" | "generating" | "awaiting_materialization" | "review_ready" | "failed";
  selectionCount: number;
  lastError?: string;
  designs: SheinStudioMaterializedDesign[];
};

export async function getSheinStudioBatchDetail(batchId: string) {
  return parseStudioBatchDetailResponse(
    await apiRequest<unknown>(`/studio/batches/${batchId}`),
  );
}

export async function approveSheinStudioBatchDesigns(batchId: string, designIds: string[]) {
  return parseStudioBatchDetailResponse(
    await apiRequest<unknown>(`/studio/batches/${batchId}/design-approvals`, {
      method: "POST",
      body: { design_ids: designIds },
    }),
  );
}
```

- [ ] **Step 4: Run tests to verify the new API client passes**

Run: `npm test -- --runInBand web/listingkit-ui/src/lib/api/shein-studio-batches.test.ts`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/lib/types/shein-studio.ts web/listingkit-ui/src/lib/api/shein-studio-batches.ts web/listingkit-ui/src/lib/api/shein-studio-batches.test.ts web/listingkit-ui/src/lib/utils/shein-studio-batches.ts
git rm web/listingkit-ui/src/lib/api/shein-studio-sessions.ts
git commit -m "feat: add itemized shein studio batch client"
```

## Task 6: Refactor Workbench, Review, and Task Creation UI to Use Items

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-actions.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-workspace.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-preview-grid.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-task-creation-actions.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-detail.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-detail.test.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`

- [ ] **Step 1: Write the failing UI tests**

```tsx
it("renders designs grouped under item ownership after loading a batch", async () => {
  getSheinStudioBatchDetail.mockResolvedValue({
    batch: { id: "batch-1", status: "review_ready", prompt: "botanical" },
    items: [
      {
        item: {
          id: "item-1",
          targetGroupKey: "size:1200x1200",
          targetGroupLabel: "1200 x 1200",
          status: "review_ready",
          selectionCount: 3,
        },
        designs: [
          {
            id: "design-1",
            batchId: "batch-1",
            itemId: "item-1",
            sourceAttemptId: "attempt-1",
            targetGroupKey: "size:1200x1200",
            imageUrl: "https://cdn.example.com/design-1.png",
            approved: false,
          },
        ],
      },
    ],
  });

  render(<SheinStudioWorkbench initialBatchId="batch-1" />);

  expect(await screen.findByText("1200 x 1200")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "批准" })).toBeInTheDocument();
});

it("submits approved design ids through the batch task endpoint", async () => {
  createStudioBatchTasks.mockResolvedValue({
    batch: { id: "batch-1", status: "tasks_created" },
    items: [],
    createdTasks: [{ id: "task-1", title: "Style 1", designId: "design-1" }],
  });

  render(<SheinStudioBatchDetail batchId="batch-1" />);

  await user.click(await screen.findByRole("button", { name: "批准" }));
  await user.click(screen.getByRole("button", { name: /生成 SHEIN 资料/i }));

  expect(createStudioBatchTasks).toHaveBeenCalledWith("batch-1", ["design-1"]);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- --runInBand web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-detail.test.tsx`

Expected: FAIL because the workbench still expects flat `designs`, `selectedIds`, and `generationJobs`

- [ ] **Step 3: Replace flat workbench state and UI selectors with itemized state**

```ts
export type SheinStudioWorkbenchState = {
  selection?: SDSProductVariantSelection;
  prompt: string;
  styleCount: string;
  variationIntensity: SheinStudioVariationIntensity;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  artworkModel: SheinStudioArtworkModel;
  transparentBackground: boolean;
  sheinStoreId: string;
  imageStrategy: SheinStudioImageStrategy;
  groupedImageMode: SheinStudioGroupedImageMode;
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  groupedSelections: GroupedSDSSelectionEligibility[];
  renderSizeImagesWithSds: boolean;
  batchId: string;
  batchStatus: string;
  items: SheinStudioBatchItem[];
  approvedDesignIds: string[];
  isGenerating: boolean;
  isCreatingTasks: boolean;
  generationError: string;
  creatingError: string;
  createdTasks: SheinStudioCreatedTask[];
};

function flattenVisibleDesigns(items: SheinStudioBatchItem[]) {
  return items.flatMap((item) =>
    item.designs.map((design) => ({
      ...design,
      targetGroupLabel: design.targetGroupLabel || item.targetGroupLabel,
    })),
  );
}

async function handleGenerate() {
  if (!activeBatchId) {
    return;
  }
  const detail = await startSheinStudioBatchGeneration(activeBatchId);
  workbench.setField("batchStatus", detail.batch.status);
  workbench.setField("items", detail.items);
}
```

- [ ] **Step 4: Run UI tests to verify itemized rendering and task submission pass**

Run: `npm test -- --runInBand web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-detail.test.tsx`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-actions.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-workspace.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-preview-grid.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-task-creation-actions.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-detail.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-detail.test.tsx
git commit -m "feat: render itemized shein studio batch flow"
```

## Task 7: Delete Obsolete Session-Design Paths and Verify the Full Redesign

**Files:**
- Modify: `internal/listingkit/httpapi/routes.go`
- Modify: `internal/listingkit/api/studio_async_jobs_handler.go`
- Modify: `internal/listingkit/studio_session_handler.go`
- Modify: `internal/listingkit/studio_session_service.go`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`
- Delete: any now-unused session-design helpers discovered during compiler/test cleanup
- Test: `go test ./internal/listingkit/...`
- Test: `npm test -- --runInBand web/listingkit-ui/src/lib/api/shein-studio-batches.test.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-detail.test.tsx`

- [ ] **Step 1: Write the failing cleanup guard tests**

```go
func TestStudioRoutesNoLongerRegisterSessionDesignAppendEndpoints(t *testing.T) {
	descriptors := listingkithttp.ListRoutes(stubStudioSessionRouteHandler{}, nil)
	for _, descriptor := range descriptors {
		if descriptor.Path == "/api/v1/listing-kits/studio/sessions/:session_id/designs" || descriptor.Path == "/api/v1/listing-kits/studio/sessions/:session_id/designs/append" {
			t.Fatalf("route %q should have been removed", descriptor.Path)
		}
	}
}
```

```ts
it("does not call appendSheinStudioSessionDesigns during generation recovery", async () => {
  render(<SheinStudioWorkbench initialBatchId="batch-1" />);
  await screen.findByText("已生成款式");
  expect(appendSheinStudioSessionDesigns).not.toHaveBeenCalled();
});
```

- [ ] **Step 2: Run cleanup guard tests to verify they fail**

Run: `go test ./internal/listingkit/httpapi -run TestStudioRoutesNoLongerRegisterSessionDesignAppendEndpoints`

Run: `npm test -- --runInBand web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`

Expected: FAIL because the obsolete session-design endpoints and frontend append path still exist

- [ ] **Step 3: Remove obsolete session-design routes and dead callers**

```go
var studioRoutes = []httproute.Descriptor{
	{Method: http.MethodGet, Path: "/api/v1/listing-kits/studio/batches", Module: "listing-kit-studio", Handler: handler.ListStudioBatches},
	{Method: http.MethodGet, Path: "/api/v1/listing-kits/studio/batches/:batch_id", Module: "listing-kit-studio", Handler: handler.GetStudioBatch},
	{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batches", Module: "listing-kit-studio", Handler: handler.UpsertStudioBatch},
	{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batches/:batch_id/generate", Module: "listing-kit-studio", Handler: handler.StartStudioBatchGeneration},
	{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batches/:batch_id/items/retry", Module: "listing-kit-studio", Handler: handler.RetryStudioBatchItems},
	{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batches/:batch_id/design-approvals", Module: "listing-kit-studio", Handler: handler.ApproveStudioBatchDesigns},
	{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/batches/:batch_id/tasks", Module: "listing-kit-studio", Handler: handler.CreateStudioBatchTasks},
}
```

```ts
const loadBatch = async (batchId: string) => {
  const detail = await getSheinStudioBatchDetail(batchId);
  workbench.applyBatch(detail.batch);
  workbench.setField("items", detail.items);
  workbench.setField(
    "approvedDesignIds",
    detail.items.flatMap((item) => item.designs.filter((design) => design.approved).map((design) => design.id)),
  );
};
```

- [ ] **Step 4: Run verification for backend and frontend redesign**

Run: `go test ./internal/listingkit/...`

Expected: PASS

Run: `npm test -- --runInBand web/listingkit-ui/src/lib/api/shein-studio-batches.test.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-detail.test.tsx`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/httpapi/routes.go internal/listingkit/api/studio_async_jobs_handler.go internal/listingkit/studio_session_handler.go internal/listingkit/studio_session_service.go web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx
git commit -m "refactor: remove session-centered shein studio generation paths"
```
