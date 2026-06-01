# Task Processor Framework Phase 15 ListingKit Generation State Acquisition Ownership Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the remaining generation-state ownership complexity in ListingKit by making current listing/result acquisition and current queue/overview/render-preview derivation flow through explicit feature-owned seams instead of remaining clustered across `task_generation_service.go`.

**Architecture:** Reuse the same bounded-seam pattern already established in `Phase 12`, `Phase 13`, and `Phase 14`. Do **not** invent a generic state-provider framework. Instead, split the current “current generation state” helpers in `task_generation_service.go` into explicit ListingKit-owned local seams: state snapshot acquisition and state view derivation. Keep business behavior unchanged and preserve current action-side and read-side consumers.

**Tech Stack:** Go, existing ListingKit task generation service, generation review decorators, action-side render-preview helpers, source-boundary guardrails

**Out of Scope For This Slice:**

- redesigning action business rules
- reopening `ExecuteTaskGenerationAction(...)` execution/projection seams from `Phase 10A`
- reopening navigation dispatch seams from `Phase 10B/11`
- reopening review-read / queue-read / task-read seams from `Phase 12/13/14`
- inventing a repo-wide “current state” abstraction
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 14`, the read-side seams are clearer, but `task_generation_service.go` still carries another shared ownership cluster:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:318)
- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:326)
- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:334)
- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:342)

Today those helpers still jointly decide:

1. how the current `Task` / `ListingKitResult` / generation tasks / reviews are acquired
2. how a decorated current result is rebuilt with generation/review state
3. how current queue and current overview views are derived from that result
4. how action-side render previews are derived from the current result

The problem is not method count by itself. The real problem is that multiple action/read helpers still depend on an implicit, clustered “current generation state” path with no single feature-owned seam to evolve or protect.

---

## Target Outcome

At the end of `Phase 15`:

- current generation-state acquisition flows through an explicit ListingKit-owned seam
- queue/overview/render-preview derivation flows through an explicit ListingKit-owned seam
- the remaining `getCurrent*` helpers become thin delegators or disappear into the seams
- action-side and read-side behavior stays unchanged
- boundary tests lock the new ownership split

---

## Task 1: Extract current generation-state snapshot seam

**Files:**
- Create: `internal/listingkit/task_generation_current_state_snapshot.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify tests if needed:
  - `internal/listingkit/task_generation_service_test.go`
  - `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Write the failing current-state snapshot tests**

Add focused coverage that locks:

1. current listing result reads still derive from one current task + persisted generation-task list + persisted reviews
2. snapshot load errors still surface unchanged
3. the decorated current result still comes from one acquisition handoff

Suggested seam shape:

```go
type taskGenerationCurrentStateSnapshot struct {
	task    *Task
	result  *ListingKitResult
	tasks   []assetgeneration.Task
	reviews []GenerationReviewRecord
}

type taskGenerationCurrentStateSnapshotPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationCurrentStateSnapshotPhase(service *taskGenerationService) *taskGenerationCurrentStateSnapshotPhase

func (p *taskGenerationCurrentStateSnapshotPhase) run(ctx context.Context, taskID string) (*taskGenerationCurrentStateSnapshot, error)
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationCurrentStateSnapshot.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the current-state snapshot seam**

Create `task_generation_current_state_snapshot.go` so the seam owns:

- `repo.GetTask(...)`
- `listAssetGenerationTasks(...)`
- `listGenerationReviews(...)`
- current `Task` handoff
- decorated `ListingKitResult` handoff via `withListingKitResultGenerationAndReview(...)`

Important:

- preserve current error behavior
- do not move queue/overview/render-preview derivation into this seam
- keep it feature-local and narrow

- [ ] **Step 4: Route `getCurrentListingKitResult(...)` through the snapshot seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:342) so `getCurrentListingKitResult(...)` delegates acquisition instead of loading task/tasks/reviews inline.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationCurrentStateSnapshot.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_current_state_snapshot.go internal/listingkit/task_generation_service.go internal/listingkit/task_generation_service_test.go internal/listingkit/service_generation_retry_test.go
git commit -m "refactor: extract listingkit current generation state snapshot seam"
```

---

## Task 2: Extract current state view-derivation seam

**Files:**
- Create: `internal/listingkit/task_generation_current_state_views.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify tests if needed:
  - `internal/listingkit/task_generation_service_test.go`
  - `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Write the failing current-view tests**

Add focused coverage that locks:

1. current overview still derives from the current decorated result
2. current queue still derives from the current decorated result
3. current action render previews still derive from the current decorated result plus query

Suggested seam shape:

```go
type taskGenerationCurrentStateViewsPhase struct{}

func buildTaskGenerationCurrentStateViewsPhase() *taskGenerationCurrentStateViewsPhase

func (p *taskGenerationCurrentStateViewsPhase) queue(result *ListingKitResult) *GenerationWorkQueue
func (p *taskGenerationCurrentStateViewsPhase) overview(result *ListingKitResult) *AssetGenerationOverview
func (p *taskGenerationCurrentStateViewsPhase) renderPreviews(result *ListingKitResult, query *GenerationQueueQuery) []PlatformAssetRenderPreviews
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationCurrentStateViews.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the current state view seam**

Create `task_generation_current_state_views.go` so the seam owns:

- current queue derivation from result
- current overview derivation from result
- action-side render-preview derivation via `buildActionPlatformRenderPreviews(...)`

Important:

- preserve nil-safe behavior
- do not absorb task/result acquisition back into this seam
- keep render-preview derivation feature-local

- [ ] **Step 4: Route current-state helpers through the new seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:318) so:

- `getCurrentAssetGenerationOverview(...)`
- `getCurrentAssetGenerationQueue(...)`
- `getCurrentActionRenderPreviews(...)`

become thin seam handoffs.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationCurrentStateViews.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_current_state_views.go internal/listingkit/task_generation_service.go internal/listingkit/task_generation_service_test.go internal/listingkit/service_generation_retry_test.go
git commit -m "refactor: extract listingkit current generation state view seam"
```

---

## Task 3: Lock generation-state ownership guardrails

**Files:**
- Create: `internal/listingkit/phase15_generation_state_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/task_generation_service_test.go`
- Verify:
  - `internal/listingkit/task_generation_current_state_snapshot.go`
  - `internal/listingkit/task_generation_current_state_views.go`
  - `internal/listingkit/task_generation_service.go`

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. `getCurrentListingKitResult(...)` delegates to the current-state snapshot seam
2. snapshot seam owns task/task-list/review acquisition and result decoration, but not queue/overview/render-preview derivation
3. view seam owns queue/overview/render-preview derivation, but not task/result acquisition

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationCurrentState.*Boundary" -count=1
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

- [ ] **Step 4: Run final generation-state verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationCurrentState.*|TestTaskGenerationAction.*|TestGetTaskGenerationTasks.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase15_generation_state_boundary_test.go internal/listingkit/task_generation_current_state_snapshot.go internal/listingkit/task_generation_current_state_views.go internal/listingkit/task_generation_service.go internal/listingkit/task_generation_service_test.go internal/listingkit/service_generation_retry_test.go
git commit -m "test: lock listingkit current generation state boundaries"
```

---

## Verification Checklist For The Whole Phase

At the end of the full phase, run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationCurrentState.*|TestTaskGenerationAction.*|TestGetTaskGenerationTasks.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

If unrelated working-tree changes are still present, do **not** silently broaden this phase to fix them. Record that broader verification may still be noisy for out-of-slice reasons.
