# Task Processor Framework Phase 10A ListingKit Task Generation Action Execution Ownership Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the remaining action-flow ownership complexity in ListingKit by making action execution branching, post-action refresh, and action result projection flow through explicit feature-owned seams instead of remaining inline inside `ExecuteTaskGenerationAction(...)`.

**Architecture:** Reuse the same bounded-seam pattern already established in `Phase 9A`. Do not invent a generic action framework. Instead, split the current action block in `task_generation_service.go` into three ListingKit-owned local seams: action execution, action refresh, and action projection. Keep business behavior unchanged and preserve current retryable / queue-only / patch-only semantics.

**Tech Stack:** Go, existing ListingKit task generation service, generation review session helpers, generation review patch/workflow helpers, existing generation action handler tests, source-boundary guardrails

**Out of Scope For This Slice:**

- redesigning generation action business rules
- changing generation navigation dispatch planning semantics
- reopening retry mutation/persistence/projection seams
- inventing a repo-wide action orchestration abstraction
- moving generation action concerns into HTTP/runtime/bootstrap layers

---

## Root Cause This Slice Addresses

After `Phase 9A`, the retry path is clearer, but the action path still concentrates several different responsibilities inside one service method:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:281)

Today `ExecuteTaskGenerationAction(...)` still jointly decides:

1. whether an action is handled by temporal side-entry or local generation state
2. how queue and base result state are loaded before execution
3. how the resolved target and expected impact are derived
4. how retryable vs queue-path execution branches run
5. when persisted generation review decisions are written
6. how overview, render previews, and current result are reloaded after execution
7. how preview fallback hydration runs when current result is sparse
8. how review session, workflow, patch, and delta token are projected back to callers

The problem is not just method length. The real problem is that action ownership is still implicit and crosses three kinds of concern:

- action execution branching
- post-action refresh
- action result projection

That makes future changes risky because response-shaping and execution-side changes can leak across one block without one clear seam to test or evolve.

---

## Target Outcome

At the end of `Phase 10A`:

- action execution branching flows through an explicit ListingKit-owned seam
- post-action refresh flows through an explicit ListingKit-owned seam
- action result projection flows through an explicit ListingKit-owned seam
- `ExecuteTaskGenerationAction(...)` becomes more orchestration-focused
- current retryable / queue-only / patch-only semantics remain unchanged
- boundary tests lock the new ownership split

---

## Task 1: Extract action execution seam

**Files:**
- Create: `internal/listingkit/task_generation_action_execute.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify: `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Write the failing execution-focused tests**

Extend [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) with focused tests that lock:

1. retryable targets still execute through `RetryTaskGenerationTasks(...)`
2. non-retryable targets still read through `GetTaskGenerationQueue(...)`
3. persisted review decisions still use the execution-path-specific queue/session source

Add at least one direct seam-level test shape:

```go
func TestTaskGenerationActionExecuteRunBranchesByInteractionMode(t *testing.T) {
	t.Parallel()

	svc := &taskGenerationService{}
	target := &AssetGenerationActionTarget{
		ActionKey:       "retry_selected",
		InteractionMode: "retryable",
		RetryRequest:    &RetryGenerationTasksRequest{Slots: []string{"main"}},
	}

	result := buildTaskGenerationActionExecutePhase(svc).run(
		context.Background(),
		"task-action-exec-1",
		target,
	)

	if result.retryPage == nil {
		t.Fatalf("retry page = nil, want retry execution path")
	}
	if result.queuePage != nil {
		t.Fatalf("queue page = %+v, want nil for retry execution path", result.queuePage)
	}
}
```

- [ ] **Step 2: Run focused execution verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRunBranchesByInteractionMode$" -count=1
```

Expected: FAIL because `buildTaskGenerationActionExecutePhase(...)` does not exist yet.

- [ ] **Step 3: Add the action execution seam**

Create `internal/listingkit/task_generation_action_execute.go` with a focused local seam that owns:

- retryable vs queue-only branch execution
- execution-path-specific result capture
- persistence-session source selection

Suggested shape:

```go
type taskGenerationActionExecutePhase struct {
	service *taskGenerationService
}

