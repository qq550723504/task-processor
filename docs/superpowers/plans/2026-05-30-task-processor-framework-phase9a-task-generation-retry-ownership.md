# Task Processor Framework Phase 9A ListingKit Task Generation Retry Ownership Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the remaining retry-flow ownership complexity in ListingKit by making retry mutation, retry durability writes, and retry result projection flow through explicit feature-owned seams instead of remaining inline inside `RetryTaskGenerationTasks(...)`.

**Architecture:** Reuse the same bounded-seam pattern already established in `Phase 7A/7B` and `Phase 8A/8B`. Do not invent a generic retry framework. Instead, split the current retry block in `task_generation_service.go` into three ListingKit-owned local seams: retry mutation, retry persistence, and retry result projection. Keep business behavior unchanged and preserve current `GenerationTaskPage` / queue-summary semantics.

**Tech Stack:** Go, existing ListingKit retry service, `assetrepo` mem repository tests, existing retry behavior tests, source-boundary guardrails

**Out of Scope For This Slice:**

- redesigning retry selection business rules
- changing deferred renderer/generator contracts
- unifying service retry flow with workflow asset-dispatch seams
- changing review/finalization behavior
- inventing a repo-wide retry orchestration abstraction

---

## Root Cause This Slice Addresses

After `Phase 8B`, the deferred workflow asset-dispatch path is clearer, but the retry path still concentrates several different responsibilities inside one service method:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:210)

Today `RetryTaskGenerationTasks(...)` still jointly decides:

1. how returned dispatch tasks are merged back into persisted generation tasks
2. how returned assets replace prior target assets in inventory
3. when inventory summary is rebuilt
4. when inventory and generation tasks are durably saved
5. how `ListingKitResult` is rebuilt from the updated inventory
6. when platform image bundles are reattached
7. when generation decoration, preview sync, and review decoration run
8. how the final retry page is projected back to callers

The problem is not just method length. The real problem is that retry ownership is still implicit and crosses three kinds of concern:

- retry mutation
- retry persistence
- retry result projection

That makes future changes risky because behavior changes can leak across one block without one clear seam to test or evolve.

---

## Target Outcome

At the end of `Phase 9A`:

- retry mutation flows through an explicit ListingKit-owned seam
- retry persistence flows through an explicit ListingKit-owned seam
- retry result rebuild / page projection flows through an explicit ListingKit-owned seam
- `RetryTaskGenerationTasks(...)` becomes more orchestration-focused
- current retry behavior and queue-summary semantics remain unchanged
- boundary tests lock the new ownership split

---

## Task 1: Extract retry mutation seam

**Files:**
- Create: `internal/listingkit/task_generation_retry_mutation.go`
- Modify: `internal/listingkit/service_generation_retry_test.go`
- Modify: `internal/listingkit/task_generation_service.go`

- [ ] **Step 1: Write the failing seam-level and behavior checks**

Extend [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) with focused coverage that locks the current mutation semantics:

1. returned dispatch tasks overwrite/append persisted generation tasks
2. returned assets replace generated assets only for retried targets
3. inventory summary is rebuilt after replacement

Add at least one direct seam-level test shape:

```go
func TestRetryGenerationMutationApplyMergesTasksAndReplacesRetriedAssets(t *testing.T) {
	t.Parallel()

	existingTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "planned"},
	}
	selectedTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "planned"},
	}
	dispatchResult := &assetgeneration.Result{
		Tasks: []assetgeneration.Task{
			{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "completed"},
			{ID: "shein:gallery", Platform: "shein", RecipeID: "gallery", ExecutionStatus: "planned"},
		},
		Assets: []asset.AssetRecord{
			{ID: "hero-rendered-1", Kind: asset.KindModelImage, Origin: asset.OriginGenerated, RecipeID: "hero"},
		},
	}
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "hero-stale-1", Kind: asset.KindModelImage, Origin: asset.OriginGenerated, RecipeID: "hero"},
			{ID: "gallery-source-1", Kind: asset.KindGalleryImage, Origin: asset.OriginSource},
		},
		Summary: &asset.InventorySummary{TotalRecords: 2, GeneratedRecords: 1},
	}

	updatedTasks := buildRetryGenerationMutationPhase().run(inventory, existingTasks, selectedTasks, dispatchResult)

	if len(updatedTasks) != 2 || updatedTasks[0].ExecutionStatus != "completed" {
		t.Fatalf("updated tasks = %+v, want merged tasks", updatedTasks)
	}
	if inventory.Summary == nil || inventory.Summary.TotalRecords != len(inventory.Records) {
		t.Fatalf("inventory summary = %+v, want rebuilt summary", inventory.Summary)
	}
}
```

