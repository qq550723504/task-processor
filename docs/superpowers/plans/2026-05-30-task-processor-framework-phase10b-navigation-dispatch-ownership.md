# Task Processor Framework Phase 10B ListingKit Task Generation Navigation Dispatch Ownership Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the remaining navigation-dispatch ownership complexity in ListingKit by making navigation entry normalization, primary dispatch routing, and dispatch response shaping flow through explicit feature-owned seams instead of remaining mixed together inside `DispatchTaskGenerationNavigation(...)`.

**Architecture:** Reuse the same bounded-seam pattern already established in `Phase 10A`. Do not invent a generic dispatch framework. Instead, split the current navigation block in `task_generation_service.go` into three ListingKit-owned local seams: navigation entry orchestration, primary dispatch routing, and dispatch projection/finalization. Keep business behavior unchanged and preserve current action / preview / queue / session semantics, including optional plan execution.

**Tech Stack:** Go, existing ListingKit task generation service, generation navigation dispatch helpers, existing navigation dispatch tests, source-boundary guardrails

**Out of Scope For This Slice:**

- redesigning generation navigation business rules
- changing dispatch-plan deduplication or parallelism semantics
- reopening `Phase 10A` action execution seams
- folding in submit-path stabilization work
- inventing a repo-wide navigation or dispatch orchestration abstraction

---

## Root Cause This Slice Addresses

After `Phase 10A`, the action path is clearer, but the navigation path still concentrates several different responsibilities inside one service method:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:350)

Today `DispatchTaskGenerationNavigation(...)` still jointly decides:

1. how the navigation target is cloned and conditional baseline is applied
2. how `response_mode` and `plan_mode` are normalized
3. how action / preview / queue / session primary routing is selected
4. when optional dispatch-plan execution should run
5. how executed-plan results are merged back into the primary response
6. how the final dispatch response is normalized before return

The problem is not just method length. The real problem is that navigation ownership is still implicit and crosses three kinds of concern:

- navigation entry orchestration
- primary dispatch routing
- dispatch response shaping

That makes future changes risky because routing changes and response-model changes can still leak across one block without one clear seam to test or evolve.

---

## Target Outcome

At the end of `Phase 10B`:

- navigation entry orchestration flows through an explicit ListingKit-owned seam
- primary dispatch routing flows through an explicit ListingKit-owned seam
- dispatch response merge/finalization flows through an explicit ListingKit-owned seam
- `DispatchTaskGenerationNavigation(...)` becomes more orchestration-focused
- current action / preview / queue / session semantics remain unchanged
- optional plan execution behavior remains unchanged
- boundary tests lock the new ownership split

---

## Task 1: Extract navigation entry seam

**Files:**
- Create: `internal/listingkit/task_generation_navigation_dispatch_entry.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify: `internal/listingkit/service_generation_navigation_dispatch_test.go`

- [ ] **Step 1: Write the failing entry-focused tests**

Extend [service_generation_navigation_dispatch_test.go](/D:/code/task-processor/internal/listingkit/service_generation_navigation_dispatch_test.go:1) with focused coverage that locks:

1. nil request / nil target still returns `ErrGenerationActionNotFound`
2. target clone and conditional baseline setup still happen before primary dispatch
3. plan mode still defaults to `resolve_only` and only triggers plan execution when `execute_plan`

Add at least one direct seam-level test shape:

```go
func TestTaskGenerationNavigationDispatchEntryRunNormalizesTargetAndPlanMode(t *testing.T) {
	t.Parallel()

	target := &GenerationReviewNavigationTarget{
		DispatchKind: "session",
		Conditional:  &GenerationConditionalState{DeltaToken: "delta-body"},
	}

	entry, err := buildTaskGenerationNavigationDispatchEntry().run(
		&GenerationReviewNavigationDispatchRequest{
			PlanMode: " execute_plan ",
			Target:   target,
		},
	)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if entry == nil || entry.target == nil {
		t.Fatalf("entry = %+v, want normalized target", entry)
	}
	if entry.planMode != "execute_plan" {
		t.Fatalf("plan mode = %q, want execute_plan", entry.planMode)
	}
	if entry.target == target {
		t.Fatalf("target should be cloned before orchestration")
	}
}
```

- [ ] **Step 2: Run focused entry verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchEntryRunNormalizesTargetAndPlanMode$" -count=1
```

