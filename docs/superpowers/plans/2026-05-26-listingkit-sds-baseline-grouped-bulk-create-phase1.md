# ListingKit SDS Baseline Grouped Bulk Create Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an SDS baseline cache, gate grouped SDS bulk creation on baseline readiness, and reuse baseline data in the ListingKit standard-product workflow without changing the one-task-per-product execution model.

**Architecture:** Introduce a dedicated SDS baseline persistence layer inside `internal/listingkit`, keyed by stable SDS identity instead of request image or copy inputs. Use that baseline in `runStandardProductWorkflow` to restore the stable canonical product skeleton, then continue applying runtime SDS overlays for images and dynamic copy. On the frontend and API boundary, grouped creation becomes an orchestration feature that validates `baseline ready` selections and expands them into ordinary ListingKit tasks with per-group store assignment.

**Tech Stack:** Go, GORM-backed repositories, existing `internal/listingkit` workflow tests, Next.js/TypeScript studio UI, Vitest, `apply_patch`

---

### Task 1: Add failing backend tests for SDS baseline identity and persistence

**Files:**
- Create: `internal/listingkit/sds_baseline_cache_test.go`
- Modify: `internal/listingkit/interfaces.go`
- Test: `internal/listingkit/sds_baseline_cache_test.go`

- [ ] **Step 1: Write the failing tests**

Add tests that define the new SDS baseline identity behavior:

```go
func TestSDSBaselineKeyFromOptions_StableAcrossImageAndCopyChanges(t *testing.T) {
	options := &SDSSyncOptions{
		VariantID:        101,
		ParentProductID:  9001,
		PrototypeGroupID: 7001,
		Variants: []SDSSyncVariantOption{
			{VariantID: 101},
			{VariantID: 102},
		},
	}

	first := sdsBaselineKey("tenant-a", options)
	second := sdsBaselineKey("tenant-a", &SDSSyncOptions{
		VariantID:        101,
		ParentProductID:  9001,
		PrototypeGroupID: 7001,
		ProductName:      "Different copy",
		MockupImageURLs:  []string{"https://example.com/changed.png"},
		Variants: []SDSSyncVariantOption{
			{VariantID: 102},
			{VariantID: 101},
		},
	})

	if first == "" || second == "" {
		t.Fatal("expected non-empty baseline keys")
	}
	if first != second {
		t.Fatalf("baseline key drifted: %q != %q", first, second)
	}
}

func TestMemTaskRepositorySaveAndGetSDSBaselineCache(t *testing.T) {
	repo := NewMemTaskRepository()
	entry := &SDSBaselineCacheEntry{
		TenantID:    "tenant-a",
		BaselineKey: "baseline-key",
		Status:      "ready",
		Version:     1,
		Identity: SDSBaselineIdentity{
			ParentProductID:  9001,
			PrototypeGroupID: 7001,
			VariantID:        101,
			SelectedVariantIDs: []int64{101, 102},
		},
		CanonicalProductBase: &canonical.Product{Title: "Baseline Product"},
	}

	if err := repo.SaveSDSBaselineCache(context.Background(), entry); err != nil {
		t.Fatalf("save baseline: %v", err)
	}
	got, err := repo.GetSDSBaselineCache(context.Background(), "tenant-a", "baseline-key")
	if err != nil {
		t.Fatalf("get baseline: %v", err)
	}
	if got == nil || got.CanonicalProductBase == nil || got.CanonicalProductBase.Title != "Baseline Product" {
		t.Fatalf("unexpected baseline entry: %+v", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/listingkit -run "TestSDSBaselineKeyFromOptions_StableAcrossImageAndCopyChanges|TestMemTaskRepositorySaveAndGetSDSBaselineCache" -count=1`
Expected: FAIL because the baseline cache types and repository methods do not exist yet.

### Task 2: Implement the SDS baseline cache model and repository support

**Files:**
- Create: `internal/listingkit/sds_baseline_cache.go`
- Modify: `internal/listingkit/interfaces.go`
- Modify: `internal/listingkit/store/mem_store.go`
- Modify: `internal/listingkit/store/task_repo.go`
- Test: `internal/listingkit/sds_baseline_cache_test.go`

- [ ] **Step 1: Add the baseline cache model and key builder**

Implement the baseline types and key function in `internal/listingkit/sds_baseline_cache.go`:

```go
type SDSBaselineIdentity struct {
	ParentProductID     int64   `json:"parent_product_id,omitempty"`
	PrototypeGroupID    int64   `json:"prototype_group_id,omitempty"`
	VariantID           int64   `json:"variant_id,omitempty"`
	SelectedVariantIDs  []int64 `json:"selected_variant_ids,omitempty"`
}

type SDSBaselineCacheEntry struct {
	TenantID             string                 `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	BaselineKey          string                 `json:"baseline_key" gorm:"primaryKey;type:varchar(128)"`
	Status               string                 `json:"status,omitempty" gorm:"type:varchar(20);index"`
	Version              int                    `json:"version"`
	SourceTaskID         string                 `json:"source_task_id,omitempty" gorm:"type:varchar(36);index"`
	Identity             SDSBaselineIdentity    `json:"identity" gorm:"type:text"`
	CanonicalProductBase *CanonicalProductCachePayload `json:"canonical_product_base,omitempty" gorm:"type:text"`
	CreatedAt            time.Time              `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            time.Time              `json:"updated_at" gorm:"autoUpdateTime"`
}

func sdsBaselineKey(tenantID string, options *SDSSyncOptions) string {
	// normalize selected variant ids and hash stable identity only
}
```

