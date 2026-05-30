# Task Processor Framework Phase 16 ListingKit Action Refresh Fallback Hydration Ownership Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the remaining refresh-side ownership complexity in ListingKit by making post-action refresh extraction and fallback hydration flow through explicit feature-owned seams instead of remaining clustered inside `taskGenerationActionRefreshPhase.run(...)`.

**Architecture:** Reuse the same bounded-seam pattern already established in `Phase 10A` and `Phase 15`. Do **not** invent a generic refresh framework. Instead, split the current refresh block in `task_generation_action_refresh.go` into explicit ListingKit-owned local seams: refresh-state extraction and fallback hydration. Keep business behavior unchanged and preserve current action result projection semantics.

**Tech Stack:** Go, existing ListingKit task generation action flow, current-state snapshot/view seams, render-preview helpers, source-boundary guardrails

**Out of Scope For This Slice:**

- redesigning action business rules
- reopening `ExecuteTaskGenerationAction(...)` execution/projection seams from `Phase 10A`
- reopening current-state snapshot/view seams from `Phase 15`
- redesigning temporal branching
- inventing a repo-wide refresh abstraction
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 15`, current state acquisition and current view derivation are explicit, but [internal/listingkit/task_generation_action_refresh.go](/D:/code/task-processor/internal/listingkit/task_generation_action_refresh.go:19) still carries another mixed-responsibility block.

Today `taskGenerationActionRefreshPhase.run(...)` still jointly decides:

1. how current result is refreshed after action execution
2. how current overview is extracted from the refreshed result
3. how current platform render previews are derived from refreshed state
4. when platform render previews fall back to `baseResult`
5. when `currentResult.PlatformAssetRenderPreviews` is backfilled
6. when `currentResult.AssetRenderPreviews` is backfilled from `baseResult`

The problem is not only method size. The real problem is that refresh extraction and fallback hydration are still implicit and live in one block, so future response-shaping changes can leak across it without one clear seam to evolve or protect.

---

## Target Outcome

At the end of `Phase 16`:

- refreshed current-state extraction flows through an explicit ListingKit-owned seam
- fallback hydration flows through an explicit ListingKit-owned seam
- `taskGenerationActionRefreshPhase.run(...)` becomes more orchestration-focused
- current preview fallback behavior remains unchanged
- boundary tests lock the new ownership split

---

## Task 1: Extract refresh-state extraction seam

**Files:**
- Create: `internal/listingkit/task_generation_action_refresh_extract.go`
- Modify: `internal/listingkit/task_generation_action_refresh.go`
- Modify tests if needed:
  - `internal/listingkit/service_generation_retry_test.go`
  - `internal/listingkit/task_generation_service_test.go`

- [ ] **Step 1: Write the failing refresh extraction tests**

Add focused coverage that locks:

1. current result is still refreshed through `getCurrentListingKitResult(...)`
2. overview still comes from the refreshed current result
3. platform render previews still come from the refreshed current result and query before any fallback hydration

Suggested seam shape:

```go
type taskGenerationActionRefreshExtractPhase struct {
	service *taskGenerationService
}

type taskGenerationActionRefreshExtractResult struct {
	currentResult          *ListingKitResult
	overview               *AssetGenerationOverview
	platformRenderPreviews []PlatformAssetRenderPreviews
}

func buildTaskGenerationActionRefreshExtractPhase(service *taskGenerationService) *taskGenerationActionRefreshExtractPhase

func (p *taskGenerationActionRefreshExtractPhase) run(
	ctx context.Context,
	taskID string,
	query *GenerationQueueQuery,
) (*taskGenerationActionRefreshExtractResult, error)
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionRefreshExtract.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the refresh extraction seam**

Create `task_generation_action_refresh_extract.go` so the seam owns:

- `getCurrentListingKitResult(...)`
- current overview extraction
- current platform render-preview derivation

Important:

- preserve current error behavior
- do not hydrate fallback values here
- keep extraction feature-local and narrow

- [ ] **Step 4: Route `taskGenerationActionRefreshPhase.run(...)` through the extraction seam**

