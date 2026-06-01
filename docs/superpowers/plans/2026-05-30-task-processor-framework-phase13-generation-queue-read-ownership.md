# Task Processor Framework Phase 13 ListingKit Generation Queue Read Ownership Plan

## Goal

Reduce the remaining queue-read ownership complexity in ListingKit by making queue snapshot acquisition, queue page shaping, and final queue response decoration flow through explicit feature-owned seams instead of remaining clustered inside `GetTaskGenerationQueue(...)`.

## Architecture

Reuse the same bounded-seam pattern already established in `Phase 12`. Do **not** invent a generic list-read or query framework. Instead, split the current queue-read path in [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:75) into three ListingKit-owned local seams:

1. shared queue snapshot acquisition
2. queue page assembly and shaping
3. queue response conditional/final decoration

Keep business behavior unchanged and preserve current filtering, sorting, paging, review-summary attachment, `NotModified`, and conditional-response semantics.

## Scope

This phase is limited to the ListingKit generation queue read path.

Primary hotspots:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:75)
- [internal/listingkit/generation_queue_list.go](/D:/code/task-processor/internal/listingkit/generation_queue_list.go:1)
- [internal/listingkit/generation_conditional_state.go](/D:/code/task-processor/internal/listingkit/generation_conditional_state.go:1)
- [internal/listingkit/service_generation_queue_test.go](/D:/code/task-processor/internal/listingkit/service_generation_queue_test.go:1)

Allowed outcomes:

- new feature-local helper/seam files under `internal/listingkit/`
- tighter service orchestration in queue read
- behavior tests and source-boundary guardrails

Not in scope:

- review session / preview seam changes
- navigation/action/retry/submit-path changes
- studio-session worktree changes
- generic paging/filtering/query abstractions
- HTTP/bootstrap/runtime changes

## Working Rules

1. Preserve queue behavior first; ownership clarity second.
2. Keep all new abstractions feature-local to ListingKit.
3. Prefer seams that reflect ownership pressure, not mechanical helper splitting.
4. Keep this phase isolated from the active `studio session` area in the worktree.
5. Add or update tests before the implementation that depends on them.

## Task Breakdown

This phase should land as four small commits:

1. `refactor: extract listingkit queue read snapshot seam`
2. `refactor: extract listingkit queue read page seam`
3. `refactor: extract listingkit queue read response seam`
4. `test: lock listingkit queue read boundaries`

Keep them in that order so each ownership split stays reviewable.

---

## Task 1: Extract queue read snapshot seam

**Files:**
- Create: `internal/listingkit/task_generation_queue_read_snapshot.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify tests if needed: `internal/listingkit/service_generation_queue_test.go`

- [ ] **Step 1: Write the failing queue-snapshot tests**

Add focused coverage that locks:

1. queue reads still derive from one current task/result/review snapshot
2. queue snapshot errors still surface unchanged
3. review-decorated result and queue come from the same acquisition handoff

Suggested seam shape:

```go
type taskGenerationQueueReadSnapshot struct {
	task *Task
	result *ListingKitResult
	queue *GenerationWorkQueue
}

type taskGenerationQueueReadSnapshotPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationQueueReadSnapshotPhase(service *taskGenerationService) *taskGenerationQueueReadSnapshotPhase

func (p *taskGenerationQueueReadSnapshotPhase) run(ctx context.Context, taskID string) (*taskGenerationQueueReadSnapshot, error)
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationQueueReadSnapshot.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the snapshot seam**

Create `task_generation_queue_read_snapshot.go` so the seam owns:

- `repo.GetTask(...)`
- `listAssetGenerationTasks(...)`
- `listGenerationReviews(...)`
- `withListingKitResultGenerationAndReview(...)`
- queue extraction from the decorated result

Important:

- preserve current error behavior
- do not move filtering/sorting/paging into this seam
- keep it feature-local and narrow

- [ ] **Step 4: Route queue service method through the snapshot seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:75) so `GetTaskGenerationQueue(...)` delegates snapshot acquisition instead of loading task/result/reviews inline.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationQueueReadSnapshot.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_queue_read_snapshot.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_queue_test.go
git commit -m "refactor: extract listingkit queue read snapshot seam"
```

---

## Task 2: Extract queue page shaping seam

**Files:**
- Create: `internal/listingkit/task_generation_queue_read_page.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify tests if needed: `internal/listingkit/service_generation_queue_test.go`

- [ ] **Step 1: Write the failing queue-page tests**

Add focused coverage that locks:

1. queue-empty response shape stays unchanged
2. filtering / sorting / paging still behave the same
3. review summary is still attached to the page before response finalization

Suggested seam shape:

