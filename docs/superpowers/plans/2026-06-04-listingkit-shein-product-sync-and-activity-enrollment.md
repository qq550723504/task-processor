# ListingKit SHEIN Product Sync And Activity Enrollment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a ListingKit-owned SHEIN on-shelf product mirror with cost-price precedence, candidate generation, and reusable activity enrollment flows that support both manual and scheduled execution.

**Architecture:** Add a new ListingKit SHEIN operations slice made of durable records, repository adapters, service orchestration, and HTTP routes. Reuse `internal/shein/api/product` for data fetching and adapt `internal/shein/activity` behind a narrow ListingKit-facing enrollment bridge so ListingKit owns product truth while preserving the mature SHEIN campaign logic.

**Tech Stack:** Go, Gin, GORM, existing ListingKit mem/gorm repository pattern, existing ListingKit HTTP module wiring, existing SHEIN API clients and activity services

---

## File Structure

### New ListingKit domain and persistence files

- Create: `internal/listingkit/shein_sync_model.go`
  - synced product, sync job, candidate, enrollment run/item records and status constants
- Create: `internal/listingkit/shein_sync_repository.go`
  - repository interfaces for sync products, jobs, candidates, and enrollment runs
- Create: `internal/listingkit/store/shein_sync_repo.go`
  - GORM repository implementations and query helpers
- Create: `internal/listingkit/store/shein_sync_repo_test.go`
  - persistence tests for uniqueness, update, inactive marking, candidate storage, and run history
- Create: `internal/listingkit/store/shein_sync_mem_store.go`
  - in-memory repo for tests and non-DB bootstrap parity

### New ListingKit services and adapters

- Create: `internal/listingkit/shein_sync_service.go`
  - sync orchestration and cost-price precedence application
- Create: `internal/listingkit/shein_candidate_service.go`
  - candidate refresh and eligibility evaluation
- Create: `internal/listingkit/shein_enrollment_service.go`
  - manual and scheduled enrollment execution using shared path
- Create: `internal/listingkit/shein_activity_adapter.go`
  - adapter from ListingKit candidate rows into reused `internal/shein/activity` contract
- Create: `internal/listingkit/shein_cost_resolver.go`
  - ListingKit-facing cost resolver abstraction and precedence rules
- Create: `internal/listingkit/shein_sync_service_test.go`
  - sync service tests
- Create: `internal/listingkit/shein_candidate_service_test.go`
  - candidate service tests
- Create: `internal/listingkit/shein_enrollment_service_test.go`
  - enrollment service tests

### New HTTP and runtime wiring files

- Modify: `internal/listingkit/interfaces.go`
  - add new service interfaces for sync, candidate, and enrollment operations
- Modify: `internal/listingkit/api/handler.go`
  - extend handler dependencies and optional services
- Create: `internal/listingkit/api/shein_sync_handler.go`
  - HTTP handlers for sync, product list, cost update, candidate refresh/list/review, and enrollment runs
- Create: `internal/listingkit/api/shein_sync_handler_test.go`
  - route binding and response tests
- Modify: `internal/listingkit/httpapi/routes.go`
  - register new `/api/v1/listing-kits/shein-sync/...` routes
- Modify: `internal/listingkit/httpapi/bootstrap.go`
  - build and auto-migrate new repositories; wire new services into module bundle

### New scheduling and execution files

- Create: `internal/listingkit/shein_sync_scheduler.go`
  - scheduled sync entrypoint
- Create: `internal/listingkit/shein_enrollment_scheduler.go`
  - scheduled auto-enrollment entrypoint
- Create: `internal/listingkit/shein_scheduler_test.go`
  - scheduling orchestration tests

---

### Task 1: Add ListingKit SHEIN domain records and repository interfaces

**Files:**
- Create: `internal/listingkit/shein_sync_model.go`
- Create: `internal/listingkit/shein_sync_repository.go`
- Test: `internal/listingkit/shein_sync_model_test.go`

- [ ] **Step 1: Write the failing model/repository contract test**

