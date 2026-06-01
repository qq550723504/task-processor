# Task Processor Framework Phase 15 Checkpoint

## Status

`Phase 15` is functionally complete for the intended slice.

This phase was not about reopening action execution ownership, revisiting navigation dispatch planning, or inventing a generic “current state provider” abstraction. The goal was narrower:

1. stop [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:318) through [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:342) from remaining the shared home of current result acquisition and current queue/overview/render-preview derivation at the same time
2. make those responsibilities explicit through feature-local ListingKit seams
3. preserve current action-side and read-side behavior
4. lock the new ownership split with low-fragility boundary guardrails

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Current generation-state acquisition now has its own local seam

The snapshot seam now lives in:

- [internal/listingkit/task_generation_current_state_snapshot.go](/D:/code/task-processor/internal/listingkit/task_generation_current_state_snapshot.go:1)

This seam now owns:

- `repo.GetTask(...)`
- `listAssetGenerationTasks(...)`
- `listGenerationReviews(...)`
- handoff of the current `Task`
- decorated `ListingKitResult` handoff via `withListingKitResultGenerationAndReview(...)`

This matters because the original hotspot was not only helper count. The real ownership pressure was that current task/result acquisition and current queue/overview/render-preview consumers still depended on an implicit shared path inside the service layer.

### 2. Current generation-state view derivation now has its own local seam

The view seam now lives in:

- [internal/listingkit/task_generation_current_state_views.go](/D:/code/task-processor/internal/listingkit/task_generation_current_state_views.go:1)

This seam now owns:

- current overview derivation from result
- current queue derivation from result
- current action render-preview derivation from result plus query

This is the main shaping split of the phase. The current-state helper cluster no longer directly mixes result acquisition with current queue/overview/render-preview derivation inline.

### 3. The remaining current-state helpers are now orchestration-focused

The current-state helpers still live in:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:318)

They now mainly coordinate:

1. snapshot seam handoff
2. view seam handoff

They no longer directly own:

- current `Task` acquisition
- persisted generation-task acquisition
- persisted review acquisition
- result decoration
- direct current queue/overview/render-preview shaping logic

That is the main ownership outcome of the phase.

### 4. Guardrails now lock current generation-state ownership boundaries

The ownership protections now live in:

- [internal/listingkit/phase15_generation_state_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase15_generation_state_boundary_test.go:1)
- [internal/listingkit/task_generation_service_test.go](/D:/code/task-processor/internal/listingkit/task_generation_service_test.go:1)

These now protect:

1. `getCurrentListingKitResult(...)` must continue delegating to the snapshot seam
2. snapshot seam must continue owning task/task-list/review acquisition and result decoration, but not queue/overview/render-preview derivation
3. service-level current queue/overview/render-preview helpers must continue delegating to the view seam
4. view seam methods must continue owning current queue/overview/render-preview derivation, but not task/result acquisition or sibling ownership

During this phase we explicitly tightened a first pass of the boundary tests so the final guardrails rely on helper names, occurrence counts, forbidden helper calls, and sibling-owner exclusion rather than local variable naming or layout-sensitive assertions.

## Acceptance Check

`Phase 15` was meant to prove four things:

1. current generation-state acquisition can live behind an explicit ListingKit-owned snapshot seam
2. current queue/overview/render-preview derivation can live behind an explicit ListingKit-owned view seam
3. the service helpers can become more orchestration-focused without changing behavior
4. the new split can be protected with focused behavior coverage and low-fragility ownership guardrails

All four are now true.

More concretely:

- `task_generation_service.go` no longer owns the old mixed current-state block
- acquisition and view derivation now have separate local homes
- action-side and read-side consumers still read the same current decorated result path
- the final boundary tests now block both acquisition regrowth and sibling view leakage

## What This Phase Did Not Try To Solve

### 1. It did not redesign action refresh semantics

This phase deliberately did not reopen:

- post-action refresh fallback hydration
- base-result preview fallback behavior
- current-result mutation semantics after refresh

That was the right tradeoff. The hotspot here was current-state ownership, not refresh business policy.

### 2. It did not redesign layer-temporal action branching

This phase also deliberately did not reopen:

- `executeLayerTemporalAction(...)`
- standard/platform-adapt temporal branch semantics
- temporal-only response shaping

That remains a separate candidate, but not the ownership pressure this slice targeted.

### 3. It did not introduce a generic current-state framework

All new seams remain local to ListingKit:

- current generation-state snapshot acquisition
- current generation-state view derivation

That is appropriate here. The pressure was concentrated inside one feature path, not spread across the repo.

## Residual Responsibilities Still Present

### Current generation-state guardrails still depend on stable seam/helper names

The ownership tests intentionally check:

- seam handoff markers
- explicit forbidden helper calls
- sibling-owner exclusion signals

That keeps them lower-fragility than layout-sensitive assertions, but they still depend on stable seam/helper naming. A future rename-only refactor will need test updates.

### Post-action refresh still carries the next likely ownership hotspot

After `Phase 15`, the most visible remaining mixed block is now:

- [internal/listingkit/task_generation_action_refresh.go](/D:/code/task-processor/internal/listingkit/task_generation_action_refresh.go:1)

That block still combines:

- current result refresh
- current overview/render-preview derivation
- fallback hydration from `baseResult`
- mutation of `currentResult` preview fields

That now looks like the next plausible ownership hotspot because the underlying current-state acquisition and view seams are explicit, but the refresh/fallback layer still clusters several responsibilities together.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “keep polishing current-state seams for symmetry.” Better next steps are:

### 1. Watch the post-action refresh/fallback hydration cluster

If future changes keep landing around:

- `taskGenerationActionRefreshPhase.run(...)`
- base-result preview fallback hydration
- current-result post-refresh shaping

then the next slice should be driven by that concrete pressure.

### 2. Leave the current-state acquisition layer alone unless a new hotspot appears

This layer is now in a good enough state:

- current-state acquisition is explicit
- current-state view derivation is explicit
- service helpers are thin
- behavior coverage remains
- guardrails exist and were tightened for lower fragility

Do not keep editing it for symmetry alone.

## Verification Summary

The final focused verification for this phase passed:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationCurrentState.*Boundary" -count=1
go test ./internal/listingkit -run "TestTaskGenerationCurrentState.*|TestTaskGenerationAction.*|TestGetTaskGenerationTasks.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Those checks are sufficient for this slice because they cover:

- current-state snapshot seam behavior
- current-state view seam behavior
- current-state ownership guardrails
- nearby action/task-read consumers that still depend on the ListingKit generation service
- downstream HTTP/temporal compile-and-test surfaces
