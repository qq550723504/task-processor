# Task Processor Framework Phase 14 Checkpoint

## Status

`Phase 14` is functionally complete for the intended slice.

This phase was not about reopening queue-read ownership, redesigning generation-task semantics, or introducing a generic task-list framework. The goal was narrower:

1. stop [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:60) from remaining the shared home of generation-task snapshot acquisition and generation-task page shaping at the same time
2. make those responsibilities explicit through feature-local ListingKit seams
3. preserve current filtering, sorting, paging, and summary behavior
4. lock the split with low-fragility boundary guardrails so the old inline read logic does not silently grow back into the service layer

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Generation-task snapshot acquisition now has its own local seam

The shared snapshot seam now lives in:

- [internal/listingkit/task_generation_tasks_read_snapshot.go](/D:/code/task-processor/internal/listingkit/task_generation_tasks_read_snapshot.go:1)

This seam now owns:

- `repo.GetTask(...)`
- `listAssetGenerationTasks(...)`
- handoff of the current `Task`
- handoff of persisted generation tasks

This matters because the original hotspot was not only method length. The real ownership pressure was that current task acquisition and persisted task acquisition stayed mixed with page shaping in one service entry.

### 2. Generation-task page shaping now has its own local seam

The page seam now lives in:

- [internal/listingkit/task_generation_tasks_read_page.go](/D:/code/task-processor/internal/listingkit/task_generation_tasks_read_page.go:1)

This seam now owns:

- empty generation-task page shaping
- `filterGenerationTasks(...)`
- `sortGenerationTasks(...)`
- `paginateGenerationTasks(...)`
- `buildGenerationTaskPage(...)`

This is the main list-shaping ownership split of the phase. The service entry no longer directly owns generation-task filtering, sorting, paging, and final page assembly inline.

### 3. The service read entry is now orchestration-focused

The generation-task read entry still lives in:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:60)

It now mainly coordinates:

1. snapshot seam handoff
2. page seam handoff

It no longer directly owns:

- `repo.GetTask(...)`
- `listAssetGenerationTasks(...)`
- empty page shaping
- filtering, sorting, or paging
- final task-page assembly

That is the main ownership outcome of the phase.

### 4. Guardrails now lock generation-task read ownership boundaries

The ownership protections now live in:

- [internal/listingkit/phase14_generation_tasks_read_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase14_generation_tasks_read_boundary_test.go:1)
- [internal/listingkit/service_generation_tasks_test.go](/D:/code/task-processor/internal/listingkit/service_generation_tasks_test.go:1)
- [internal/listingkit/task_generation_service_test.go](/D:/code/task-processor/internal/listingkit/task_generation_service_test.go:1)

These now protect three things:

1. `GetTaskGenerationTasks(...)` must continue delegating to snapshot/page seams in order
2. snapshot seam must continue owning task/task-list acquisition, but not page shaping
3. page seam must continue owning filtering/sorting/paging/page assembly, but not snapshot acquisition

The current guardrails intentionally lean on helper names, occurrence counts, ordered seam handoff, explicit forbidden calls, and responsibility-level signals. During this phase we explicitly removed a first pass that depended on local variable names so the final boundary tests stayed low-fragility.

## Acceptance Check

`Phase 14` was meant to prove four things:

1. current task acquisition and persisted task acquisition can live behind an explicit ListingKit-owned snapshot seam
2. generation-task page shaping can live behind an explicit ListingKit-owned page seam
3. the service entry can become more orchestration-focused without changing generation-task read behavior
4. the new split can be protected with focused behavior coverage and low-fragility ownership guardrails

All four are now true.

More concretely:

- `task_generation_service.go` no longer owns the mixed generation-task read block
- snapshot and page responsibilities now have separate local homes
- generation-task filtering, sorting, paging, and summary behavior remain covered
- the boundary tests now block the most likely regrowth directions back into the service layer

## What This Phase Did Not Try To Solve

### 1. It did not redesign generation-task business semantics

This phase deliberately did not reopen:

- generation-task classification semantics
- retry-path ownership
- action/navigation ownership
- queue/review read contracts

That was the right tradeoff. The hotspot here was generation-task read ownership, not generation-task business policy.

### 2. It did not introduce a generic task-query framework

All new seams remain local to ListingKit:

- generation-task snapshot acquisition
- generation-task page shaping

That is appropriate here. The pressure was concentrated inside one feature path, not spread across the repo.

### 3. It did not fold unrelated working-tree changes into this slice

The working tree still contains an unrelated change in:

- [data/sensitive_words_shein.json](/D:/code/task-processor/data/sensitive_words_shein.json:1)

This phase intentionally left that alone.

## Residual Responsibilities Still Present

### Generation-task read still depends on stable seam/helper names

The ownership tests intentionally check:

- seam handoff markers
- explicit forbidden helper calls
- focused source-boundary signals

That keeps them lower-fragility than layout-sensitive assertions, but they still depend on stable seam/helper naming. A future rename-only refactor will need test updates.

### Current generation state acquisition is still clustered elsewhere in the same service

`Phase 14` cleaned up the generation-task read entry, but the service still has another live-state acquisition cluster around:

- `executeLayerTemporalAction(...)`
- `getCurrentAssetGenerationOverview(...)`
- `getCurrentAssetGenerationQueue(...)`
- `getCurrentActionRenderPreviews(...)`
- `getCurrentListingKitResult(...)`

Those helpers still look like the next plausible ownership hotspot because they mix current-state acquisition, derived queue/overview shaping, and action/read-side consumers.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “keep carving generation-task read seams for symmetry.” Better next steps are:

### 1. Watch the current generation-state acquisition cluster

If future changes keep landing around:

- current listing result acquisition
- current queue/overview derivation
- action-side preview shaping
- layer-temporal action branching

then the next slice should be driven by that concrete pressure.

### 2. Leave this read layer alone unless a new ownership hotspot appears

This layer is now in a good enough state:

- snapshot acquisition is explicit
- page shaping is explicit
- service orchestration is thin
- behavior coverage remains
- guardrails exist and are lower-fragility after the final cleanup

Do not keep editing it for symmetry alone.

## Verification Summary

The final focused verification for this phase passed:

```powershell
go test ./internal/listingkit -run "TestGetTaskGenerationTasks.*|TestTaskGenerationServiceGetTaskGenerationTasks.*|TestTaskGenerationTasksRead.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Those checks are sufficient for this slice because they cover:

- generation-task read behavior
- snapshot/page seam behavior
- generation-task read ownership guardrails
- downstream HTTP/temporal compile-and-test surfaces that depend on ListingKit