- [ ] **Step 2: Extend the repository contract**

Add a new repository interface in `internal/listingkit/interfaces.go`:

```go
type SDSBaselineCacheRepository interface {
	GetSDSBaselineCache(ctx context.Context, tenantID string, baselineKey string) (*SDSBaselineCacheEntry, error)
	SaveSDSBaselineCache(ctx context.Context, entry *SDSBaselineCacheEntry) error
}
```

- [ ] **Step 3: Implement in-memory repository support**

Add a baseline cache map to `MemTaskRepository` and implement:

```go
func (r *MemTaskRepository) GetSDSBaselineCache(ctx context.Context, tenantID, baselineKey string) (*listingkit.SDSBaselineCacheEntry, error) {
	key := tenantID + ":" + baselineKey
	if entry, ok := r.sdsBaselineCache[key]; ok {
		return cloneSDSBaselineCacheEntry(entry), nil
	}
	return nil, nil
}

func (r *MemTaskRepository) SaveSDSBaselineCache(ctx context.Context, entry *listingkit.SDSBaselineCacheEntry) error {
	key := entry.TenantID + ":" + entry.BaselineKey
	r.sdsBaselineCache[key] = cloneSDSBaselineCacheEntry(entry)
	return nil
}
```

- [ ] **Step 4: Implement database repository support**

Add the GORM-backed methods in `internal/listingkit/store/task_repo.go`:

```go
func (r *taskRepository) GetSDSBaselineCache(ctx context.Context, tenantID, baselineKey string) (*listingkit.SDSBaselineCacheEntry, error) {
	var entry listingkit.SDSBaselineCacheEntry
	db := applyTenantScope(r.db.WithContext(ctx), ctx, "tenant_id")
	if err := db.Where("baseline_key = ?", tenantID+":"+baselineKey).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &entry, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/listingkit -run "TestSDSBaselineKeyFromOptions_StableAcrossImageAndCopyChanges|TestMemTaskRepositorySaveAndGetSDSBaselineCache" -count=1`
Expected: PASS.

### Task 3: Add failing workflow tests for baseline reuse and fallback

**Files:**
- Modify: `internal/listingkit/workflow_studio_sds_metadata_test.go`
- Modify: `internal/listingkit/service_test.go`
- Test: `internal/listingkit/workflow_studio_sds_metadata_test.go`

- [ ] **Step 1: Write the failing tests**

Add tests that prove `runStandardProductWorkflow` prefers SDS baseline cache when available and falls back when missing:

```go
func TestRunStandardProductWorkflow_UsesSDSBaselineBeforeProductEnrich(t *testing.T) {
	repo := NewMemTaskRepository()
	task := &Task{
		ID: "task-1",
		Request: &GenerateRequest{
			Platforms: []string{"shein"},
			Options: &GenerateOptions{
				SDS: &SDSSyncOptions{
					ParentProductID:  9001,
					PrototypeGroupID: 7001,
					VariantID:        101,
				},
			},
		},
	}
	_ = repo.SaveSDSBaselineCache(context.Background(), &SDSBaselineCacheEntry{
		TenantID:    "",
		BaselineKey: sdsBaselineKey("", task.Request.Options.SDS),
		Status:      "ready",
		Version:     1,
		CanonicalProductBase: mustCanonicalPayload(t, &canonical.Product{Title: "Baseline Title"}),
	})

	svc := newTestListingKitServiceWithRepo(repo)
	state, err := svc.runStandardProductWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}
	if state.result.CanonicalProduct == nil || state.result.CanonicalProduct.Title == "" {
		t.Fatalf("expected canonical product from baseline, got %+v", state.result.CanonicalProduct)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/listingkit -run "TestRunStandardProductWorkflow_UsesSDSBaselineBeforeProductEnrich" -count=1`
Expected: FAIL because the workflow does not look up SDS baseline cache yet.

### Task 4: Implement baseline-aware standard-product workflow

**Files:**
- Modify: `internal/listingkit/service.go`
- Modify: `internal/listingkit/workflow_standard.go`
- Create: `internal/listingkit/sds_baseline_service.go`
- Modify: `internal/listingkit/sds_canonical_metadata.go`
- Test: `internal/listingkit/workflow_studio_sds_metadata_test.go`

- [ ] **Step 1: Add a focused baseline helper**

Create `internal/listingkit/sds_baseline_service.go`:

```go
type sdsBaselineService struct {
	repo Repository
}

func (s *service) sdsBaselineOrDefault() *sdsBaselineService {
	return &sdsBaselineService{repo: s.repo}
}

func (b *sdsBaselineService) GetReadyBaseline(ctx context.Context, task *Task) (*canonical.Product, bool, error) {
	// resolve tenant id, build key, load entry, return canonical clone when status == ready
}
```