- [ ] **Step 2: Run focused retry mutation verification**

Run:

```powershell
go test ./internal/listingkit -run "TestRetryGenerationMutationApplyMergesTasksAndReplacesRetriedAssets$" -count=1
```

Expected: FAIL because `buildRetryGenerationMutationPhase(...)` does not exist yet.

- [ ] **Step 3: Add the retry mutation seam**

Create `internal/listingkit/task_generation_retry_mutation.go` with a focused local seam that owns:

- `mergeGenerationTasks(...)`
- `replaceGeneratedAssetsForTargets(...)`
- `rebuildInventorySummary(...)`

Shape:

```go
type retryGenerationMutationPhase struct{}

func buildRetryGenerationMutationPhase() *retryGenerationMutationPhase {
	return &retryGenerationMutationPhase{}
}

func (p *retryGenerationMutationPhase) run(
	inventory *asset.Inventory,
	existingTasks []assetgeneration.Task,
	selectedTasks []assetgeneration.Task,
	dispatchResult *assetgeneration.Result,
) []assetgeneration.Task
```

Important:

- keep the seam nil-safe
- keep task merge and inventory mutation together for this slice
- do not add persistence or result rebuild here

- [ ] **Step 4: Route retry flow through the mutation seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:210) so the inline block:

- `mergeGenerationTasks(...)`
- `replaceGeneratedAssetsForTargets(...)`
- `rebuildInventorySummary(...)`

is replaced by one `buildRetryGenerationMutationPhase().run(...)` handoff.

- [ ] **Step 5: Re-run focused retry mutation verification**

Run:

```powershell
go test ./internal/listingkit -run "TestRetryGenerationMutationApplyMergesTasksAndReplacesRetriedAssets$" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_retry_mutation.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_retry_test.go
git commit -m "refactor: extract listingkit retry mutation seam"
```

---

## Task 2: Extract retry persistence seam

**Files:**
- Create: `internal/listingkit/task_generation_retry_persist.go`
- Modify: `internal/listingkit/service_generation_retry_test.go`
- Modify: `internal/listingkit/task_generation_service.go`

- [ ] **Step 1: Write the failing persistence-focused tests**

Extend [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) with focused tests that lock:

1. inventory is saved before generation tasks
2. persistence errors still abort retry execution
3. no-op retry paths do not call the persistence seam

Use spies or existing mem-repo wrappers instead of new integration harnesses.

- [ ] **Step 2: Run focused persistence verification**

Run:

```powershell
go test ./internal/listingkit -run "TestRetryGeneration(PersistenceSavesInventoryBeforeTasks|PersistenceReturnsSaveErrors)$" -count=1
```

Expected: FAIL because no explicit retry persistence seam exists yet.

- [ ] **Step 3: Add the retry persistence seam**

Create `internal/listingkit/task_generation_retry_persist.go` with a seam that owns:

- `SaveInventory(...)`
- `SaveGenerationTasks(...)`

Suggested shape:

```go
type retryGenerationPersistPhase struct {
	assetRepo asset.Repository
}

func buildRetryGenerationPersistPhase(assetRepo assetrepo.Repository) *retryGenerationPersistPhase

func (p *retryGenerationPersistPhase) run(
	ctx context.Context,
	taskID string,
	inventory *asset.Inventory,
	updatedTasks []assetgeneration.Task,
) error
```

Important:

- preserve current call order
- preserve current hard-fail behavior on save error
- do not rebuild result or page here

- [ ] **Step 4: Route retry flow through the persistence seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:210) so the inline durability block is replaced by `buildRetryGenerationPersistPhase(s.assetRepo).run(...)`.

- [ ] **Step 5: Re-run focused persistence verification**

Run:

```powershell
go test ./internal/listingkit -run "TestRetryGeneration(PersistenceSavesInventoryBeforeTasks|PersistenceReturnsSaveErrors)$" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_retry_persist.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_retry_test.go
git commit -m "refactor: extract listingkit retry persistence seam"
```

---

## Task 3: Extract retry result projection seam

**Files:**
- Create: `internal/listingkit/task_generation_retry_projection.go`
- Modify: `internal/listingkit/service_generation_retry_test.go`
- Modify: `internal/listingkit/task_generation_service.go`

- [ ] **Step 1: Write the failing projection-focused tests**

Extend [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) with focused tests that lock:

1. rebuilt result refreshes `AssetBundle` and `AssetInventorySummary`
2. platform image bundles are reattached from `updatedTasks`
3. generation decoration, preview sync, and review decoration remain applied
4. matched/executed queue summaries still reflect `selectedTasks` and `dispatchResult.Tasks`

