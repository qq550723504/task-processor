# Task Processor Framework Phase 11 Checkpoint

## Status

`Phase 11` is functionally complete for the intended slice.

This phase was not about redesigning ListingKit navigation business semantics, reopening `Phase 10B` entry/routing/projection ownership, or introducing a generic plan-engine framework. The goal was narrower:

1. stop [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:475) from remaining the shared home of plan orchestration, parallel dedup scheduling, step-level read execution, and sequential stop/backfill behavior at the same time
2. make those navigation dispatch plan mechanics explicit through feature-local seams
3. preserve current `queue` / `preview` / `session` execution behavior, dedupe replay, and sequential stop semantics
4. lock the new ownership split so the old inline plan-engine logic does not silently grow back into the service layer

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Plan orchestration now has its own local seam

The top-level plan orchestration seam now lives in:

- [internal/listingkit/task_generation_navigation_dispatch_plan.go](/D:/code/task-processor/internal/listingkit/task_generation_navigation_dispatch_plan.go:1)

This seam now owns:

- nil target / nil descriptor / nil dispatch-plan handling
- plan clone
- execution root initialization
- sequential vs parallel branch selection
- post-execution rule application through `applyGenerationNavigationDispatchExecutionRules(...)`

This matters because the root problem here was not just function size. The risk was that top-level dispatch-plan ownership still mixed orchestration with scheduling, step execution, and result shaping details.

### 2. Parallel scheduling and dedup replay now have their own local seam

The parallel seam now lives in:

- [internal/listingkit/task_generation_navigation_dispatch_plan_parallel.go](/D:/code/task-processor/internal/listingkit/task_generation_navigation_dispatch_plan_parallel.go:1)

This seam now owns:

- dedupe entry construction
- `MaxParallelism <= 0` fallback to `1`
- goroutine fan-out / join
- deduplicated-step source-state replay
- parallel execution stats aggregation

It no longer mixes those responsibilities back into the service method body.

### 3. Step execution and sequential aggregation now have their own local seam

The step-execution seam now lives in:

- [internal/listingkit/task_generation_navigation_dispatch_step_execution.go](/D:/code/task-processor/internal/listingkit/task_generation_navigation_dispatch_step_execution.go:1)

This seam now owns:

- per-step `queue` / `preview` / `session` execution
- `ResponseMode` passthrough into query shaping
- per-step `DeltaToken` / `NotModified` / `NoChanges` shaping
- error classification on failed step execution
- sequential loop aggregation
- `StopReason` propagation
- skipped-step backfill

This is the main execution ownership split of the phase. The service layer no longer directly performs step-level read branching inline.

### 4. The service methods are now orchestration-focused wrappers

The service entry points still live in:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:475)

They now mainly coordinate:

1. top-level plan seam handoff
2. parallel seam handoff
3. step-execution seam handoff

They no longer directly own:

- dedupe entry creation
- semaphore / waitgroup scheduling details
- source-state replay for deduplicated steps
- step-level `queue` / `preview` / `session` read execution
- sequential stop / skipped-step backfill loops

That is the main ownership outcome of this phase.

### 5. Guardrails now lock navigation plan-engine boundaries

The ownership protections now live in:

- [internal/listingkit/phase11_generation_navigation_plan_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase11_generation_navigation_plan_boundary_test.go:1)
- [internal/listingkit/service_generation_navigation_dispatch_test.go](/D:/code/task-processor/internal/listingkit/service_generation_navigation_dispatch_test.go:1)

These now protect four things:

1. top-level/service-level plan methods must continue delegating to seam-owned helpers
2. parallel seam must continue owning dedupe preparation and replay, but not step-level reads
3. step-execution seam must continue owning `queue` / `preview` / `session` reads, but not top-level orchestration or finalization
4. top-level plan seam must not reabsorb dedupe, scheduling, or step-read details

The current guardrails intentionally lean on helper names, explicit forbidden calls, and focused source-boundary checks instead of locking loop bodies or goroutine formatting.

## Acceptance Check

`Phase 11` was meant to prove four things:

1. plan orchestration, parallel scheduling/dedup, and step execution can live behind separate explicit ListingKit-owned seams
2. the service layer can become more orchestration-focused without changing navigation dispatch behavior
3. dedupe replay and sequential stop/backfill semantics can stay protected after the ownership split
4. the new split can be protected with focused behavior tests and low-fragility guardrails

All four are now true.

More concretely:

- `task_generation_service.go` no longer owns the old mixed plan-engine block
- orchestration, parallel scheduling, and step execution now have separate local homes
- dedupe replay and sequential backfill semantics remain covered
- guardrails now block the most likely regrowth directions back into the service layer

## What This Phase Did Not Try To Solve

### 1. It did not redesign navigation business policy

This phase deliberately did not reopen:

- dispatch target normalization
- primary navigation routing
- executed-plan response projection
- action-path behavior

That was the right tradeoff. Those ownership seams were already settled in `Phase 10B`.

### 2. It did not introduce a generic planner or executor framework

All new seams remain local to ListingKit:

- plan orchestration
- parallel scheduling / dedup replay
- step execution / sequential aggregation

That is appropriate here. The pressure was concentrated inside one feature path, not spread across the repo.

### 3. It did not fold unrelated working-tree changes into this slice

The working tree still contains an unrelated change in:

- [data/sensitive_words_shein.json](/D:/code/task-processor/data/sensitive_words_shein.json:1)

This phase intentionally left that alone.

## Residual Responsibilities Still Present

### Boundary tests still rely on stable seam/helper names

The ownership tests intentionally check:

- seam handoff markers
- explicit forbidden helper calls
- occurrence counts on ownership signals

That keeps them low-fragility compared with line-shape assertions, but they still depend on stable seam/helper naming. A future rename-only refactor will need test updates.

### Parallel and step seam guardrails use file-level source checks

For the seam-owned files, the current tests use whole-file checks rather than only extracting a single function body. That gives stronger ownership protection, but it also means unrelated helpers added to the same file could trigger false positives if they cross the forbidden boundary.

That is acceptable for now because both files are intentionally seam-owned files.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “keep carving navigation seams for symmetry.” Better next steps are:

### 1. Watch whether navigation dispatch plan mechanics still carry another concrete hotspot

If future changes keep landing around:

- sequential/parallel execution policy
- step result aggregation
- retry / dedupe interaction

then the next slice should be driven by that concrete pressure, not by the existence of `Phase 11`.

### 2. Leave this layer alone unless a new ownership hotspot appears

This layer is now in a good enough state:

- orchestration is explicit
- parallel scheduling is explicit
- step execution is explicit
- service wrappers are thin
- behavior coverage exists
- ownership guardrails exist

Do not keep editing it for symmetry alone.

## Verification Summary

The final `Phase 11` verification that passed on this branch was:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigation.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Additional focused verification that passed during the phase included:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchPlanRunChoosesExecutionModeAndAppliesRules$|TestTaskGenerationNavigationDispatchPlanRunReturnsNilForMissingDispatchPlan$|TestExecuteGenerationNavigationDispatchPlanDeduplicatesDuplicateSteps$" -count=1
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchParallelPhaseDeduplicatesAndReplaysSourceState$|TestExecuteGenerationNavigationDispatchPlanDeduplicatesDuplicateSteps$" -count=1
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchStepExecutionRunBuildsStepResults$|TestTaskGenerationNavigationDispatchStepExecutionRunSequentialBackfillsSkippedSteps$|TestDispatchTaskGenerationNavigationExecutesDispatchPlanForSessionTarget$" -count=1
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchPlan.*Boundary" -count=1
```
