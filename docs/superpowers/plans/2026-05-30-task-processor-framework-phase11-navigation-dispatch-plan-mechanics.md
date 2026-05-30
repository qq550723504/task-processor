# Task Processor Framework Phase 11 ListingKit Navigation Dispatch Plan Mechanics Ownership Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the remaining navigation dispatch plan-engine ownership complexity in ListingKit by making plan orchestration, parallel scheduling/deduplication, and step execution/result aggregation flow through explicit feature-owned seams instead of remaining clustered inside `executeGenerationNavigationDispatchPlan(...)`.

**Architecture:** Reuse the same bounded-seam pattern already established in `Phase 10B`. Do not invent a generic planner framework. Instead, split the current plan engine in `task_generation_service.go` into three ListingKit-owned local seams: plan orchestration, parallel scheduling/deduplication, and step execution/result shaping. Keep business behavior unchanged and preserve current stop/skip/dedup/winner semantics.

**Tech Stack:** Go, existing ListingKit navigation dispatch plan helpers, navigation dispatch rules/merge helpers, existing navigation dispatch tests, source-boundary guardrails

**Out of Scope For This Slice:**

- redesigning navigation dispatch business rules
- changing fallback / recovery semantics unless a failing test proves a concrete need
- reopening the already-stable entry / primary / projection seams from `Phase 10B`
- folding submit-path stabilization work into the same phase
- inventing a repo-wide planner or scheduler abstraction

---

## Root Cause This Slice Addresses

After `Phase 10B`, the top-level navigation path is clearer, but the plan engine still concentrates several different responsibilities inside one service-side execution cluster:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:483)

Today `executeGenerationNavigationDispatchPlan(...)` and its adjacent methods still jointly decide:

1. whether a dispatch plan exists and should execute
2. whether execution runs sequentially or in parallel
3. how duplicate steps are detected and coalesced
4. how step execution results are recorded and counted
5. when stop conditions and skipped-step backfill trigger
6. how step execution returns queue / preview / session result state

The problem is not just file size. The real problem is that plan-engine ownership is still implicit and crosses three kinds of concern:

- plan orchestration
- parallel scheduling / deduplication
- step execution / execution-result shaping

That makes future changes risky because scheduling and aggregation changes can still leak across one service block without one clear seam to test or evolve.

---

## Target Outcome

At the end of `Phase 11`:

- plan orchestration flows through an explicit ListingKit-owned seam
- parallel scheduling / deduplication flows through an explicit ListingKit-owned seam
- step execution and execution-result shaping flow through an explicit ListingKit-owned seam
- `executeGenerationNavigationDispatchPlan(...)` becomes more orchestration-focused
- current stop / skip / dedup / response-mode semantics remain unchanged
- boundary tests lock the new ownership split

---

## Task 1: Extract plan orchestration seam

**Files:**
- Create: `internal/listingkit/task_generation_navigation_dispatch_plan.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify: `internal/listingkit/service_generation_navigation_dispatch_test.go`

- [ ] **Step 1: Write the failing orchestration-focused tests**

Extend [service_generation_navigation_dispatch_test.go](/D:/code/task-processor/internal/listingkit/service_generation_navigation_dispatch_test.go:1) with focused coverage that locks:

1. nil / missing `DispatchPlan` still returns `nil, nil`
2. orchestration still chooses parallel execution only when `generationNavigationDispatchPlanRunsInParallel(...)` says so
3. execution rules still apply after sequential or parallel completion

Add at least one direct seam-level test shape:

```go
func TestTaskGenerationNavigationDispatchPlanRunChoosesExecutionModeAndAppliesRules(t *testing.T) {
	t.Parallel()

	plan := &GenerationNavigationDispatchPlan{
		Steps: []GenerationNavigationDispatchStep{
			{Kind: "queue"},
			{Kind: "session"},
		},
	}
	target := &GenerationReviewNavigationTarget{
		Descriptor: &GenerationNavigationDescriptor{DispatchPlan: plan},
	}

	svc := newTaskGenerationNavigationDispatchTestService(t)
	execution, err := buildTaskGenerationNavigationDispatchPlanPhase(svc).run(
		context.Background(),
		"task-navigation-plan-1",
		target,
		"full",
	)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if execution == nil {
		t.Fatalf("execution = nil, want orchestrated dispatch execution")
	}
}
```

- [ ] **Step 2: Run focused orchestration verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchPlanRunChoosesExecutionModeAndAppliesRules$" -count=1
```

