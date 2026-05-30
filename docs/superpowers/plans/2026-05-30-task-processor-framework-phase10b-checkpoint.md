# Task Processor Framework Phase 10B Checkpoint

## Status

`Phase 10B` is functionally complete for the intended slice.

This phase was not about redesigning ListingKit navigation business policy, reopening dispatch-plan parallelism, or introducing a generic dispatch framework. The goal was narrower:

1. stop [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:350) from remaining the shared home of navigation entry normalization, primary dispatch routing, optional executed-plan merge, and final response shaping at the same time
2. make those navigation responsibilities explicit through feature-local seams
3. preserve current action / preview / queue / session navigation behavior, including implicit routing and `primary_only` / `execute_plan` semantics
4. lock the new ownership split so the old inline navigation logic does not silently grow back into the service entry

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Navigation entry normalization now has its own local seam

The navigation-entry seam now lives in:

- [internal/listingkit/task_generation_navigation_dispatch_entry.go](/D:/code/task-processor/internal/listingkit/task_generation_navigation_dispatch_entry.go:1)

This seam now owns:

- nil request / nil target validation
- target clone
- `ApplyGenerationConditionalBaselineToNavigationTarget(...)`
- `responseMode` normalization
- `planMode` normalization

This matters because the root problem here was not method length. The risk was that service entry orchestration still directly owned request-shaping concerns that should stay decoupled from dispatch routing and response projection.

### 2. Primary dispatch routing now has its own local seam

The primary-dispatch seam now lives in:

- [internal/listingkit/task_generation_navigation_dispatch_primary.go](/D:/code/task-processor/internal/listingkit/task_generation_navigation_dispatch_primary.go:1)

This seam now owns:

- dispatch-kind routing
- action / preview / queue / session request shaping
- implicit routing through `normalizeGenerationReviewDispatchKind(...)`
- session-query precedence through `resolveTaskGenerationNavigationPrimarySessionQuery(...)`
- primary `GenerationReviewNavigationDispatchResponse` assembly

This is the main routing ownership split of the phase. The service entry no longer directly decides how to fan out across action, preview, queue, and session paths.

### 3. Dispatch projection/finalization now has its own local seam

The projection/finalization seam now lives in:

- [internal/listingkit/task_generation_navigation_dispatch_projection.go](/D:/code/task-processor/internal/listingkit/task_generation_navigation_dispatch_projection.go:1)

This seam now owns:

- `PlanMode` assignment on the dispatch response
- optional `execute_plan` merge through `applyExecutedPlanToDispatchResponse(...)`
- final response normalization through `finalizeGenerationReviewNavigationDispatchResponse(...)`

This is the main response-model ownership split of the phase. The service entry no longer directly performs executed-plan merge and final panel normalization inline.

### 4. The service entry is now orchestration-focused

The service entry still lives in:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:350)

It now mainly coordinates:

1. entry normalization seam handoff
2. primary dispatch seam handoff
3. optional dispatch-plan execution
4. projection/finalization seam handoff

It no longer directly owns:

- target clone / conditional baseline application
- response-mode and plan-mode normalization
- action / preview / queue / session routing
- session-query precedence resolution
- executed-plan merge into the primary response
- final navigation response normalization

That is the main ownership outcome of this phase.

### 5. Guardrails now lock navigation ownership boundaries

The ownership protections now live in:

- [internal/listingkit/phase10b_generation_navigation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase10b_generation_navigation_boundary_test.go:1)
- [internal/listingkit/service_generation_navigation_dispatch_test.go](/D:/code/task-processor/internal/listingkit/service_generation_navigation_dispatch_test.go:1)

These now protect five things:

1. `DispatchTaskGenerationNavigation(...)` must continue delegating to entry, primary, and projection seams
2. entry normalization must stay out of the service entry
3. primary routing must stay out of the service entry
4. executed-plan merge and finalization must stay out of the service entry
5. primary and projection seams must not reach back into top-level dispatch or down into plan execution internals