Expected: FAIL because `buildTaskGenerationNavigationDispatchEntry()` does not exist yet.

- [ ] **Step 3: Add the navigation entry seam**

Create `internal/listingkit/task_generation_navigation_dispatch_entry.go` with a focused local seam that owns:

- request/target nil validation
- target clone
- `ApplyGenerationConditionalBaselineToNavigationTarget(...)`
- `responseMode` / `planMode` normalization

Suggested shape:

```go
type taskGenerationNavigationDispatchEntry struct{}

type taskGenerationNavigationDispatchInput struct {
	target       *GenerationReviewNavigationTarget
	responseMode string
	planMode     string
}

func buildTaskGenerationNavigationDispatchEntry() *taskGenerationNavigationDispatchEntry

func (e *taskGenerationNavigationDispatchEntry) run(
	req *GenerationReviewNavigationDispatchRequest,
) (*taskGenerationNavigationDispatchInput, error)
```

Important:

- keep current error behavior unchanged
- keep conditional baseline application here
- do not route action / preview / queue / session dispatch here

- [ ] **Step 4: Route `DispatchTaskGenerationNavigation(...)` through the entry seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:350) so the inline request/target normalization block is replaced by one `buildTaskGenerationNavigationDispatchEntry().run(...)` handoff.

- [ ] **Step 5: Re-run focused entry verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchEntryRunNormalizesTargetAndPlanMode$" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_navigation_dispatch_entry.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_navigation_dispatch_test.go
git commit -m "refactor: extract listingkit navigation entry seam"
```

---

## Task 2: Extract primary dispatch seam

**Files:**
- Create: `internal/listingkit/task_generation_navigation_dispatch_primary.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify: `internal/listingkit/service_generation_navigation_dispatch_test.go`

- [ ] **Step 1: Write the failing primary-dispatch tests**

Extend [service_generation_navigation_dispatch_test.go](/D:/code/task-processor/internal/listingkit/service_generation_navigation_dispatch_test.go:1) with focused coverage that locks:

1. `action` targets still delegate through `ExecuteTaskGenerationAction(...)`
2. `preview` targets still delegate through `GetTaskGenerationReviewPreview(...)`
3. `queue` targets still delegate through `GetTaskGenerationQueue(...)`
4. session fallback still resolves query precedence as `SessionQuery -> QueueQuery -> PreviewQuery`

Add at least one direct seam-level test shape:

```go
func TestTaskGenerationNavigationPrimaryRunRoutesDispatchKinds(t *testing.T) {
	t.Parallel()

	svc := newTaskGenerationNavigationDispatchTestService(t)
	target := &GenerationReviewNavigationTarget{
		DispatchKind: "queue",
		QueueQuery:   &GenerationQueueQuery{Platform: "shein"},
	}

	response, err := buildTaskGenerationNavigationDispatchPrimaryPhase(svc).run(
		context.Background(),
		"task-navigation-primary-1",
		target,
		"patch_only",
	)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if response == nil || response.DispatchKind != "queue" {
		t.Fatalf("response = %+v, want queue dispatch response", response)
	}
}
```