```go
func TestEffectiveCostPricePrefersManual(t *testing.T) {
	t.Parallel()

	product := &SheinSyncedProductRecord{
		AutoCostPrice:   ptrFloat64(12.50),
		ManualCostPrice: ptrFloat64(15.80),
	}

	ApplyEffectiveCostPrice(product)

	require.NotNil(t, product.EffectiveCostPrice)
	require.Equal(t, 15.80, *product.EffectiveCostPrice)
	require.Equal(t, SheinCostPriceSourceManual, product.CostPriceSource)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit -run TestEffectiveCostPricePrefersManual -count=1`

Expected: FAIL with undefined `SheinSyncedProductRecord`, `ApplyEffectiveCostPrice`, or related cost-source constants.

- [ ] **Step 3: Write the minimal record and repository contract definitions**

```go
type SheinCostPriceSource string

const (
	SheinCostPriceSourceNone   SheinCostPriceSource = "none"
	SheinCostPriceSourceAuto   SheinCostPriceSource = "auto"
	SheinCostPriceSourceManual SheinCostPriceSource = "manual"
)

type SheinSyncedProductRecord struct {
	ID                 int64      `json:"id" gorm:"primaryKey"`
	TenantID           int64      `json:"tenant_id" gorm:"index"`
	StoreID            int64      `json:"store_id" gorm:"index"`
	SKCName            string     `json:"skc_name" gorm:"index"`
	ShelfStatus        string     `json:"shelf_status"`
	AutoCostPrice      *float64   `json:"auto_cost_price"`
	ManualCostPrice    *float64   `json:"manual_cost_price"`
	EffectiveCostPrice *float64   `json:"effective_cost_price"`
	CostPriceSource    SheinCostPriceSource `json:"cost_price_source"`
	IsActive           bool       `json:"is_active"`
}

func ApplyEffectiveCostPrice(record *SheinSyncedProductRecord) {
	if record == nil {
		return
	}
	switch {
	case record.ManualCostPrice != nil:
		record.EffectiveCostPrice = ptrFloat64(*record.ManualCostPrice)
		record.CostPriceSource = SheinCostPriceSourceManual
	case record.AutoCostPrice != nil:
		record.EffectiveCostPrice = ptrFloat64(*record.AutoCostPrice)
		record.CostPriceSource = SheinCostPriceSourceAuto
	default:
		record.EffectiveCostPrice = nil
		record.CostPriceSource = SheinCostPriceSourceNone
	}
}

type SheinSyncRepository interface {
	UpsertSyncedProducts(ctx context.Context, records []*SheinSyncedProductRecord) error
	ListSyncedProducts(ctx context.Context, query *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error)
	UpdateManualCostPrice(ctx context.Context, productID int64, manualCost *float64) error
	SaveSyncJob(ctx context.Context, job *SheinSyncJobRecord) error
	SaveCandidates(ctx context.Context, records []*SheinActivityCandidateRecord) error
	CreateEnrollmentRun(ctx context.Context, run *SheinActivityEnrollmentRunRecord) error
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/listingkit -run TestEffectiveCostPricePrefersManual -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/shein_sync_model.go internal/listingkit/shein_sync_repository.go internal/listingkit/shein_sync_model_test.go
git commit -m "feat: add listingkit shein sync domain models"
```

### Task 2: Implement GORM and in-memory repositories for the new SHEIN records

**Files:**
- Create: `internal/listingkit/store/shein_sync_repo.go`
- Create: `internal/listingkit/store/shein_sync_mem_store.go`
- Create: `internal/listingkit/store/shein_sync_repo_test.go`

- [ ] **Step 1: Write the failing repository upsert test**