Also add a source-shape test asserting `task_generation_service.go` delegates result rebuild/page assembly through one local helper.

- [ ] **Step 2: Run focused projection verification**

Run:

```powershell
go test ./internal/listingkit -run "TestRetryGeneration(ResultProjectionRebuildsListingKitResult|ResultProjectionBuildsQueues|TaskGenerationServiceFileDelegatesRetryProjection)$" -count=1
```

Expected: FAIL because no explicit projection seam exists yet.

- [ ] **Step 3: Add the retry projection seam**

Create `internal/listingkit/task_generation_retry_projection.go` with a seam that owns:

- rebuilding `ListingKitResult`
- `attachPlatformImageBundles(...)`
- `decorateListingKitResultGeneration(...)`
- `syncAssetRenderPreviews(...)`
- `decorateListingKitResultReview(...)`
- `buildGenerationTaskPage(...)`
- `buildMatchedGenerationQueue(...)`

Suggested shape:

```go
type retryGenerationProjectionPhase struct {
	assetRecipeResolver assetrecipe.Resolver
	assetBundleBuilder  assetbundle.Builder
}

func buildRetryGenerationProjectionPhase(
	resolver assetrecipe.Resolver,
	builder assetbundle.Builder,
) *retryGenerationProjectionPhase

func (p *retryGenerationProjectionPhase) run(
	task *Task,
	inventory *asset.Inventory,
	updatedTasks []assetgeneration.Task,
	selectedTasks []assetgeneration.Task,
	dispatchResult *assetgeneration.Result,
	reviews []GenerationReviewRecord,
) (*ListingKitResult, *GenerationTaskPage)
```

Important:

- keep current queue semantics unchanged
- keep review decoration after task-result persistence call site
- do not move repo writes into this seam

- [ ] **Step 4: Route retry flow through the projection seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:210) so the rebuilt-result / page block is replaced by one projection seam call plus the existing `SaveTaskResult(...)` handoff.

- [ ] **Step 5: Re-run focused projection verification**

Run:

```powershell
go test ./internal/listingkit -run "TestRetryGeneration(ResultProjectionRebuildsListingKitResult|ResultProjectionBuildsQueues|TaskGenerationServiceFileDelegatesRetryProjection)$" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_retry_projection.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_retry_test.go
git commit -m "refactor: extract listingkit retry projection seam"
```

---

## Task 4: Lock retry ownership boundaries

**Files:**
- Create: `internal/listingkit/phase9_task_generation_retry_boundary_test.go`
- Modify: `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Write the boundary tests**

Create [phase9_task_generation_retry_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase9_task_generation_retry_boundary_test.go:1) to lock:

1. `task_generation_service.go` delegates retry mutation to `buildRetryGenerationMutationPhase().run(`
2. `task_generation_service.go` delegates retry persistence to `buildRetryGenerationPersistPhase(s.assetRepo).run(`
3. `task_generation_service.go` delegates retry projection to `buildRetryGenerationProjectionPhase(...).run(`
4. `task_generation_service.go` no longer directly contains:
   - `mergeGenerationTasks(`
   - `replaceGeneratedAssetsForTargets(`
   - `rebuildInventorySummary(`
   - `SaveInventory(`
   - `SaveGenerationTasks(`
   - `attachPlatformImageBundles(`
5. each new file owns only its intended side of the retry flow

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationRetry(FlowDelegatesToOwnedSeams|MutationFileOwnsRetryMutation|PersistFileOwnsRetryPersistence|ProjectionFileOwnsRetryProjection)" -count=1
```

Expected: PASS

- [ ] **Step 3: Run full verification**

Run:

```powershell
go test ./internal/listingkit -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/listingkit/phase9_task_generation_retry_boundary_test.go internal/listingkit/service_generation_retry_test.go
git commit -m "test: lock listingkit retry ownership boundaries"
```

---

## Self-Review Checklist

Before executing, verify the plan still satisfies the scope:

- it only clarifies retry ownership; it does not redesign retry business policy
- it keeps the work fully feature-owned inside `internal/listingkit`
- it does not force the retry path to exactly mirror workflow asset-dispatch seam names beyond what helps local clarity
- it preserves `GenerationTaskPage` / matched queue / executed queue behavior
- it adds both behavior and source-boundary protection

## Expected Outcome

When `Phase 9A` is complete:

- [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:1) will no longer be the primary shared home of retry mutation, persistence, and result projection at the same time
- retry mutation, persistence, and result projection will each have explicit local homes
- existing retry behavior will stay protected in tests
- the retry path will be easier to evolve without reopening the now-stable workflow asset-dispatch seams