Update [task_generation_action_refresh.go](/D:/code/task-processor/internal/listingkit/task_generation_action_refresh.go:19) so the inline refresh extraction block is replaced by one seam handoff.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionRefreshExtract.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_action_refresh_extract.go internal/listingkit/task_generation_action_refresh.go internal/listingkit/service_generation_retry_test.go internal/listingkit/task_generation_service_test.go
git commit -m "refactor: extract listingkit action refresh extraction seam"
```

---

## Task 2: Extract refresh fallback hydration seam

**Files:**
- Create: `internal/listingkit/task_generation_action_refresh_hydration.go`
- Modify: `internal/listingkit/task_generation_action_refresh.go`
- Modify tests if needed:
  - `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Write the failing fallback hydration tests**

Add focused coverage that locks:

1. platform render previews still fall back to `baseResult` when refreshed state is sparse
2. `currentResult.PlatformAssetRenderPreviews` is still backfilled when needed
3. `currentResult.AssetRenderPreviews` is still backfilled from `baseResult` when needed

Suggested seam shape:

```go
type taskGenerationActionRefreshHydrationPhase struct{}

func buildTaskGenerationActionRefreshHydrationPhase() *taskGenerationActionRefreshHydrationPhase

func (p *taskGenerationActionRefreshHydrationPhase) run(
	baseResult *ListingKitResult,
	refresh *taskGenerationActionRefreshExtractResult,
) *taskGenerationActionRefreshResult
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionRefreshHydration.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the fallback hydration seam**

Create `task_generation_action_refresh_hydration.go` so the seam owns:

- platform preview fallback from `baseResult`
- backfill of `currentResult.PlatformAssetRenderPreviews`
- backfill of `currentResult.AssetRenderPreviews`
- final `taskGenerationActionRefreshResult` assembly

Important:

- preserve existing fallback priority exactly
- do not reload current state here
- do not move projection concerns here

- [ ] **Step 4: Route refresh through the hydration seam**

Update [task_generation_action_refresh.go](/D:/code/task-processor/internal/listingkit/task_generation_action_refresh.go:19) so fallback hydration becomes a single seam handoff after extraction.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionRefreshHydration.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_action_refresh_hydration.go internal/listingkit/task_generation_action_refresh.go internal/listingkit/service_generation_retry_test.go
git commit -m "refactor: extract listingkit action refresh fallback hydration seam"
```

---

## Task 3: Lock action refresh ownership guardrails

**Files:**
- Create: `internal/listingkit/phase16_action_refresh_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/service_generation_retry_test.go`
- Verify:
  - `internal/listingkit/task_generation_action_refresh.go`
  - `internal/listingkit/task_generation_action_refresh_extract.go`
  - `internal/listingkit/task_generation_action_refresh_hydration.go`

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. `taskGenerationActionRefreshPhase.run(...)` delegates extraction then hydration in order
2. extraction seam owns refreshed current-state extraction, but not fallback hydration
3. hydration seam owns fallback/backfill behavior, but not current-state reload

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionRefresh.*Boundary" -count=1
```

Expected: FAIL until the guardrails reflect the final seam split.

- [ ] **Step 3: Keep the guardrails low-fragility**

Anchor the ownership tests on:

- helper names
- occurrence counts
- explicit forbidden helper calls
- responsibility-level signals

Avoid fragile dependence on:

- local variable names
- exact conditional layout
- whitespace-sensitive snippets

- [ ] **Step 4: Run final action refresh verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionRefresh.*|TestTaskGenerationCurrentState.*|TestTaskGenerationAction.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase16_action_refresh_boundary_test.go internal/listingkit/task_generation_action_refresh.go internal/listingkit/task_generation_action_refresh_extract.go internal/listingkit/task_generation_action_refresh_hydration.go internal/listingkit/service_generation_retry_test.go
git commit -m "test: lock listingkit action refresh boundaries"
```

---

## Verification Checklist For The Whole Phase

At the end of the full phase, run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionRefresh.*|TestTaskGenerationCurrentState.*|TestTaskGenerationAction.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

If unrelated working-tree changes are still present, do **not** silently broaden this phase to fix them. Record that broader verification may still be noisy for out-of-slice reasons.
