# Task Processor Framework Phase 12 ListingKit Generation Review Read Ownership Plan

## Goal

Reduce the remaining review-read ownership complexity in ListingKit by making review snapshot acquisition, session response assembly, and preview response assembly flow through explicit feature-owned seams instead of remaining clustered inside `GetTaskGenerationReviewSession(...)` and `GetTaskGenerationReviewPreview(...)`.

## Architecture

Reuse the same bounded-seam pattern already established in `Phase 10B` and `Phase 11`. Do **not** invent a generic read/query framework. Instead, split the current review read path in [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:1) into three ListingKit-owned local seams:

1. shared review snapshot acquisition
2. review session read response shaping
3. review preview read response shaping

Keep business behavior unchanged and preserve current `patch_only`, `NotModified`, preview projection, revision-status, and conditional-state semantics.

## Scope

This phase is limited to the ListingKit generation review read path.

Primary hotspots:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:130)
- [internal/listingkit/generation_review_session.go](/D:/code/task-processor/internal/listingkit/generation_review_session.go:1)
- [internal/listingkit/generation_review_patch.go](/D:/code/task-processor/internal/listingkit/generation_review_patch.go:1)
- [internal/listingkit/generation_conditional_state.go](/D:/code/task-processor/internal/listingkit/generation_conditional_state.go:1)

Allowed outcomes:

- new feature-local helper/seam files under `internal/listingkit/`
- tighter service orchestration in review read methods
- behavior tests and source-boundary guardrails

Not in scope:

- queue-read refactoring as a first-class slice
- navigation entry/primary/projection refactoring
- navigation plan-engine refactoring
- action-path or retry-path changes
- generic read/query abstractions
- HTTP/bootstrap/runtime changes

## Working Rules

1. Preserve current behavior first; ownership clarity second.
2. Keep all new abstractions feature-local to ListingKit.
3. Prefer one seam per responsibility hotspot, not one seam per DTO field group.
4. When a helper is shared only by session and preview reads, keep it local to this phase instead of promoting it early.
5. Add or update tests before the implementation that depends on them.

## Task Breakdown

This phase should land as four small commits:

1. `refactor: extract listingkit review read snapshot seam`
2. `refactor: extract listingkit review session read seam`
3. `refactor: extract listingkit review preview read seam`
4. `test: lock listingkit review read boundaries`

Keep them in that order so each ownership split stays reviewable.

---

## Task 1: Extract shared review snapshot seam

**Files:**
- Create: `internal/listingkit/task_generation_review_read_snapshot.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify tests if needed: `internal/listingkit/service_generation_queue_test.go`

- [ ] **Step 1: Write the failing shared-snapshot tests**

Add focused coverage that locks:

1. session and preview reads still derive from the same current result + queue snapshot
2. missing session snapshot still returns the same empty response shape as today
3. snapshot load errors still surface unchanged

Suggested seam shape:

```go
type taskGenerationReviewReadSnapshot struct {
	taskID string
	result *ListingKitResult
	queue  *GenerationWorkQueue
}

type taskGenerationReviewReadSnapshotPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationReviewReadSnapshotPhase(service *taskGenerationService) *taskGenerationReviewReadSnapshotPhase

func (p *taskGenerationReviewReadSnapshotPhase) run(ctx context.Context, taskID string) (*taskGenerationReviewReadSnapshot, error)
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationReviewReadSnapshot.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the snapshot seam**

Create `task_generation_review_read_snapshot.go` so the seam owns:

- `getCurrentListingKitResult(...)`
- `getCurrentAssetGenerationQueue(...)`
- shared snapshot handoff for session/preview reads

Important:

- preserve current error behavior
- do not move queue-read logic into this seam
- keep it feature-local and narrow

- [ ] **Step 4: Route service methods through the snapshot seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:130) so both review read methods delegate snapshot acquisition instead of loading result/queue inline.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationReviewReadSnapshot.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_review_read_snapshot.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_queue_test.go
git commit -m "refactor: extract listingkit review read snapshot seam"
```

---

## Task 2: Extract review session read seam

**Files:**
- Create: `internal/listingkit/task_generation_review_session_read.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify tests if needed: `internal/listingkit/service_generation_queue_test.go`

- [ ] **Step 1: Write the failing session-read tests**

Add focused coverage that locks:

1. `patch_only` still returns patch payload instead of full session
2. `ResponseMode` still normalizes the same way
3. `NotModified` still short-circuits before payload shaping
4. delta-token behavior remains unchanged

Suggested seam shape:

```go
type taskGenerationReviewSessionReadPhase struct{}

func buildTaskGenerationReviewSessionReadPhase() *taskGenerationReviewSessionReadPhase

func (p *taskGenerationReviewSessionReadPhase) run(
	taskID string,
	snapshot *taskGenerationReviewReadSnapshot,
	query *GenerationQueueQuery,
) *GenerationReviewSessionResponse
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationReviewSessionRead.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the session-read seam**

Create `task_generation_review_session_read.go` so the seam owns:

- `buildGenerationReviewSession(...)`
- `buildGenerationReviewReadDeltaToken(...)`
- `patch_only` shaping
- `NotModified` short-circuit
- final `applyGenerationConditionalStateToReviewSessionResponse(...)`

Important:

- preserve base-session patch construction
- preserve empty-response behavior when session is nil
- do not absorb preview-specific logic

- [ ] **Step 4: Route session service method through the new seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:130) so `GetTaskGenerationReviewSession(...)` becomes a thin orchestration wrapper.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationReviewSessionRead.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_review_session_read.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_queue_test.go
git commit -m "refactor: extract listingkit review session read seam"
```

---

## Task 3: Extract review preview read seam

**Files:**
- Create: `internal/listingkit/task_generation_review_preview_read.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify tests if needed: `internal/listingkit/service_generation_queue_test.go`

- [ ] **Step 1: Write the failing preview-read tests**

Add focused coverage that locks:

1. preview reads still use the same session snapshot baseline
2. `NotModified` still short-circuits before preview projection
3. viewer / preview / target / toolbar / revision-status shaping still matches current behavior
4. final conditional-state decoration still happens

Suggested seam shape:

```go
type taskGenerationReviewPreviewReadPhase struct{}

func buildTaskGenerationReviewPreviewReadPhase() *taskGenerationReviewPreviewReadPhase

func (p *taskGenerationReviewPreviewReadPhase) run(
	taskID string,
	snapshot *taskGenerationReviewReadSnapshot,
	query *GenerationQueueQuery,
) *GenerationReviewPreviewResponse
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationReviewPreviewRead.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the preview-read seam**

Create `task_generation_review_preview_read.go` so the seam owns:

- `buildGenerationReviewSession(...)`
- `buildGenerationReviewReadDeltaToken(...)`
- preview projection via `resolveGenerationReviewPreviewResponse(...)`
- revision status shaping via `resolveGenerationReviewPreviewRevisionStatus(...)`
- final `applyGenerationConditionalStateToReviewPreviewResponse(...)`

Important:

- preserve empty-response behavior when session is nil
- preserve preview-scene preset shaping
- do not absorb session `patch_only` logic

- [ ] **Step 4: Route preview service method through the new seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:173) so `GetTaskGenerationReviewPreview(...)` becomes a thin orchestration wrapper.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationReviewPreviewRead.*|TestTaskGenerationReviewSessionRead.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_review_preview_read.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_queue_test.go
git commit -m "refactor: extract listingkit review preview read seam"
```

---

## Task 4: Lock review-read ownership guardrails

**Files:**
- Create: `internal/listingkit/phase12_generation_review_read_boundary_test.go`
- Modify if needed: `internal/listingkit/service_generation_queue_test.go`
- Verify:
  - `internal/listingkit/task_generation_review_read_snapshot.go`
  - `internal/listingkit/task_generation_review_session_read.go`
  - `internal/listingkit/task_generation_review_preview_read.go`
  - `internal/listingkit/task_generation_service.go`

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. service review-read methods delegate to snapshot/session/preview seams
2. snapshot seam owns current result + queue acquisition, but not response shaping
3. session seam owns `patch_only` / `NotModified` / session conditional shaping, but not preview projection
4. preview seam owns preview projection / revision-status / preview conditional shaping, but not snapshot acquisition

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationReview(Read|Preview|Session).*Boundary" -count=1
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

- [ ] **Step 4: Run final review-read verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationReview.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase12_generation_review_read_boundary_test.go internal/listingkit/service_generation_queue_test.go internal/listingkit/task_generation_review_read_snapshot.go internal/listingkit/task_generation_review_session_read.go internal/listingkit/task_generation_review_preview_read.go internal/listingkit/task_generation_service.go
git commit -m "test: lock listingkit review read boundaries"
```

---

## Verification Checklist For The Whole Phase

At the end of the full phase, run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationReview.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

If unrelated working-tree changes are still present, do **not** silently broaden this phase to fix them. Record that broader verification may still be noisy for out-of-slice reasons.