Expected: FAIL because `buildTaskGenerationNavigationDispatchPlanPhase(...)` does not exist yet.

- [ ] **Step 3: Add the plan orchestration seam**

Create `internal/listingkit/task_generation_navigation_dispatch_plan.go` with a focused local seam that owns:

- dispatch-plan existence / clone / execution object setup
- sequential vs parallel branch selection
- post-execution rules application

Suggested shape:

```go
type taskGenerationNavigationDispatchPlanPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationNavigationDispatchPlanPhase(service *taskGenerationService) *taskGenerationNavigationDispatchPlanPhase

func (p *taskGenerationNavigationDispatchPlanPhase) run(
	ctx context.Context,
	taskID string,
	target *GenerationReviewNavigationTarget,
	responseMode string,
) (*GenerationNavigationDispatchExecution, error)
```

Important:

- preserve nil / missing plan behavior
- preserve `generationNavigationDispatchPlanRunsInParallel(...)`
- keep sequential / parallel bodies out of this seam for the next tasks

- [ ] **Step 4: Route `executeGenerationNavigationDispatchPlan(...)` through the orchestration seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:483) so the top-level plan block becomes a handoff to `buildTaskGenerationNavigationDispatchPlanPhase(s).run(...)`.

- [ ] **Step 5: Re-run focused orchestration verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchPlanRunChoosesExecutionModeAndAppliesRules$" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_navigation_dispatch_plan.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_navigation_dispatch_test.go
git commit -m "refactor: extract listingkit navigation plan orchestration seam"
```

---

## Task 2: Extract parallel scheduling and deduplication seam

**Files:**
- Create: `internal/listingkit/task_generation_navigation_dispatch_plan_parallel.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify: `internal/listingkit/service_generation_navigation_dispatch_test.go`
- Reuse: `internal/listingkit/service_generation_navigation_dispatch_helpers.go`

- [ ] **Step 1: Write the failing parallel/dedup tests**

Extend [service_generation_navigation_dispatch_test.go](/D:/code/task-processor/internal/listingkit/service_generation_navigation_dispatch_test.go:1) with focused coverage that locks:

1. duplicate plan steps still collapse onto one executed source
2. deduplicated steps still inherit `DeltaToken` / `NotModified` / `NoChanges` from the source step
3. `MaxParallelism <= 0` still falls back to `1`

Add at least one direct seam-level test shape:

```go
func TestTaskGenerationNavigationDispatchParallelPhaseDeduplicatesAndReplaysSourceState(t *testing.T) {
	t.Parallel()

	svc := newTaskGenerationNavigationDispatchTestService(t)
	plan := &GenerationNavigationDispatchPlan{
		MaxParallelism: 0,
		Steps: []GenerationNavigationDispatchStep{
			{Kind: "queue", Query: &GenerationQueueQuery{Platform: "shein", Slot: "main"}},
			{Kind: "queue", Query: &GenerationQueueQuery{Platform: "shein", Slot: "main"}},
		},
	}
	execution := &GenerationNavigationDispatchExecution{}

	buildTaskGenerationNavigationDispatchPlanParallelPhase(svc).run(
		context.Background(),
		"task-navigation-parallel-1",
		"full",
		plan,
		execution,
	)

	if len(execution.Steps) != 2 {
		t.Fatalf("steps = %+v, want original fanout shape preserved", execution.Steps)
	}
}
```