```go
func TestSheinSyncRepositoryUpsertSyncedProductsByStoreAndSKC(t *testing.T) {
	t.Parallel()

	repo := newTestSheinSyncRepository(t)
	ctx := context.Background()

	first := &listingkit.SheinSyncedProductRecord{TenantID: 1, StoreID: 101, SKCName: "skc-1", ShelfStatus: "ON_SHELF", IsActive: true}
	second := &listingkit.SheinSyncedProductRecord{TenantID: 1, StoreID: 101, SKCName: "skc-1", ShelfStatus: "OFF_SHELF", IsActive: false}

	require.NoError(t, repo.UpsertSyncedProducts(ctx, []*listingkit.SheinSyncedProductRecord{first}))
	require.NoError(t, repo.UpsertSyncedProducts(ctx, []*listingkit.SheinSyncedProductRecord{second}))

	items, total, err := repo.ListSyncedProducts(ctx, &listingkit.SheinSyncedProductQuery{TenantID: 1, StoreID: 101})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, "OFF_SHELF", items[0].ShelfStatus)
	require.False(t, items[0].IsActive)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit/store -run TestSheinSyncRepositoryUpsertSyncedProductsByStoreAndSKC -count=1`

Expected: FAIL with missing repository constructor or missing upsert behavior.

- [ ] **Step 3: Write the minimal repository implementations**