- [ ] **Step 2: Run focused primary-dispatch verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigationPrimaryRunRoutesDispatchKinds$" -count=1
```

Expected: FAIL because `buildTaskGenerationNavigationDispatchPrimaryPhase(...)` does not exist yet.

- [ ] **Step 3: Add the primary dispatch seam**

Create `internal/listingkit/task_generation_navigation_dispatch_primary.go` with a seam that owns:

- dispatch-kind routing
- request shaping for action / preview / queue / session dispatch
- session-query precedence resolution
- primary `GenerationReviewNavigationDispatchResponse` assembly

Suggested shape:

```go
type taskGenerationNavigationDispatchPrimaryPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationNavigationDispatchPrimaryPhase(service *taskGenerationService) *taskGenerationNavigationDispatchPrimaryPhase

func (p *taskGenerationNavigationDispatchPrimaryPhase) run(
	ctx context.Context,
	taskID string,
	target *GenerationReviewNavigationTarget,
	responseMode string,
) (*GenerationReviewNavigationDispatchResponse, error)
```

Important:

- preserve current action / preview / queue / session behavior
- keep optional plan execution out of this seam
- do not finalize or merge executed plans here

- [ ] **Step 4: Route navigation primary dispatch through the new seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:479) so the inline routing body is replaced by one `buildTaskGenerationNavigationDispatchPrimaryPhase(s).run(...)` handoff.

- [ ] **Step 5: Re-run focused primary-dispatch verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigationPrimaryRunRoutesDispatchKinds$" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_navigation_dispatch_primary.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_navigation_dispatch_test.go
git commit -m "refactor: extract listingkit navigation primary seam"
```

---

## Task 3: Extract dispatch projection/finalization seam

**Files:**
- Create: `internal/listingkit/task_generation_navigation_dispatch_projection.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify: `internal/listingkit/service_generation_navigation_dispatch_test.go`
- Reuse: `internal/listingkit/service_generation_navigation_dispatch_helpers.go`

- [ ] **Step 1: Write the failing projection/finalization tests**

Extend [service_generation_navigation_dispatch_test.go](/D:/code/task-processor/internal/listingkit/service_generation_navigation_dispatch_test.go:1) with focused coverage that locks:

1. `execute_plan` still merges executed-plan results into the primary response
2. final dispatch responses still flow through `finalizeGenerationReviewNavigationDispatchResponse(...)`
3. `resolve_only` still skips plan execution

Add at least one narrow service-boundary test shape:

```go
func TestTaskGenerationNavigationDispatchProjectionAppliesExecutedPlanAndFinalizes(t *testing.T) {
	t.Parallel()

	response := &GenerationReviewNavigationDispatchResponse{
		TaskID:       "task-navigation-projection-1",
		DispatchKind: "session",
		ResponseMode: "patch_only",
	}
	execution := &GenerationNavigationDispatchExecution{
		Strategy: "parallel",
		Steps: []GenerationNavigationDispatchExecutionStep{
			{Kind: "preview", Status: "completed"},
		},
	}

	projected := buildTaskGenerationNavigationDispatchProjectionPhase().run(response, "execute_plan", execution)
	if projected == nil || projected.PlanMode != "execute_plan" {
		t.Fatalf("projected = %+v, want execute_plan response", projected)
	}
	if projected.ExecutedPlan == nil {
		t.Fatalf("projected response should keep executed plan")
	}
}
```

- [ ] **Step 2: Run focused projection/finalization verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchProjectionAppliesExecutedPlanAndFinalizes$" -count=1
```

Expected: FAIL because `buildTaskGenerationNavigationDispatchProjectionPhase()` does not exist yet.

- [ ] **Step 3: Add the dispatch projection/finalization seam**

Create `internal/listingkit/task_generation_navigation_dispatch_projection.go` with a seam that owns:

- `response.PlanMode` assignment
- optional executed-plan merge
- final response normalization through `finalizeGenerationReviewNavigationDispatchResponse(...)`

Suggested shape:

```go
type taskGenerationNavigationDispatchProjectionPhase struct{}

func buildTaskGenerationNavigationDispatchProjectionPhase() *taskGenerationNavigationDispatchProjectionPhase

func (p *taskGenerationNavigationDispatchProjectionPhase) run(
	response *GenerationReviewNavigationDispatchResponse,
	planMode string,
	executedPlan *GenerationNavigationDispatchExecution,
) *GenerationReviewNavigationDispatchResponse
```

