# Task Processor Framework Phase 18 ListingKit Action Service-Entry Orchestration Ownership Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the remaining service-entry ownership complexity in ListingKit by making action bootstrap, persisted-review handoff, and post-projection finalization flow through explicit feature-owned seams instead of remaining clustered inline inside `ExecuteTaskGenerationAction(...)`.

**Architecture:** Reuse the same bounded-seam pattern already established in `Phase 10A`, `Phase 16`, and `Phase 17`. Do **not** invent a generic action orchestrator. Instead, split the current `ExecuteTaskGenerationAction(...)` service block in `task_generation_service.go` into explicit ListingKit-owned local seams: action entry/bootstrap, persisted-review handoff, and action finalization. Keep business behavior unchanged and preserve current temporal short-circuit / retryable / queue-only / patch-only semantics.

**Tech Stack:** Go, existing ListingKit task generation service, generation action execute/refresh/projection seams, generation review session helpers, generation conditional-state helpers, source-boundary guardrails

**Out of Scope For This Slice:**

- redesigning generation action business rules
- reopening `Phase 17` projection session/finalization seam internals
- redesigning `executeLayerTemporalAction(...)`
- changing generation navigation dispatch behavior
- inventing a repo-wide action orchestration abstraction
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 17`, the lower action seams are explicit, but the service entry still remains the shared home of several orchestration responsibilities:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:164)

Today `ExecuteTaskGenerationAction(...)` still jointly decides:

1. how current queue and base result state are bootstrapped
2. how action target, expected impact, and previous review session are derived
3. how action audit metadata is constructed
4. when persisted review decisions are durably written
5. how projection output is copied back into the outward action result
6. when conditional-state finalization is applied

The problem is not just method length. The real problem is that the service entry still owns both pre-execution setup and post-projection finalization even though execution, refresh, and projection themselves are now explicitly phased.

---

## Target Outcome

At the end of `Phase 18`:

- action bootstrap flows through an explicit ListingKit-owned seam
- persisted-review handoff flows through an explicit ListingKit-owned seam
- post-projection action finalization flows through an explicit ListingKit-owned seam
- `ExecuteTaskGenerationAction(...)` becomes more orchestration-focused
- current temporal short-circuit / retryable / queue-only / patch-only semantics remain unchanged
- boundary tests lock the new service-entry ownership split

---

## Task 1: Extract action entry/bootstrap seam

**Files:**
- Create: `internal/listingkit/task_generation_action_entry.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify tests if needed:
  - `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Write the failing bootstrap tests**

Add focused coverage that locks:

1. current queue and base result are still loaded before local action execution
2. target resolution still derives `ExpectedImpact` when absent
3. previous review session still builds from base result plus current queue
4. action audit still reflects requested key, resolved key, resolution source, and interaction mode

Suggested seam shape:

```go
type taskGenerationActionEntryPhase struct {
	service *taskGenerationService
}

type taskGenerationActionEntryResult struct {
	queue                 *GenerationQueuePage
	baseResult            *ListingKitResult
	target                *AssetGenerationActionTarget
	previousReviewSession *GenerationReviewSession
	result                *GenerationActionExecutionResult
}

func buildTaskGenerationActionEntryPhase(service *taskGenerationService) *taskGenerationActionEntryPhase

func (p *taskGenerationActionEntryPhase) run(
	ctx context.Context,
	taskID string,
	req *ExecuteGenerationActionRequest,
) (*taskGenerationActionEntryResult, error)
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionEntry.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the action entry/bootstrap seam**

Create `task_generation_action_entry.go` so the seam owns:

- `getCurrentAssetGenerationQueue(...)`
- `getCurrentListingKitResult(...)`
- overview construction from the current queue
- target resolution and `ExpectedImpact` backfill
- previous review-session assembly
- base outward `GenerationActionExecutionResult` plus audit metadata construction

Important:

- preserve current target resolution and audit semantics exactly
- do not move `executeLayerTemporalAction(...)` into this seam
- do not execute retryable / queue-only action branches here
- keep the seam feature-local and narrow

- [ ] **Step 4: Route `ExecuteTaskGenerationAction(...)` through the new entry seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:164) so the inline bootstrap block is replaced by one entry-seam handoff.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionEntry.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_action_entry.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_retry_test.go
git commit -m "refactor: extract listingkit action entry seam"
```

---

## Task 2: Extract persisted-review handoff seam

**Files:**
- Create: `internal/listingkit/task_generation_action_persist.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify tests if needed:
  - `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Write the failing persistence-handoff tests**

Add focused coverage that locks:

1. persisted review decisions still run only for persisted review action keys
2. the persistence handoff still uses `execution.persistenceSession`
3. non-persisted actions still skip the persistence seam cleanly

Suggested seam shape:

```go
type taskGenerationActionPersistPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationActionPersistPhase(service *taskGenerationService) *taskGenerationActionPersistPhase

