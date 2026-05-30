# Task Processor Framework Phase 17 ListingKit Action Projection Finalization Ownership Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the remaining action-response ownership complexity in ListingKit by making review queue selection/session assembly and projection finalization flow through explicit feature-owned seams instead of remaining clustered inside `taskGenerationActionProjectionPhase.run(...)`.

**Architecture:** Reuse the same bounded-seam pattern already established in `Phase 10A`, `Phase 15`, and `Phase 16`. Do **not** invent a generic response-finalization framework. Instead, split the current projection block in `task_generation_action_projection.go` into explicit ListingKit-owned local seams: review-session assembly and projection finalization. Keep business behavior unchanged and preserve current `patch_only`, workflow, patch, and delta-token semantics.

**Tech Stack:** Go, existing ListingKit action flow, review session/workflow helpers, refresh seams, source-boundary guardrails

**Out of Scope For This Slice:**

- redesigning action business rules
- reopening action execution seams from `Phase 10A`
- reopening current-state/refresh seams from `Phase 15/16`
- redesigning temporal branching
- inventing a repo-wide projection abstraction
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 16`, execution and refresh are explicit, but [internal/listingkit/task_generation_action_projection.go](/D:/code/task-processor/internal/listingkit/task_generation_action_projection.go:19) still carries another mixed-responsibility block.

Today `taskGenerationActionProjectionPhase.run(...)` still jointly decides:

1. how retry vs queue execution results are surfaced
2. how refreshed overview/render previews are attached
3. how the review queue is selected for session assembly
4. how review session and workflow results are built
5. how workflow results are applied into the session
6. how patch generation and delta-token finalization work
7. how `patch_only` response shaping is enforced

The problem is not only method size. The real problem is that response assembly, review session shaping, and finalization policy are still implicit and live in one block, so future action-response changes can leak across it without one clear seam to evolve or protect.

---

## Target Outcome

At the end of `Phase 17`:

- review queue selection and review-session assembly flow through an explicit ListingKit-owned seam
- workflow/patch/delta-token finalization flows through an explicit ListingKit-owned seam
- `taskGenerationActionProjectionPhase.run(...)` becomes more orchestration-focused
- current `patch_only`, workflow, patch, and delta-token behavior remains unchanged
- boundary tests lock the new ownership split

---

## Task 1: Extract action projection session-assembly seam

**Files:**
- Create: `internal/listingkit/task_generation_action_projection_session.go`
- Modify: `internal/listingkit/task_generation_action_projection.go`
- Modify tests if needed:
  - `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Write the failing session-assembly tests**

Add focused coverage that locks:

1. review queue selection still uses retry queue for `retryable` interaction mode and queue page for non-retryable modes
2. review session still builds from the selected queue plus current/refreshed result
3. refreshed current result still wins over base current result when available

Suggested seam shape:

```go
type taskGenerationActionProjectionSessionPhase struct{}

type taskGenerationActionProjectionSessionResult struct {
	currentResult  *ListingKitResult
	reviewQueue    *GenerationWorkQueue
	reviewSession  *GenerationReviewSession
}

func buildTaskGenerationActionProjectionSessionPhase() *taskGenerationActionProjectionSessionPhase

func (p *taskGenerationActionProjectionSessionPhase) run(
	input *taskGenerationActionProjectionInput,
) *taskGenerationActionProjectionSessionResult
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionProjectionSession.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the session-assembly seam**

Create `task_generation_action_projection_session.go` so the seam owns:

- current-result selection between base and refreshed result
- review queue selection
- review-session assembly via `buildGenerationReviewSession(...)`

Important:

- preserve current queue-selection behavior exactly
- do not finalize workflow/patch/delta-token here
- keep it feature-local and narrow

- [ ] **Step 4: Route projection through the new session seam**

Update [task_generation_action_projection.go](/D:/code/task-processor/internal/listingkit/task_generation_action_projection.go:19) so session assembly becomes a single seam handoff.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionProjectionSession.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_action_projection_session.go internal/listingkit/task_generation_action_projection.go internal/listingkit/service_generation_retry_test.go
git commit -m "refactor: extract listingkit action projection session seam"
```

---

## Task 2: Extract action projection finalization seam

**Files:**
- Create: `internal/listingkit/task_generation_action_projection_finalize.go`
- Modify: `internal/listingkit/task_generation_action_projection.go`
- Modify tests if needed:
  - `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Write the failing finalization tests**

Add focused coverage that locks:

1. workflow result is still derived from action key + target
2. workflow still applies into the review session before patch generation
3. patch/delta-token finalization still follows existing priority
4. `patch_only` still strips `ReviewSession` and `PlatformRenderPreviews`

Suggested seam shape:

```go
type taskGenerationActionProjectionFinalizePhase struct{}

func buildTaskGenerationActionProjectionFinalizePhase() *taskGenerationActionProjectionFinalizePhase

func (p *taskGenerationActionProjectionFinalizePhase) run(
	input *taskGenerationActionProjectionInput,
	result *GenerationActionExecutionResult,
	session *taskGenerationActionProjectionSessionResult,
) *GenerationActionExecutionResult
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionProjectionFinalize.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the finalization seam**

Create `task_generation_action_projection_finalize.go` so the seam owns:

- workflow result construction
- workflow application into review session
- patch generation
- delta-token finalization
- `patch_only` shaping

Important:

- preserve current finalization order exactly
- do not rebuild review queue/session selection here
- do not move execution/refresh concerns here

- [ ] **Step 4: Route projection through the finalization seam**

Update [task_generation_action_projection.go](/D:/code/task-processor/internal/listingkit/task_generation_action_projection.go:19) so finalization becomes a single seam handoff after session assembly.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionProjectionFinalize.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_action_projection_finalize.go internal/listingkit/task_generation_action_projection.go internal/listingkit/service_generation_retry_test.go
git commit -m "refactor: extract listingkit action projection finalization seam"
```

---

## Task 3: Lock action projection ownership guardrails

**Files:**
- Create: `internal/listingkit/phase17_action_projection_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/service_generation_retry_test.go`
- Verify:
  - `internal/listingkit/task_generation_action_projection.go`
  - `internal/listingkit/task_generation_action_projection_session.go`
  - `internal/listingkit/task_generation_action_projection_finalize.go`

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. `taskGenerationActionProjectionPhase.run(...)` delegates session assembly then finalization in order
2. session seam owns current-result/review-queue/review-session assembly, but not workflow/patch/delta-token finalization
3. finalization seam owns workflow/patch/delta-token finalization and `patch_only` shaping, but not queue/session selection

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionProjection.*Boundary" -count=1
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

- [ ] **Step 4: Run final action projection verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionProjection.*|TestTaskGenerationActionRefresh.*|TestTaskGenerationAction.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase17_action_projection_boundary_test.go internal/listingkit/task_generation_action_projection.go internal/listingkit/task_generation_action_projection_session.go internal/listingkit/task_generation_action_projection_finalize.go internal/listingkit/service_generation_retry_test.go
git commit -m "test: lock listingkit action projection boundaries"
```

---

## Verification Checklist For The Whole Phase

At the end of the full phase, run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionProjection.*|TestTaskGenerationActionRefresh.*|TestTaskGenerationAction.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

If unrelated working-tree changes are still present, do **not** silently broaden this phase to fix them. Record that broader verification may still be noisy for out-of-slice reasons.