Important:

- preserve current `execute_plan` merge behavior
- keep plan execution itself outside this seam
- do not change helper semantics in `service_generation_navigation_dispatch_helpers.go` unless a test forces it

- [ ] **Step 4: Route `DispatchTaskGenerationNavigation(...)` through the projection seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:350) so the inline plan-mode assignment, optional merge, and finalization block are replaced by one projection seam handoff.

- [ ] **Step 5: Re-run focused projection/finalization verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchProjectionAppliesExecutedPlanAndFinalizes$" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_navigation_dispatch_projection.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_navigation_dispatch_test.go internal/listingkit/service_generation_navigation_dispatch_helpers.go
git commit -m "refactor: extract listingkit navigation projection seam"
```

---

## Task 4: Lock navigation ownership guardrails

**Files:**
- Create: `internal/listingkit/phase10b_generation_navigation_boundary_test.go`
- Modify: `internal/listingkit/service_generation_navigation_dispatch_test.go`
- Verify: `internal/listingkit/task_generation_service.go`
- Verify: `internal/listingkit/task_generation_navigation_dispatch_entry.go`
- Verify: `internal/listingkit/task_generation_navigation_dispatch_primary.go`
- Verify: `internal/listingkit/task_generation_navigation_dispatch_projection.go`

- [ ] **Step 1: Add failing ownership guardrails**

Create [phase10b_generation_navigation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase10b_generation_navigation_boundary_test.go:1) with source-boundary tests that lock:

1. `DispatchTaskGenerationNavigation(...)` delegates to entry, primary, and projection seams
2. entry normalization logic does not grow back into the service entry
3. primary route-selection logic does not grow back into the service entry
4. executed-plan merge/finalization logic does not grow back into the service entry
5. primary dispatch seam does not take over plan execution mechanics

Suggested structure:

```go
func TestTaskGenerationNavigationDelegationBoundary(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "task_generation_service.go", "DispatchTaskGenerationNavigation")

	assertSourceOccurrenceCount(t, source, "buildTaskGenerationNavigationDispatchEntry()", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationNavigationDispatchPrimaryPhase(s)", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationNavigationDispatchProjectionPhase()", 1)
}
```

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigation.*Boundary" -count=1
```

Expected: FAIL until the guardrails match the final seam split.

- [ ] **Step 3: Narrow brittle assertions before finalizing**

Keep the ownership tests anchored on:

- helper names
- literal dispatch markers
- occurrence counts
- explicit forbidden helper calls

Avoid fragile dependence on:

- local variable names
- inline struct layout
- exact whitespace or append formatting

- [ ] **Step 4: Run final navigation-focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigation.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase10b_generation_navigation_boundary_test.go internal/listingkit/service_generation_navigation_dispatch_test.go internal/listingkit/task_generation_service.go internal/listingkit/task_generation_navigation_dispatch_entry.go internal/listingkit/task_generation_navigation_dispatch_primary.go internal/listingkit/task_generation_navigation_dispatch_projection.go
git commit -m "test: lock listingkit navigation dispatch boundaries"
```

---

## Verification Checklist For The Whole Phase

At the end of the full phase, run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigation.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

If the working tree still contains unrelated submit-path edits, do **not** silently broaden this phase to fix them. Record that broader `./internal/listingkit` package verification may still be noisy for out-of-slice reasons.

---

## Expected Commits

This phase should land as four small commits:

1. `refactor: extract listingkit navigation entry seam`
2. `refactor: extract listingkit navigation primary seam`
3. `refactor: extract listingkit navigation projection seam`
4. `test: lock listingkit navigation dispatch boundaries`

Keep them in that order so each seam becomes reviewable on its own.