func (p *taskGenerationActionPersistPhase) run(
	ctx context.Context,
	taskID string,
	target *AssetGenerationActionTarget,
	execution *taskGenerationActionExecution,
) error
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionPersist.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the persisted-review handoff seam**

Create `task_generation_action_persist.go` so the seam owns:

- persisted-review action eligibility check
- nil-safe use of `s.persistGenerationReviewDecision`
- persistence-session handoff into `persistGenerationReviewDecision(...)`

Important:

- preserve current persistence timing exactly: after execution, before refresh
- do not move execution or refresh concerns here
- do not build review sessions or patches here

- [ ] **Step 4: Route `ExecuteTaskGenerationAction(...)` through the persistence seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:164) so the inline persisted-review block becomes one seam handoff.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionPersist.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_action_persist.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_retry_test.go
git commit -m "refactor: extract listingkit action persistence seam"
```

---

## Task 3: Extract post-projection finalization seam

**Files:**
- Create: `internal/listingkit/task_generation_action_finalize.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify tests if needed:
  - `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Write the failing finalization tests**

Add focused coverage that locks:

1. projection output still copies back `Overview`, `Queue`, `Retry`, `ReviewWorkflow`, `ReviewSession`, `ReviewPatch`, `PlatformRenderPreviews`, and `DeltaToken`
2. conditional-state finalization still happens after projection copy-back
3. outward result still preserves the entry-phase `ActionKey`, `ResolvedTarget`, `InteractionMode`, `ResponseMode`, and `Audit`

Suggested seam shape:

```go
type taskGenerationActionFinalizePhase struct{}

func buildTaskGenerationActionFinalizePhase() *taskGenerationActionFinalizePhase

func (p *taskGenerationActionFinalizePhase) run(
	result *GenerationActionExecutionResult,
	projection *GenerationActionExecutionResult,
) *GenerationActionExecutionResult
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionFinalize.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the post-projection finalization seam**

Create `task_generation_action_finalize.go` so the seam owns:

- projection field copy-back into the outward result
- final `applyGenerationConditionalStateToActionResult(...)` handoff

Important:

- preserve copy-back field set and order exactly
- do not rebuild queue/result bootstrap here
- do not reopen projection internals here

- [ ] **Step 4: Route `ExecuteTaskGenerationAction(...)` through the finalization seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:164) so the inline projection copy-back and conditional-state finalization block is replaced by one seam handoff.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionFinalize.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_action_finalize.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_retry_test.go
git commit -m "refactor: extract listingkit action finalization seam"
```

---

## Task 4: Lock service-entry orchestration guardrails

**Files:**
- Create: `internal/listingkit/phase18_action_service_entry_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/phase10_task_generation_action_boundary_test.go`
  - `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. `ExecuteTaskGenerationAction(...)` still checks `executeLayerTemporalAction(...)` first
2. local action path then delegates in order: entry seam, execute seam, persist seam, refresh seam, projection seam, finalize seam
3. `task_generation_service.go` no longer directly owns queue/result bootstrap, target resolution, persisted-review handoff, or projection copy-back
4. each new file owns only its intended side of the orchestration

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationAction(ServiceEntry|PhaseOwnership).*Boundary" -count=1
```

Expected: FAIL until the guardrails reflect the final seam split.

- [ ] **Step 3: Keep the guardrails low-fragility**

Anchor the ownership tests on:

- helper names
- ordered delegation
- explicit forbidden helper calls
- responsibility-level file signals

Avoid fragile dependence on:

- local variable names
- exact conditional layout
- whitespace-sensitive snippets

- [ ] **Step 4: Run final phase verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationAction.*|TestTaskGenerationActionProjection.*|TestTaskGenerationActionRefresh.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase18_action_service_entry_boundary_test.go internal/listingkit/phase10_task_generation_action_boundary_test.go internal/listingkit/service_generation_retry_test.go internal/listingkit/task_generation_service.go
git commit -m "test: lock listingkit action service-entry boundaries"
```

---

## Verification Checklist For The Whole Phase

At the end of the full phase, run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationAction.*|TestTaskGenerationActionProjection.*|TestTaskGenerationActionRefresh.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

If unrelated working-tree changes are still present, do **not** silently broaden this phase to fix them. Record that broader verification may still be noisy for out-of-slice reasons.

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-31-task-processor-framework-phase18-action-service-entry-orchestration.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