- [ ] **Step 2: Run focused parallel/dedup verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchParallelPhaseDeduplicatesAndReplaysSourceState$" -count=1
```

Expected: FAIL because `buildTaskGenerationNavigationDispatchPlanParallelPhase(...)` does not exist yet.

- [ ] **Step 3: Add the parallel scheduling/dedup seam**

Create `internal/listingkit/task_generation_navigation_dispatch_plan_parallel.go` with a seam that owns:

- dedupe entry construction
- `MaxParallelism` fallback
- goroutine fan-out and join
- deduplicated-step replay of source state back into `execution.Steps`

Suggested shape:

```go
type taskGenerationNavigationDispatchPlanParallelPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationNavigationDispatchPlanParallelPhase(service *taskGenerationService) *taskGenerationNavigationDispatchPlanParallelPhase

func (p *taskGenerationNavigationDispatchPlanParallelPhase) run(
	ctx context.Context,
	taskID string,
	responseMode string,
	plan *GenerationNavigationDispatchPlan,
	execution *GenerationNavigationDispatchExecution,
)
```

Important:

- preserve dedupe-key behavior
- preserve result replay for deduplicated steps
- keep actual per-step queue/preview/session reads out of this seam

- [ ] **Step 4: Route the parallel branch through the new seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:505) so the old parallel block becomes a handoff to `buildTaskGenerationNavigationDispatchPlanParallelPhase(s).run(...)`.

- [ ] **Step 5: Re-run focused parallel/dedup verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchParallelPhaseDeduplicatesAndReplaysSourceState$|TestExecuteGenerationNavigationDispatchPlanDeduplicatesDuplicateSteps$" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_navigation_dispatch_plan_parallel.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_navigation_dispatch_test.go internal/listingkit/service_generation_navigation_dispatch_helpers.go
git commit -m "refactor: extract listingkit navigation parallel plan seam"
```

---

## Task 3: Extract step execution and sequential aggregation seam

**Files:**
- Create: `internal/listingkit/task_generation_navigation_dispatch_step_execution.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify: `internal/listingkit/service_generation_navigation_dispatch_test.go`
- Reuse: `internal/listingkit/service_generation_navigation_dispatch_helpers.go`

- [ ] **Step 1: Write the failing step-execution tests**

Extend [service_generation_navigation_dispatch_test.go](/D:/code/task-processor/internal/listingkit/service_generation_navigation_dispatch_test.go:1) with focused coverage that locks:

1. step execution still routes `queue` / `preview` / `session` reads correctly
2. `ResponseMode` still flows into step query shaping
3. sequential execution still propagates stop reason and skipped-step backfill correctly

Add at least one direct seam-level test shape:

```go
func TestTaskGenerationNavigationDispatchStepExecutionRunBuildsStepResults(t *testing.T) {
	t.Parallel()

	svc := newTaskGenerationNavigationDispatchTestService(t)
	step := GenerationNavigationDispatchStep{
		Kind:  "queue",
		Query: &GenerationQueueQuery{Platform: "shein", Slot: "main"},
	}

	result := buildTaskGenerationNavigationDispatchStepExecutionPhase(svc).run(
		context.Background(),
		"task-navigation-step-1",
		step,
		"patch_only",
	)

	if result == nil || result.Kind != "queue" {
		t.Fatalf("result = %+v, want queue step result", result)
	}
}
```

- [ ] **Step 2: Run focused step-execution verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchStepExecutionRunBuildsStepResults$" -count=1
```

Expected: FAIL because `buildTaskGenerationNavigationDispatchStepExecutionPhase(...)` does not exist yet.

- [ ] **Step 3: Add the step-execution seam**

Create `internal/listingkit/task_generation_navigation_dispatch_step_execution.go` with a seam that owns:

- per-step queue / preview / session execution
- per-step `DeltaToken` / `NotModified` / `NoChanges` shaping
- sequential loop aggregation through a helper or sibling seam

Suggested shape:

```go
type taskGenerationNavigationDispatchStepExecutionPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationNavigationDispatchStepExecutionPhase(service *taskGenerationService) *taskGenerationNavigationDispatchStepExecutionPhase

func (p *taskGenerationNavigationDispatchStepExecutionPhase) run(
	ctx context.Context,
	taskID string,
	step GenerationNavigationDispatchStep,
	responseMode string,
) *GenerationNavigationDispatchExecutionStep
```

If needed, add a small sequential-aggregation helper in the same file rather than creating an extra file in this phase.

