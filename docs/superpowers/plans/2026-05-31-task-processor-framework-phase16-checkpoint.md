# Task Processor Framework Phase 16 Checkpoint

## Status

`Phase 16` is functionally complete for the intended slice.

This phase was not about reopening current-state acquisition/view seams, redesigning temporal branching, or inventing a generic refresh framework. The goal was narrower:

1. stop [internal/listingkit/task_generation_action_refresh.go](/D:/code/task-processor/internal/listingkit/task_generation_action_refresh.go:1) from remaining the shared home of refreshed current-state extraction and fallback hydration at the same time
2. make those responsibilities explicit through feature-local ListingKit seams
3. preserve current action refresh and fallback/backfill behavior
4. lock the new ownership split with lower-fragility boundary guardrails

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Refresh-state extraction now has its own local seam

The extraction seam now lives in:

- [internal/listingkit/task_generation_action_refresh_extract.go](/D:/code/task-processor/internal/listingkit/task_generation_action_refresh_extract.go:1)

This seam now owns:

- `getCurrentListingKitResult(...)`
- current overview extraction from refreshed state
- current platform render-preview derivation from refreshed state plus query

This matters because the original refresh hotspot was not only method length. The real ownership pressure was that current-state extraction and fallback/backfill policy were still clustered in one block.

### 2. Fallback hydration now has its own local seam

The hydration seam now lives in:

- [internal/listingkit/task_generation_action_refresh_hydration.go](/D:/code/task-processor/internal/listingkit/task_generation_action_refresh_hydration.go:1)

This seam now owns:

- platform preview fallback from `baseResult`
- backfill of `currentResult.PlatformAssetRenderPreviews`
- backfill of `currentResult.AssetRenderPreviews`
- final `taskGenerationActionRefreshResult` assembly

This is the main fallback-policy split of the phase. The refresh orchestrator no longer directly mixes refreshed current-state extraction with fallback/backfill policy.

### 3. The refresh orchestrator is now thinner

The refresh entry still lives in:

- [internal/listingkit/task_generation_action_refresh.go](/D:/code/task-processor/internal/listingkit/task_generation_action_refresh.go:1)

It now mainly coordinates:

1. refresh extraction seam handoff
2. fallback hydration seam handoff

It no longer directly owns:

- current result refresh
- current overview extraction
- current render-preview derivation
- platform preview fallback
- preview backfill mutations
- final refresh-result assembly

That is the main ownership outcome of the phase.

### 4. Guardrails now lock action refresh ownership boundaries

The ownership protections now live in:

- [internal/listingkit/phase16_action_refresh_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase16_action_refresh_boundary_test.go:1)
- [internal/listingkit/service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1)
- [internal/listingkit/phase10_task_generation_action_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase10_task_generation_action_boundary_test.go:1)

These now protect:

1. `taskGenerationActionRefreshPhase.run(...)` must continue delegating extraction before hydration
2. extraction seam must continue owning refreshed current-state extraction, but not fallback/backfill behavior
3. hydration seam must continue owning fallback/backfill behavior, but not current-state reload or refreshed-state derivation
4. the older action boundary suite remains aligned with the new seam split instead of pinning refresh to the old inline shape

During this phase we explicitly tightened a first pass of the boundary tests so the final guardrails rely on helper names, occurrence counts, ordered delegation, and sibling-owner exclusion instead of local variable names or exact statement spelling.

## Acceptance Check

`Phase 16` was meant to prove four things:

1. refreshed current-state extraction can live behind an explicit ListingKit-owned seam
2. fallback hydration can live behind an explicit ListingKit-owned seam
3. the refresh entry can become more orchestration-focused without changing refresh behavior
4. the new split can be protected with focused behavior coverage and lower-fragility ownership guardrails

All four are now true.

More concretely:

- `task_generation_action_refresh.go` no longer owns the old mixed refresh block
- extraction and hydration now have separate local homes
- fallback/backfill priority remains unchanged
- the final boundary tests now block both extraction regrowth and hydration ownership leakage

## What This Phase Did Not Try To Solve

### 1. It did not redesign action projection semantics

This phase deliberately did not reopen:

- review session assembly
- review workflow projection
- review patch/delta token finalization
- patch-only response shaping

That was the right tradeoff. The hotspot here was refresh ownership, not action projection policy.

### 2. It did not redesign layer-temporal branching

This phase also deliberately did not reopen:

- `executeLayerTemporalAction(...)`
- standard/platform-adapt temporal branch semantics
- temporal-only response shaping

That remains a candidate, but not the ownership pressure this slice targeted.

### 3. It did not introduce a generic refresh framework

All new seams remain local to ListingKit:

- action refresh extraction
- action refresh fallback hydration

That is appropriate here. The pressure was concentrated inside one feature path, not spread across the repo.

## Residual Responsibilities Still Present

### Refresh guardrails still depend on stable seam/helper names

The ownership tests intentionally check:

- seam handoff markers
- explicit forbidden helper calls
- sibling-owner exclusion signals

That keeps them lower-fragility than layout-sensitive assertions, but they still depend on stable seam/helper naming. A future rename-only refactor will need test updates.

### Action projection now looks like the next likely ownership hotspot

After `Phase 16`, the most visible remaining mixed block in the action flow is now:

- [internal/listingkit/task_generation_action_projection.go](/D:/code/task-processor/internal/listingkit/task_generation_action_projection.go:1)

That block still combines:

- queue selection for review-session assembly
- review session assembly
- workflow result projection
- workflow application into session
- patch generation
- delta token finalization
- patch-only response shaping

That now looks like the next plausible ownership hotspot because execution, refresh extraction, and refresh hydration are explicit, but projection/finalization policy still clusters several responsibilities together.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “keep polishing refresh seams for symmetry.” Better next steps are:

### 1. Watch the action projection/finalization cluster

If future changes keep landing around:

- `taskGenerationActionProjectionPhase.run(...)`
- review session assembly
- review patch/delta token finalization
- patch-only response shaping

then the next slice should be driven by that concrete pressure.

### 2. Leave the refresh layer alone unless a new hotspot appears

This layer is now in a good enough state:

- refresh extraction is explicit
- fallback hydration is explicit
- refresh orchestration is thin
- behavior coverage remains
- guardrails exist and were tightened for lower fragility

Do not keep editing it for symmetry alone.

## Verification Summary

The final focused verification for this phase passed:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionRefresh.*Boundary" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionRefresh.*|TestTaskGenerationCurrentState.*|TestTaskGenerationAction.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Those checks are sufficient for this slice because they cover:

- refresh extraction seam behavior
- refresh hydration seam behavior
- action refresh ownership guardrails
- nearby current-state/action consumers that still depend on ListingKit action flow
- downstream HTTP/temporal compile-and-test surfaces