The current guardrails are intentionally narrower than the first drafts in this phase. They now lean on helper names, occurrence counts, explicit forbidden helper calls, and behavior-level routing tests instead of locking whole source-text shapes or individual assignment lines.

## Acceptance Check

`Phase 10B` was meant to prove four things:

1. navigation entry normalization, primary routing, and dispatch projection can live behind separate explicit ListingKit-owned seams
2. the service entry can become more orchestration-focused without changing navigation behavior
3. implicit routing and session precedence can stay protected after the ownership split
4. the new split can be protected with focused behavior tests and low-fragility boundary guardrails

All four are now true.

More concretely:

- `task_generation_service.go` no longer owns the old mixed navigation block
- entry, routing, and projection now have separate local homes
- implicit routing and `primary_only` / `execute_plan` semantics remain covered
- guardrails now block the most likely regrowth directions back into the service entry

## What This Phase Did Not Try To Solve

### 1. It did not redesign dispatch-plan execution semantics

This phase deliberately did not reopen:

- sequential vs parallel plan execution
- deduplication policy
- step-level execution rules
- stop/fallback strategy semantics

That was the right tradeoff. The actual hotspot was service-entry ownership, not plan-engine behavior.

### 2. It did not introduce a generic navigation framework

All new seams remain local to ListingKit:

- navigation entry
- navigation primary dispatch
- navigation projection/finalization

That is appropriate here. The pressure was concentrated inside one feature path, not spread across the repo.

### 3. It did not fold submit-path stabilization into the same slice

The working tree still contains an unrelated change in:

- [data/sensitive_words_shein.json](/D:/code/task-processor/data/sensitive_words_shein.json:1)

This phase intentionally left that alone. Framework-evolution work and submit-path stabilization should not be silently merged into one slice.

## Residual Responsibilities Still Present

### Boundary tests are still source-shape guardrails

The ownership tests intentionally check:

- seam handoff markers
- forbidden helper calls
- occurrence counts on specific ownership signals

That is pragmatic and consistent with the current testing style, but it is still a source-shape strategy rather than an AST-level contract.

### Projection seam still returns the same response pointer in-place

The projection seam currently mutates and returns the same response instance. That is acceptable for now because it preserves the pre-split calling convention, but it slightly reduces freedom for a future purely functional projection refactor.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “keep carving navigation seams for symmetry.” Better next steps are:

### 1. Watch whether dispatch-plan mechanics become the next real hotspot

If future changes keep landing around:

- plan deduplication
- sequential vs parallel execution
- step-level response aggregation

then the next slice should be driven by that concrete pressure, not by the existence of `Phase 10B`.

### 2. Leave this layer alone unless a new ownership hotspot appears

This layer is now in a good enough state:

- entry normalization is explicit
- routing is explicit
- projection is explicit
- service orchestration is visible
- implicit routing is covered
- guardrails exist

Do not keep editing it for symmetry alone.

## Verification Summary

The final `Phase 10B` verification that passed on this branch was:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigation.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Additional focused verification that passed during the phase included:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchEntryRunNormalizesTargetAndPlanMode$|TestTaskGenerationNavigationDispatchEntryRunRejectsMissingTarget$" -count=1
go test ./internal/listingkit -run "TestDispatchTaskGenerationNavigationDefaultsPlanModeToPrimaryOnly$" -count=1
go test ./internal/listingkit -run "TestTaskGenerationNavigationPrimaryRunRoutesDispatchKinds$" -count=1
go test ./internal/listingkit -run "TestTaskGenerationNavigationDispatchProjectionAppliesExecutedPlanAndFinalizes$|TestTaskGenerationNavigationDispatchProjectionSkipsExecutedPlanForPrimaryOnly$" -count=1
go test ./internal/listingkit -run "TestTaskGenerationNavigation.*Boundary" -count=1
```