```go
type sheinSyncRepository struct {
	db *gorm.DB
}

func NewSheinSyncRepository(db *gorm.DB) listingkit.SheinSyncRepository {
	return &sheinSyncRepository{db: db}
}

func (r *sheinSyncRepository) UpsertSyncedProducts(ctx context.Context, records []*listingkit.SheinSyncedProductRecord) error {
	for _, record := range records {
		if record == nil {
			continue
		}
		listingkit.ApplyEffectiveCostPrice(record)
		if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"},
				{Name: "store_id"},
				{Name: "skc_name"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"shelf_status",
				"auto_cost_price",
				"manual_cost_price",
				"effective_cost_price",
				"cost_price_source",
				"is_active",
				"updated_at",
			}),
		}).Create(record).Error; err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run repository tests**

Run: `go test ./internal/listingkit/store -run TestSheinSyncRepositoryUpsertSyncedProductsByStoreAndSKC -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/store/shein_sync_repo.go internal/listingkit/store/shein_sync_mem_store.go internal/listingkit/store/shein_sync_repo_test.go
git commit -m "feat: add listingkit shein sync repositories"
```

### Task 3: Build the SHEIN on-shelf sync service and cost resolver

**Files:**
- Create: `internal/listingkit/shein_cost_resolver.go`
- Create: `internal/listingkit/shein_sync_service.go`
- Create: `internal/listingkit/shein_sync_service_test.go`
- Modify: `internal/listingkit/interfaces.go`

- [ ] **Step 1: Write the failing sync-service test**

```go
func TestSyncSheinOnShelfProductsOnlyPersistsOnShelfItems(t *testing.T) {
	t.Parallel()

	productAPI := &stubSheinProductAPI{
		listResponses: []*sheinproduct.ProductListResponse{
			{
				Info: struct {
					Data []sheinproduct.ProductListItem `json:"data"`
					Meta struct{ Count int `json:"count"` } `json:"meta"`
				}{
					Data: []sheinproduct.ProductListItem{
						{SpuName: "spu-1", SkcInfoList: []sheinproduct.SkcInfoItem{{SkcName: "skc-1"}}, ShelfStatus: "ON_SHELF"},
					},
				},
			},
		},
	}

	service := newTestSheinSyncService(t, productAPI, &stubCostResolver{})
	job, err := service.SyncSheinOnShelfProducts(context.Background(), 1, 101, listingkit.SheinSyncTriggerManual)

	require.NoError(t, err)
	require.Equal(t, 1, job.FetchedCount)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit -run TestSyncSheinOnShelfProductsOnlyPersistsOnShelfItems -count=1`

Expected: FAIL with undefined `SyncSheinOnShelfProducts` service or trigger constants.

- [ ] **Step 3: Write the minimal sync and cost resolver implementation**

```go
type SheinSyncTriggerMode string

const (
	SheinSyncTriggerManual   SheinSyncTriggerMode = "manual"
	SheinSyncTriggerSchedule SheinSyncTriggerMode = "schedule"
)

type SheinCostResolver interface {
	ResolveByStoreAndSKC(ctx context.Context, tenantID, storeID int64, skcName string, supplierCode string) (*float64, error)
}

type SheinSyncService interface {
	SyncSheinOnShelfProducts(ctx context.Context, tenantID, storeID int64, triggerMode SheinSyncTriggerMode) (*SheinSyncJobRecord, error)
}

func (s *sheinSyncService) SyncSheinOnShelfProducts(ctx context.Context, tenantID, storeID int64, triggerMode SheinSyncTriggerMode) (*SheinSyncJobRecord, error) {
	job := &SheinSyncJobRecord{TenantID: tenantID, StoreID: storeID, TriggerMode: string(triggerMode), Status: "running"}
	if err := s.repo.SaveSyncJob(ctx, job); err != nil {
		return nil, err
	}
	req := &sheinproduct.ProductListRequest{
		Language:  "en",
		ShelfType: "ON_SHELF",
		SortType:  1,
	}
	resp, err := s.productAPI.ListProducts(1, 100, req)
	if err != nil {
		job.Status = "failed"
		job.ErrorSummary = err.Error()
		_ = s.repo.SaveSyncJob(ctx, job)
		return nil, err
	}
	records := make([]*SheinSyncedProductRecord, 0, len(resp.Info.Data))
	for _, item := range resp.Info.Data {
		for _, skc := range item.SkcInfoList {
			record := &SheinSyncedProductRecord{
				TenantID:    tenantID,
				StoreID:     storeID,
				SpuName:     item.SpuName,
				SKCName:     skc.SkcName,
				ShelfStatus: item.ShelfStatus,
				IsActive:    true,
			}
			record.AutoCostPrice, _ = s.costResolver.ResolveByStoreAndSKC(ctx, tenantID, storeID, skc.SkcName, skc.SupplierCode)
			ApplyEffectiveCostPrice(record)
			records = append(records, record)
		}
	}
	if err := s.repo.UpsertSyncedProducts(ctx, records); err != nil {
		return nil, err
	}
	job.Status = "succeeded"
	job.FetchedCount = len(records)
	return job, s.repo.SaveSyncJob(ctx, job)
}
```

- [ ] **Step 4: Run the sync-service tests**

Run: `go test ./internal/listingkit -run 'TestSyncSheinOnShelfProducts|TestEffectiveCostPrice' -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/interfaces.go internal/listingkit/shein_cost_resolver.go internal/listingkit/shein_sync_service.go internal/listingkit/shein_sync_service_test.go
git commit -m "feat: add listingkit shein on-shelf sync service"
```

### Task 4: Build candidate refresh from the ListingKit mirror

**Files:**
- Create: `internal/listingkit/shein_candidate_service.go`
- Create: `internal/listingkit/shein_candidate_service_test.go`

- [ ] **Step 1: Write the failing candidate-refresh test**

```go
func TestRefreshSheinActivityCandidatesSkipsProductsWithoutEffectiveCost(t *testing.T) {
	t.Parallel()

	repo := &stubSheinSyncRepository{
		products: []listingkit.SheinSyncedProductRecord{
			{ID: 1, TenantID: 1, StoreID: 101, SKCName: "skc-1", ShelfStatus: "ON_SHELF", IsActive: true},
			{ID: 2, TenantID: 1, StoreID: 101, SKCName: "skc-2", ShelfStatus: "ON_SHELF", IsActive: true, EffectiveCostPrice: ptrFloat64(15)},
		},
	}

	service := newTestSheinCandidateService(repo)
	count, err := service.RefreshSheinActivityCandidates(context.Background(), 1, 101, "PROMOTION")

	require.NoError(t, err)
	require.Equal(t, 1, count)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit -run TestRefreshSheinActivityCandidatesSkipsProductsWithoutEffectiveCost -count=1`

Expected: FAIL with undefined candidate service or no filtering behavior.

- [ ] **Step 3: Write the minimal candidate-refresh implementation**

```go
type SheinCandidateService interface {
	RefreshSheinActivityCandidates(ctx context.Context, tenantID, storeID int64, activityType string) (int, error)
}

func (s *sheinCandidateService) RefreshSheinActivityCandidates(ctx context.Context, tenantID, storeID int64, activityType string) (int, error) {
	products, _, err := s.repo.ListSyncedProducts(ctx, &SheinSyncedProductQuery{
		TenantID: tenantID,
		StoreID:  storeID,
		ActiveOnly: true,
	})
	if err != nil {
		return 0, err
	}
	candidates := make([]*SheinActivityCandidateRecord, 0, len(products))
	for _, product := range products {
		if product.EffectiveCostPrice == nil {
			continue
		}
		candidates = append(candidates, &SheinActivityCandidateRecord{
			TenantID:           tenantID,
			StoreID:            storeID,
			SyncedProductID:    product.ID,
			ActivityType:       activityType,
			EligibilityStatus:  "eligible",
			ReviewStatus:       "pending_review",
			EffectiveCostPrice: *product.EffectiveCostPrice,
		})
	}
	return len(candidates), s.repo.SaveCandidates(ctx, candidates)
}
```

- [ ] **Step 4: Run candidate-service tests**

Run: `go test ./internal/listingkit -run TestRefreshSheinActivityCandidatesSkipsProductsWithoutEffectiveCost -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/shein_candidate_service.go internal/listingkit/shein_candidate_service_test.go
git commit -m "feat: add listingkit shein candidate refresh service"
```

### Task 5: Add the ListingKit-to-SHEIN activity adapter and shared enrollment service

**Files:**
- Create: `internal/listingkit/shein_activity_adapter.go`
- Create: `internal/listingkit/shein_enrollment_service.go`
- Create: `internal/listingkit/shein_enrollment_service_test.go`

- [ ] **Step 1: Write the failing enrollment-path test**

```go
func TestExecuteSheinActivityEnrollmentUsesApprovedCandidates(t *testing.T) {
	t.Parallel()

	repo := &stubSheinSyncRepository{
		candidates: []listingkit.SheinActivityCandidateRecord{
			{ID: 1, SyncedProductID: 11, ActivityType: "PROMOTION", ReviewStatus: "approved"},
			{ID: 2, SyncedProductID: 12, ActivityType: "PROMOTION", ReviewStatus: "rejected"},
		},
	}
	adapter := &stubSheinActivityAdapter{}
	service := newTestSheinEnrollmentService(repo, adapter)

	run, err := service.ExecuteSheinActivityEnrollment(context.Background(), 101, "PROMOTION", "manual_confirmed", 1, 2)

	require.NoError(t, err)
	require.Equal(t, 1, run.SubmittedCount)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit -run TestExecuteSheinActivityEnrollmentUsesApprovedCandidates -count=1`

Expected: FAIL with undefined enrollment service or missing review-status filtering.

- [ ] **Step 3: Write the minimal adapter and enrollment implementation**

```go
type SheinActivityEnrollmentAdapter interface {
	EnrollPromotionCandidates(ctx context.Context, storeID int64, products []SheinEnrollmentProduct) (int, []SheinEnrollmentItemResult, error)
}

func (s *sheinEnrollmentService) ExecuteSheinActivityEnrollment(ctx context.Context, storeID int64, activityType string, triggerMode string, candidateIDs ...int64) (*SheinActivityEnrollmentRunRecord, error) {
	candidates, err := s.repo.ListCandidatesByIDs(ctx, candidateIDs)
	if err != nil {
		return nil, err
	}
	approved := make([]SheinEnrollmentProduct, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ReviewStatus != "approved" && candidate.ReviewStatus != "auto_queued" {
			continue
		}
		approved = append(approved, SheinEnrollmentProduct{
			CandidateID:      candidate.ID,
			SyncedProductID:  candidate.SyncedProductID,
			EffectiveCostPrice: candidate.EffectiveCostPrice,
		})
	}
	run := &SheinActivityEnrollmentRunRecord{
		StoreID:        storeID,
		ActivityType:   activityType,
		TriggerMode:    triggerMode,
		Status:         "running",
		CandidateCount: len(candidates),
		SubmittedCount: len(approved),
	}
	if err := s.repo.CreateEnrollmentRun(ctx, run); err != nil {
		return nil, err
	}
	successes, items, err := s.adapter.EnrollPromotionCandidates(ctx, storeID, approved)
	run.SucceededCount = successes
	run.FailedCount = len(items) - successes
	if err != nil {
		run.Status = "failed"
		run.ErrorSummary = err.Error()
	} else {
		run.Status = "succeeded"
	}
	return run, s.repo.SaveEnrollmentResult(ctx, run, items)
}
```

- [ ] **Step 4: Run enrollment-service tests**

Run: `go test ./internal/listingkit -run TestExecuteSheinActivityEnrollmentUsesApprovedCandidates -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/shein_activity_adapter.go internal/listingkit/shein_enrollment_service.go internal/listingkit/shein_enrollment_service_test.go
git commit -m "feat: add listingkit shein enrollment service"
```

### Task 6: Add HTTP handlers and route registration for sync, candidates, and enrollment

**Files:**
- Modify: `internal/listingkit/interfaces.go`
- Modify: `internal/listingkit/api/handler.go`
- Create: `internal/listingkit/api/shein_sync_handler.go`
- Create: `internal/listingkit/api/shein_sync_handler_test.go`
- Modify: `internal/listingkit/httpapi/routes.go`

- [ ] **Step 1: Write the failing route registration test**

```go
func TestAppendRouteDescriptorsIncludesSheinSyncRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()
	module := httpapi.NewHTTPModule(stubRouteHandlerWithSheinSync{})

	require.NoError(t, module.Register(reg))

	keys := routeKeys(reg.Routes())
	require.Contains(t, keys, "POST /api/v1/listing-kits/shein-sync/stores/:store_id/sync")
	require.Contains(t, keys, "GET /api/v1/listing-kits/shein-sync/stores/:store_id/products")
	require.Contains(t, keys, "POST /api/v1/listing-kits/shein-sync/stores/:store_id/enrollments")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit/httpapi -run TestAppendRouteDescriptorsIncludesSheinSyncRoutes -count=1`

Expected: FAIL with missing route descriptors or missing handler interface methods.

- [ ] **Step 3: Write the minimal route and handler integration**

```go
type sheinSyncRouteHandler interface {
	TriggerSheinStoreSync(c *gin.Context)
	ListSheinSyncedProducts(c *gin.Context)
	UpdateSheinSyncedProductCost(c *gin.Context)
	RefreshSheinActivityCandidates(c *gin.Context)
	ListSheinActivityCandidates(c *gin.Context)
	ReviewSheinActivityCandidate(c *gin.Context)
	ExecuteSheinActivityEnrollment(c *gin.Context)
	ListSheinSyncJobs(c *gin.Context)
	ListSheinEnrollmentRuns(c *gin.Context)
}

func appendSheinSyncRouteDescriptors(routes []httproute.Descriptor, handler sheinSyncRouteHandler) []httproute.Descriptor {
	return append(routes,
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/sync", Module: "listing-kit", Handler: handler.TriggerSheinStoreSync},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/products", Module: "listing-kit", Handler: handler.ListSheinSyncedProducts},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/shein-sync/products/:id/cost", Module: "listing-kit", Handler: handler.UpdateSheinSyncedProductCost},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/candidates/refresh", Module: "listing-kit", Handler: handler.RefreshSheinActivityCandidates},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/candidates", Module: "listing-kit", Handler: handler.ListSheinActivityCandidates},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/shein-sync/candidates/:id/review", Module: "listing-kit", Handler: handler.ReviewSheinActivityCandidate},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/shein-sync/stores/:store_id/enrollments", Module: "listing-kit", Handler: handler.ExecuteSheinActivityEnrollment},
	)
}
```

- [ ] **Step 4: Run handler and route tests**

Run: `go test ./internal/listingkit/api ./internal/listingkit/httpapi -run 'TestAppendRouteDescriptorsIncludesSheinSyncRoutes|TestTriggerSheinStoreSync' -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/interfaces.go internal/listingkit/api/handler.go internal/listingkit/api/shein_sync_handler.go internal/listingkit/api/shein_sync_handler_test.go internal/listingkit/httpapi/routes.go
git commit -m "feat: add listingkit shein sync http routes"
```

### Task 7: Wire repositories and services into ListingKit bootstrap

**Files:**
- Modify: `internal/listingkit/httpapi/bootstrap.go`
- Modify: `internal/listingkit/api/handler.go`

- [ ] **Step 1: Write the failing bootstrap test**

```go
func TestBuildModuleWiresSheinSyncServices(t *testing.T) {
	t.Parallel()

	module, err := BuildModule(testBuildModuleInput(t))

	require.NoError(t, err)
	require.NotNil(t, module.Handler)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit/httpapi -run TestBuildModuleWiresSheinSyncServices -count=1`

Expected: FAIL because the new repository builders or handler dependencies are not wired.

- [ ] **Step 3: Write the minimal bootstrap wiring**

```go
type CoreRepositoryBuilders struct {
	Task                 func(*config.Config, *logrus.Logger) (listingkit.Repository, []func() error, error)
	StudioAsyncJob       func(*config.Config, *logrus.Logger) (listingkit.StudioAsyncJobRepository, []func() error, error)
	StudioBatch          func(*config.Config, *logrus.Logger) (listingkit.StudioBatchRepository, []func() error, error)
	StudioBatchRun       func(*config.Config, *logrus.Logger) (listingkit.StudioBatchRunRepository, []func() error, error)
	SheinSync            func(*config.Config, *logrus.Logger) (listingkit.SheinSyncRepository, []func() error, error)
	Subscription         func(*config.Config, *logrus.Logger) (listingsubscription.Repository, []func() error, error)
	// ...
}

// inside BuildService / BuildModule:
sheinSyncRepo, sheinSyncClosers, err := input.Repositories.Core.SheinSync(cfg, logger)
syncService := listingkit.NewSheinSyncService(sheinSyncRepo, sheinProductAPI, costResolver)
candidateService := listingkit.NewSheinCandidateService(sheinSyncRepo, activityEvaluator)
enrollmentService := listingkit.NewSheinEnrollmentService(sheinSyncRepo, activityAdapter)

handler, err := listingkitapi.NewHandler(service,
	listingkitapi.WithSheinSyncServices(syncService, candidateService, enrollmentService),
)
```

- [ ] **Step 4: Run bootstrap and package tests**

Run: `go test ./internal/listingkit/httpapi -run TestBuildModuleWiresSheinSyncServices -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/httpapi/bootstrap.go internal/listingkit/api/handler.go
git commit -m "feat: wire listingkit shein sync services"
```

### Task 8: Add scheduled sync and auto-enrollment orchestration

**Files:**
- Create: `internal/listingkit/shein_sync_scheduler.go`
- Create: `internal/listingkit/shein_enrollment_scheduler.go`
- Create: `internal/listingkit/shein_scheduler_test.go`

- [ ] **Step 1: Write the failing scheduler test**

```go
func TestRunScheduledSheinEnrollmentSyncsThenRefreshesThenEnrolls(t *testing.T) {
	t.Parallel()

	deps := &stubSheinSchedulerDeps{}
	runner := NewSheinEnrollmentScheduler(deps.sync, deps.candidates, deps.enrollment)

	err := runner.Run(context.Background(), 1, 101, "PROMOTION")

	require.NoError(t, err)
	require.Equal(t, []string{"sync", "refresh", "enroll"}, deps.calls)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit -run TestRunScheduledSheinEnrollmentSyncsThenRefreshesThenEnrolls -count=1`

Expected: FAIL with undefined scheduler type or wrong orchestration sequence.

- [ ] **Step 3: Write the minimal scheduler implementations**

```go
type SheinEnrollmentScheduler struct {
	syncService       SheinSyncService
	candidateService  SheinCandidateService
	enrollmentService SheinEnrollmentService
}

func (s *SheinEnrollmentScheduler) Run(ctx context.Context, tenantID, storeID int64, activityType string) error {
	if _, err := s.syncService.SyncSheinOnShelfProducts(ctx, tenantID, storeID, SheinSyncTriggerSchedule); err != nil {
		return err
	}
	if _, err := s.candidateService.RefreshSheinActivityCandidates(ctx, tenantID, storeID, activityType); err != nil {
		return err
	}
	_, err := s.enrollmentService.ExecuteAutoSheinActivityEnrollment(ctx, tenantID, storeID, activityType)
	return err
}
```

- [ ] **Step 4: Run scheduler tests**

Run: `go test ./internal/listingkit -run TestRunScheduledSheinEnrollmentSyncsThenRefreshesThenEnrolls -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/shein_sync_scheduler.go internal/listingkit/shein_enrollment_scheduler.go internal/listingkit/shein_scheduler_test.go
git commit -m "feat: add listingkit shein sync schedulers"
```

### Task 9: Run formatting and broader verification

**Files:**
- Modify: all files from Tasks 1-8 as needed after verification

- [ ] **Step 1: Run Go formatting on touched files**

Run:

```bash
gofmt -w internal/listingkit/shein_sync_model.go internal/listingkit/shein_sync_repository.go internal/listingkit/shein_cost_resolver.go internal/listingkit/shein_sync_service.go internal/listingkit/shein_candidate_service.go internal/listingkit/shein_activity_adapter.go internal/listingkit/shein_enrollment_service.go internal/listingkit/shein_sync_scheduler.go internal/listingkit/shein_enrollment_scheduler.go internal/listingkit/store/shein_sync_repo.go internal/listingkit/store/shein_sync_mem_store.go internal/listingkit/api/shein_sync_handler.go internal/listingkit/httpapi/routes.go internal/listingkit/httpapi/bootstrap.go internal/listingkit/interfaces.go
```

Expected: no output

- [ ] **Step 2: Run focused package diagnostics**

Run:

```bash
go test ./internal/listingkit/... ./internal/shein/activity/... -count=1
```

Expected: PASS

- [ ] **Step 3: Run broader verification**

Run:

```bash
go test ./... -count=1
```

Expected: PASS, or document unrelated pre-existing failures before proceeding

- [ ] **Step 4: Run vet for changed packages**

Run:

```bash
go vet ./internal/listingkit/... ./internal/shein/activity/...
```

Expected: no output

- [ ] **Step 5: Commit the verification and any cleanup**

```bash
git add internal/listingkit internal/shein/activity docs/superpowers/plans/2026-06-04-listingkit-shein-product-sync-and-activity-enrollment.md
git commit -m "test: verify listingkit shein sync enrollment flow"
```

---

## Spec Coverage Check

- ListingKit-owned synced product mirror: covered by Tasks 1-3
- cost-price precedence and manual override: covered by Tasks 1 and 3
- candidate pool generation: covered by Task 4
- reused `internal/shein/activity` enrollment path: covered by Task 5
- ListingKit HTTP routes: covered by Task 6
- bootstrap/runtime wiring: covered by Task 7
- manual and scheduled execution: covered by Tasks 3, 5, and 8
- verification and completion checks: covered by Task 9

## Placeholder Scan

No `TODO`, `TBD`, or "implement later" placeholders were intentionally left in this plan. The one allowed abstraction point is the `SheinActivityEnrollmentAdapter`, which is explicitly assigned in Task 5 rather than deferred.

## Type Consistency Check

- `SheinSyncService.SyncSheinOnShelfProducts` is introduced in Task 3 and used again in Task 8 with the same signature.
- `SheinCandidateService.RefreshSheinActivityCandidates` is introduced in Task 4 and reused in Task 8 with the same signature.
- `SheinActivityEnrollmentRunRecord` and `SheinActivityCandidateRecord` are introduced in Task 1 and referenced consistently in Tasks 5 and 6.