Important:

- preserve `ResponseMode` passthrough
- preserve step error classification
- preserve stop / skipped-step semantics in sequential execution

- [ ] **Step 4: Route sequential execution and step execution through the new seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:494) so:

- `executeGenerationNavigationDispatchPlanSequential(...)` delegates step execution to the new seam
- `executeGenerationNavigationDispatchPlanStep(...)` becomes a thin wrapper or delegates completely

- [ ] **Step 5: Re-run focused step-execution verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchStepExecutionRunBuildsStepResults$|TestExecuteGenerationNavigationDispatchPlanDeduplicatesDuplicateSteps$|TestDispatchTaskGenerationNavigationExecutesDispatchPlanForSessionTarget$" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_navigation_dispatch_step_execution.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_navigation_dispatch_test.go internal/listingkit/service_generation_navigation_dispatch_helpers.go
git commit -m "refactor: extract listingkit navigation step execution seam"
```

---

## Task 4: Lock navigation plan-engine guardrails

**Files:**
- Create: `internal/listingkit/phase11_generation_navigation_plan_boundary_test.go`
- Modify if needed: `internal/listingkit/service_generation_navigation_dispatch_test.go`
- Verify: `internal/listingkit/task_generation_navigation_dispatch_plan.go`
- Verify: `internal/listingkit/task_generation_navigation_dispatch_plan_parallel.go`
- Verify: `internal/listingkit/task_generation_navigation_dispatch_step_execution.go`
- Verify: `internal/listingkit/task_generation_service.go`

- [ ] **Step 1: Add failing ownership guardrails**

Create [phase11_generation_navigation_plan_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase11_generation_navigation_plan_boundary_test.go:1) with source-boundary tests that lock:

1. top-level plan seam delegates orchestration to parallel / sequential sub-seams
2. parallel seam owns dedupe preparation and replay, but not queue/preview/session reads
3. step-execution seam owns queue/preview/session reads, but not top-level orchestration or plan-merge/finalization
4. top-level plan seam does not reabsorb dedupe/scheduling/step-read details

Suggested structure:

```go
func TestTaskGenerationNavigationDispatchPlanBoundary(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "task_generation_navigation_dispatch_plan.go", "run")

	assertSourceContainsAll(t, source, []string{
		"generationNavigationDispatchPlanRunsInParallel(",
	})
	assertSourceExcludesAll(t, source, []string{
		"GetTaskGenerationQueue(",
		"GetTaskGenerationReviewPreview(",
		"GetTaskGenerationReviewSession(",
	})
}
```

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchPlan.*Boundary" -count=1
```

Expected: FAIL until the guardrails match the final seam split.

- [ ] **Step 3: Keep the guardrails low-fragility**

Anchor the ownership tests on:

- helper names
- occurrence counts
- explicit forbidden helper calls
- small signature-level or responsibility-level signals

Avoid fragile dependence on:

- local variable names
- exact loop bodies
- goroutine formatting
- whitespace-sensitive snippets

- [ ] **Step 4: Run final plan-engine verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigation.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase11_generation_navigation_plan_boundary_test.go internal/listingkit/service_generation_navigation_dispatch_test.go internal/listingkit/task_generation_navigation_dispatch_plan.go internal/listingkit/task_generation_navigation_dispatch_plan_parallel.go internal/listingkit/task_generation_navigation_dispatch_step_execution.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_navigation_dispatch_helpers.go
git commit -m "test: lock listingkit navigation plan boundaries"
```

---

## Verification Checklist For The Whole Phase

At the end of the full phase, run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigation.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

If unrelated working-tree changes are still present, do **not** silently broaden this phase to fix them. Record that broader non-navigation verification may still be noisy for out-of-slice reasons.

---

## Expected Commits

This phase should land as four small commits:

1. `refactor: extract listingkit navigation plan orchestration seam`
2. `refactor: extract listingkit navigation parallel plan seam`
3. `refactor: extract listingkit navigation step execution seam`
4. `test: lock listingkit navigation plan boundaries`

Keep them in that order so each seam becomes reviewable on its own.