type taskGenerationActionExecution struct {
	retryPage           *GenerationTaskPage
	queuePage           *GenerationQueuePage
	persistenceSession  *GenerationReviewSession
}

func buildTaskGenerationActionExecutePhase(service *taskGenerationService) *taskGenerationActionExecutePhase

func (p *taskGenerationActionExecutePhase) run(
	ctx context.Context,
	taskID string,
	target *AssetGenerationActionTarget,
) (*taskGenerationActionExecution, error)
```

Important:

- keep retryable and queue-path behavior unchanged
- keep `persistGenerationReviewDecision` in service orchestration for this slice
- do not refresh current result or render previews here

- [ ] **Step 4: Route `ExecuteTaskGenerationAction(...)` through the execution seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:281) so the inline branch block is replaced by one `buildTaskGenerationActionExecutePhase(s).run(...)` handoff.

- [ ] **Step 5: Re-run focused execution verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRunBranchesByInteractionMode$" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_action_execute.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_retry_test.go
git commit -m "refactor: extract listingkit action execution seam"
```

---

## Task 2: Extract post-action refresh seam

**Files:**
- Create: `internal/listingkit/task_generation_action_refresh.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify: `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Write the failing refresh-focused tests**

Extend [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) with focused tests that lock:

1. overview is refreshed after action execution
2. platform render previews are refreshed after action execution
3. current result fallback hydration still copies `PlatformAssetRenderPreviews` and `AssetRenderPreviews` when refreshed state is sparse

Use direct seam tests or narrow service tests; avoid full handler-level integration.

- [ ] **Step 2: Run focused refresh verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionRefresh(RehydratesOverviewAndRenderPreviews|HydratesCurrentResultFallbacks)$" -count=1
```

Expected: FAIL because no explicit refresh seam exists yet.

- [ ] **Step 3: Add the post-action refresh seam**

Create `internal/listingkit/task_generation_action_refresh.go` with a seam that owns:

- `getCurrentAssetGenerationOverview(...)`
- `getCurrentActionRenderPreviews(...)`
- `getCurrentListingKitResult(...)`
- current-result fallback hydration for render previews

Suggested shape:

```go
type taskGenerationActionRefreshPhase struct {
	service *taskGenerationService
}

type taskGenerationActionRefreshResult struct {
	overview               *AssetGenerationOverview
	platformRenderPreviews []PlatformAssetRenderPreviews
	currentResult          *ListingKitResult
}

func buildTaskGenerationActionRefreshPhase(service *taskGenerationService) *taskGenerationActionRefreshPhase

func (p *taskGenerationActionRefreshPhase) run(
	ctx context.Context,
	taskID string,
	baseResult *ListingKitResult,
	query *GenerationQueueQuery,
) (*taskGenerationActionRefreshResult, error)
```

Important:

- preserve current fallback behavior unchanged
- do not build review session, review patch, or workflow result here
- do not write persistence here

- [ ] **Step 4: Route `ExecuteTaskGenerationAction(...)` through the refresh seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:281) so the inline refresh and fallback hydration block is replaced by one refresh seam handoff.

- [ ] **Step 5: Re-run focused refresh verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionRefresh(RehydratesOverviewAndRenderPreviews|HydratesCurrentResultFallbacks)$" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_action_refresh.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_retry_test.go
git commit -m "refactor: extract listingkit action refresh seam"
```

---

## Task 3: Extract action result projection seam

**Files:**
- Create: `internal/listingkit/task_generation_action_projection.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify: `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Write the failing projection-focused tests**

Extend [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) with focused tests that lock:

1. review session still builds from refreshed current result plus execution-path-specific queue source
2. review workflow and review patch still reflect the resolved target/action key
3. patch-only responses still clear `ReviewSession` and `PlatformRenderPreviews`
4. delta token still follows existing review patch / session fallback rules