- [ ] **Step 2: Use baseline before product-enrich cache lookup**

Update `runStandardProductWorkflow` to try SDS baseline first:

```go
if baseline, ok, err := s.sdsBaselineOrDefault().GetReadyBaseline(ctx, task); err == nil && ok {
	canonicalProduct = baseline
} else if err != nil {
	log.WithError(err).Warn("sds baseline lookup failed; continuing")
}
```

- [ ] **Step 3: Preserve runtime overlays**

Keep the existing SDS overlay flow after baseline hydration so this run still updates images and dynamic title data:

```go
if applySDSSyncMetadataToCanonical(canonicalProduct, result.SDSSync, sdsOptions) {
	result.CatalogProduct = catalog.BuildProduct(canonicalProduct)
}
```

- [ ] **Step 4: Run targeted workflow tests**

Run: `go test ./internal/listingkit -run "TestRunStandardProductWorkflow_UsesSDSBaselineBeforeProductEnrich|TestApplySDSSyncMetadataToCanonical" -count=1`
Expected: PASS.

### Task 5: Add grouped bulk-create eligibility on the frontend and task-creation helper

**Files:**
- Create: `web/listingkit-ui/src/lib/api/sds-baseline.ts`
- Create: `web/listingkit-ui/src/lib/types/sds-baseline.ts`
- Modify: `web/listingkit-ui/src/lib/shein-studio/create-review-tasks.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`
- Create: `web/listingkit-ui/src/lib/shein-studio/grouped-sds-create.test.ts`
- Test: `web/listingkit-ui/src/lib/shein-studio/grouped-sds-create.test.ts`

- [ ] **Step 1: Write the failing frontend tests**

Add tests that prove non-ready selections are rejected before grouped create:

```ts
it("rejects grouped create when a selection is missing baseline readiness", async () => {
  await expect(
    createGroupedSheinReviewTasks({
      groups: [
        {
          sheinStoreId: "869",
          selections: [{ selection: baseSelection, baselineReady: false }],
          designs: [baseDesign],
          selectedIds: [baseDesign.id],
        },
      ],
    }),
  ).rejects.toThrow("baseline ready");
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `npm test -- grouped-sds-create.test.ts`
Expected: FAIL because grouped create helper and readiness checks do not exist.

- [ ] **Step 3: Add baseline readiness types and API**

Create a lightweight readiness shape:

```ts
export type SDSBaselineStatus = "ready" | "missing" | "failed";

export type SDSBaselineReadiness = {
  baselineKey: string;
  status: SDSBaselineStatus;
  reason?: string;
};
```

- [ ] **Step 4: Add grouped task creation helper**

Extend `create-review-tasks.ts` with a grouped helper that validates readiness and reuses the existing single-selection flow:

```ts
export async function createGroupedSheinReviewTasks(input: {
  groups: Array<{
    sheinStoreId: string;
    selection: SDSProductVariantSelection;
    baselineReady: boolean;
    designs: SheinStudioGeneratedDesign[];
    selectedIds: string[];
  }>;
}) {
  for (const group of input.groups) {
    if (!group.baselineReady) {
      throw new Error("Only baseline ready SDS products can be created in grouped mode.");
    }
    await createSheinReviewTasks({
      sheinStoreId: group.sheinStoreId,
      selection: group.selection,
      designs: group.designs,
      selectedIds: group.selectedIds,
      prompt: "",
    });
  }
}
```

- [ ] **Step 5: Update workbench state to carry grouped readiness**

Add fields that allow the UI to track grouped eligibility without rebuilding the whole studio architecture in one shot:

```ts
groupedSelections: Array<{
  selectionId: string;
  baselineStatus: "ready" | "missing" | "failed";
  baselineReason: string;
  sheinStoreId: string;
}>;
```

- [ ] **Step 6: Run targeted frontend tests**

Run: `npm test -- grouped-sds-create.test.ts shein-studio-workbench.test.tsx`
Expected: PASS.

### Task 6: Run end-to-end regression checks for phase 1

**Files:**
- Test: `internal/listingkit/...`
- Test: `web/listingkit-ui/...`

- [ ] **Step 1: Run backend regression tests**

Run: `go test ./internal/listingkit -count=1`
Expected: PASS.

- [ ] **Step 2: Run frontend targeted regression tests**

Run: `npm test -- shein-studio-workbench.test.tsx shein-studio-batch-detail.test.tsx grouped-sds-create.test.ts`
Expected: PASS.

- [ ] **Step 3: Run frontend type checks**

Run: `npm run typecheck`
Expected: PASS.

- [ ] **Step 4: Commit phase 1**

Run:

```bash
git add internal/listingkit web/listingkit-ui/src docs/superpowers/plans/2026-05-26-listingkit-sds-baseline-grouped-bulk-create-phase1.md
git commit -m "feat: add SDS baseline grouped bulk create phase 1"
```