```go
type taskGenerationQueueReadPagePhase struct{}

func buildTaskGenerationQueueReadPagePhase() *taskGenerationQueueReadPagePhase

func (p *taskGenerationQueueReadPagePhase) run(
	snapshot *taskGenerationQueueReadSnapshot,
	query *GenerationQueueQuery,
) *GenerationQueuePage
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationQueueReadPage.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the queue-page seam**

Create `task_generation_queue_read_page.go` so the seam owns:

- empty queue response shaping
- `filterGenerationQueueItems(...)`
- `sortGenerationQueueItems(...)`
- `paginateGenerationQueueItems(...)`
- `buildGenerationQueuePage(...)`
- `attachReviewSummaryToGenerationQueuePage(...)`

Important:

- preserve current page / pageSize defaults
- preserve summary shaping
- do not absorb conditional-read short-circuit or final conditional decoration

- [ ] **Step 4: Route queue service method through the new seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:75) so queue page shaping becomes a single seam handoff.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationQueueReadPage.*|TestGetTaskGenerationQueue.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_queue_read_page.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_queue_test.go
git commit -m "refactor: extract listingkit queue read page seam"
```

---

## Task 3: Extract queue response/finalization seam

**Files:**
- Create: `internal/listingkit/task_generation_queue_read_response.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify tests if needed: `internal/listingkit/service_generation_queue_test.go`

- [ ] **Step 1: Write the failing queue-response tests**

Add focused coverage that locks:

1. delta-token behavior remains unchanged
2. `NotModified` still short-circuits before final payload usage
3. final `applyGenerationConditionalStateToQueuePage(...)` still happens

Suggested seam shape:

```go
type taskGenerationQueueReadResponsePhase struct{}

func buildTaskGenerationQueueReadResponsePhase() *taskGenerationQueueReadResponsePhase

func (p *taskGenerationQueueReadResponsePhase) run(
	taskID string,
	page *GenerationQueuePage,
	query *GenerationQueueQuery,
) *GenerationQueuePage
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationQueueReadResponse.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the queue-response seam**

Create `task_generation_queue_read_response.go` so the seam owns:

- `buildGenerationQueueDeltaToken(...)`
- `isGenerationReviewReadNotModified(...)`
- `applyGenerationConditionalStateToQueuePage(...)`
- final not-modified queue response shape

Important:

- preserve current `NotModified` payload shape
- preserve current conditional/recovery/action summary decoration
- do not absorb queue snapshot or queue page shaping back into this seam

- [ ] **Step 4: Route queue service method through the new seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:75) so queue response finalization becomes a single seam handoff.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationQueueReadResponse.*|TestGetTaskGenerationQueue.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_queue_read_response.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_queue_test.go
git commit -m "refactor: extract listingkit queue read response seam"
```

---

## Task 4: Lock queue-read ownership guardrails

**Files:**
- Create: `internal/listingkit/phase13_generation_queue_read_boundary_test.go`
- Modify if needed: `internal/listingkit/service_generation_queue_test.go`
- Verify:
  - `internal/listingkit/task_generation_queue_read_snapshot.go`
  - `internal/listingkit/task_generation_queue_read_page.go`
  - `internal/listingkit/task_generation_queue_read_response.go`
  - `internal/listingkit/task_generation_service.go`

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. `GetTaskGenerationQueue(...)` delegates to snapshot/page/response seams
2. snapshot seam owns task/result/review acquisition, but not queue shaping or final response decoration
3. page seam owns queue shaping and review-summary attachment, but not conditional-read short-circuit
4. response seam owns delta-token / `NotModified` / final conditional decoration, but not snapshot acquisition or filtering/sorting/paging

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationQueueRead.*Boundary" -count=1
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

- [ ] **Step 4: Run final queue-read verification**

Run:

```powershell
go test ./internal/listingkit -run "TestGetTaskGenerationQueue.*|TestTaskGenerationQueueRead.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase13_generation_queue_read_boundary_test.go internal/listingkit/service_generation_queue_test.go internal/listingkit/task_generation_queue_read_snapshot.go internal/listingkit/task_generation_queue_read_page.go internal/listingkit/task_generation_queue_read_response.go internal/listingkit/task_generation_service.go
git commit -m "test: lock listingkit queue read boundaries"
```

---

## Verification Checklist For The Whole Phase

At the end of the full phase, run:

```powershell
go test ./internal/listingkit -run "TestGetTaskGenerationQueue.*|TestTaskGenerationQueueRead.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

If unrelated working-tree changes are still present, do **not** silently broaden this phase to fix them. Record that broader verification may still be noisy for out-of-slice reasons.