Also add a narrow source-shape test asserting `task_generation_service.go` delegates action result assembly through one local helper.

- [ ] **Step 2: Run focused projection verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionProjection(BuildsReviewSessionAndPatch|SupportsPatchOnlyResponses|ServiceDelegatesActionProjection)$" -count=1
```

Expected: FAIL because no explicit action projection seam exists yet.

- [ ] **Step 3: Add the action result projection seam**

Create `internal/listingkit/task_generation_action_projection.go` with a seam that owns:

- review-session assembly
- review-workflow result assembly
- review-patch assembly
- delta-token derivation
- patch-only response trimming

Suggested shape:

```go
type taskGenerationActionProjectionPhase struct{}

type taskGenerationActionProjectionInput struct {
	actionKey              string
	target                 *AssetGenerationActionTarget
	responseMode           string
	previousReviewSession  *GenerationReviewSession
	currentResult          *ListingKitResult
	refresh                *taskGenerationActionRefreshResult
	execution              *taskGenerationActionExecution
}

func buildTaskGenerationActionProjectionPhase() *taskGenerationActionProjectionPhase

func (p *taskGenerationActionProjectionPhase) run(
	input taskGenerationActionProjectionInput,
) *GenerationActionExecutionResult
```

Important:

- keep `applyGenerationConditionalStateToActionResult(...)` at the service call site for this slice
- do not move temporal short-circuit handling into this seam
- preserve current retryable / queue-only / patch-only semantics unchanged

- [ ] **Step 4: Route `ExecuteTaskGenerationAction(...)` through the projection seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:281) so the rebuilt `GenerationActionExecutionResult` assembly is replaced by one projection seam call plus the existing final conditional-state handoff.

- [ ] **Step 5: Re-run focused projection verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionProjection(BuildsReviewSessionAndPatch|SupportsPatchOnlyResponses|ServiceDelegatesActionProjection)$" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_action_projection.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_retry_test.go
git commit -m "refactor: extract listingkit action projection seam"
```

---

## Task 4: Lock action ownership boundaries

**Files:**
- Create: `internal/listingkit/phase10_task_generation_action_boundary_test.go`
- Modify: `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Write the boundary tests**

Create [phase10_task_generation_action_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase10_task_generation_action_boundary_test.go:1) to lock:

1. `task_generation_service.go` delegates action branching to `buildTaskGenerationActionExecutePhase(...).run(`
2. `task_generation_service.go` delegates post-action refresh to `buildTaskGenerationActionRefreshPhase(...).run(`
3. `task_generation_service.go` delegates action result assembly to `buildTaskGenerationActionProjectionPhase().run(`
4. `task_generation_service.go` no longer directly contains:
   - retryable / queue-path execution branch body
   - inline refresh/fallback hydration
   - inline review session / review patch / patch-only projection body
5. each new file owns only its intended side of the action flow

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationAction.*Boundary|TestTaskGenerationServiceFileDelegatesActionProjection" -count=1
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
git add internal/listingkit/phase10_task_generation_action_boundary_test.go internal/listingkit/service_generation_retry_test.go
git commit -m "test: lock listingkit action ownership boundaries"
```

---

## Self-Review Checklist

Before executing, verify the plan still satisfies the scope:

- it only clarifies action ownership; it does not redesign generation action business policy
- it keeps the work fully feature-owned inside `internal/listingkit`
- it does not force the navigation dispatch path to match action seams for symmetry
- it preserves retryable / queue-only / patch-only action behavior
- it adds both behavior and source-boundary protection

## Expected Outcome

When `Phase 10A` is complete:

- [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:1) will no longer be the primary shared home of action execution branching, refresh, and response projection at the same time
- action execution, refresh, and projection will each have explicit local homes
- existing action behavior will stay protected in tests
- the action path will be easier to evolve without reopening the now-stable retry seams

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-30-task-processor-framework-phase10a-action-execution-ownership.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
