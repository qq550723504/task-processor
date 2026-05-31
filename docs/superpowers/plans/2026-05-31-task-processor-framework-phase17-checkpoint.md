# Task Processor Framework Phase 17 Checkpoint

## Status

`Phase 17` is functionally complete for the intended slice.

This phase was not about reopening action execution seams, redesigning the service entry, or jumping ahead to layer-temporal branching symmetry. The goal was narrower:

1. extract a feature-local session seam from ListingKit action projection
2. extract a feature-local finalization seam from ListingKit action projection
3. align ownership guardrails with that seam split
4. harden those guardrails so they stop depending on fragile string/brace scanning

That goal is now met.

## What Landed

### 1. Action projection session assembly now has its own seam

The new session seam now lives in:

- [internal/listingkit/task_generation_action_projection_session.go](/D:/code/task-processor/internal/listingkit/task_generation_action_projection_session.go:1)

This landed in commit:

- `5c0e8d06` `refactor: extract listingkit action projection session seam`

This seam now owns:

- current-result selection between base and refreshed state
- review-queue selection from retry vs queue execution output
- review-session assembly via `buildGenerationReviewSession(...)`

This matters because the original projection hotspot was not only method length. The real pressure was that queue/session shaping and finalization policy were still clustered in one projection block.

### 2. Action projection finalization now has its own seam

The new finalization seam now lives in:

- [internal/listingkit/task_generation_action_projection_finalize.go](/D:/code/task-processor/internal/listingkit/task_generation_action_projection_finalize.go:1)

This landed in commit:

- `ad7d497c` `refactor: extract listingkit action projection finalization seam`

This seam now owns:

- workflow result construction
- workflow application into the review session
- review-patch generation
- delta-token finalization
- `patch_only` response shaping

Just as importantly, the finalization seam now accepts only `*GenerationReviewSession`, not the wider session result wrapper. That keeps queue/current-result assembly concerns out of the finalization boundary instead of letting them leak back in through a broader handoff type.

### 3. The projection orchestrator is now thinner

The projection entry still lives in:

- [internal/listingkit/task_generation_action_projection.go](/D:/code/task-processor/internal/listingkit/task_generation_action_projection.go:1)

It now mainly coordinates:

1. baseline response assembly
2. session seam handoff
3. finalization seam handoff

It no longer directly owns:

- review queue selection
- review-session construction
- workflow application
- patch generation
- delta-token finalization
- `patch_only` shaping

That is the main ownership outcome of the phase.

### 4. Boundary alignment and guardrail hardening both landed

The ownership protections now live in:

- [internal/listingkit/phase17_action_projection_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase17_action_projection_boundary_test.go:1)
- [internal/listingkit/phase10_task_generation_action_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase10_task_generation_action_boundary_test.go:1)

This landed in two follow-up commits:

- `cd929b45` `test: lock listingkit action projection boundaries`
- `a82f69d9` `test: harden listingkit action projection guardrails`

Those updates now protect:

1. `taskGenerationActionProjectionPhase.run(...)` must continue delegating session assembly before finalization
2. the session seam must continue owning queue/current-result/session assembly, but not workflow/patch/delta-token finalization
3. the finalization seam must continue owning workflow/patch/delta-token finalization, but not queue/session selection
4. the older action boundary suite remains aligned with the Phase 17 split instead of pinning projection to the old inline shape

The hardening pass matters because the source-boundary helpers were upgraded from fragile string/brace scanning to AST/token-based extraction. That makes the ownership tests less sensitive to incidental formatting while still catching boundary regressions.

## Acceptance Check

`Phase 17` was meant to prove four things:

1. review-session assembly can live behind an explicit ListingKit-owned seam
2. projection finalization can live behind an explicit ListingKit-owned seam
3. the finalization seam can stay narrow by accepting only `*GenerationReviewSession`
4. the seam split can be protected with lower-fragility ownership guardrails

All four are now true.

More concretely:

- [internal/listingkit/task_generation_action_projection.go](/D:/code/task-processor/internal/listingkit/task_generation_action_projection.go:1) no longer contains the old mixed projection/finalization block
- session and finalization now have separate local homes
- finalization no longer depends on the wider session result wrapper
- boundary coverage now checks ownership through AST/token-backed helpers instead of brittle brace counting

That is sufficient to call `Phase 17` functionally complete for its intended slice.

## What This Phase Did Not Try To Solve

### 1. It did not reopen action execution or refresh ownership

This phase deliberately did not reopen:

- [internal/listingkit/task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:1)
- [internal/listingkit/task_generation_action_refresh.go](/D:/code/task-processor/internal/listingkit/task_generation_action_refresh.go:1)

That was the right tradeoff. The hotspot here was projection/finalization ownership, not execution or refresh semantics.

### 2. It did not redesign the service entry

This phase also deliberately did not reopen:

- [internal/listingkit/task_generation_service.go:164](/D:/code/task-processor/internal/listingkit/task_generation_service.go:164)

The service entry still coordinates queue/result bootstrap, target resolution, audit construction, persisted-review timing, projection copy-back, and conditional-state finalization. That remaining orchestration pressure is now more visible precisely because the lower seams are explicit.

### 3. It did not jump to layer-temporal branching

This phase did not reopen:

- [internal/listingkit/task_generation_service.go:253](/D:/code/task-processor/internal/listingkit/task_generation_service.go:253)

That remains a future candidate, but it was not the first-order ownership pressure in this slice.

## Residual Responsibilities Still Present

### ExecuteTaskGenerationAction now looks like the next hotspot

After `Phase 17`, the most visible remaining mixed block is now:

- [internal/listingkit/task_generation_service.go:164](/D:/code/task-processor/internal/listingkit/task_generation_service.go:164)

That entry still directly mixes:

1. current queue/result bootstrap
2. action target resolution and expected-impact derivation
3. action audit construction
4. persisted review-decision timing
5. projection field copy-back into the service result
6. final conditional-state application

The problem is no longer that projection internals are implicit. The remaining problem is that the service entry is still the shared home of both pre-execution setup and post-projection finalization while execute/refresh/projection seams already exist below it.

## What Should Move To The Next Phase

If we continue, the next highest-value work should be:

### 1. Service-entry orchestration ownership

The best next slice is to thin:

- [internal/listingkit/task_generation_service.go:164](/D:/code/task-processor/internal/listingkit/task_generation_service.go:164)

without reopening the already-explicit execute/refresh/projection seams.

### 2. Leave layer-temporal branching alone unless it becomes the real hotspot

`executeLayerTemporalAction(...)` still matters, but its branching is already isolated behind its own early-return helper. After `Phase 17`, the bigger regrowth risk sits in the service entry orchestration above the new seams, not in the temporal helper itself.

## Verification Summary

Fresh verification already passed in the main workspace:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionProjection.*Boundary|TestTaskGenerationActionPhaseOwnershipBoundary|TestTaskGenerationActionProjectionFinalize.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionProjection.*|TestTaskGenerationActionRefresh.*|TestTaskGenerationAction.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Those checks are sufficient for this slice because they cover:

- session seam behavior
- finalization seam behavior
- action projection ownership guardrails
- broader action/refresh integration surfaces that still depend on the ListingKit action pipeline
- downstream HTTP and temporal compile-and-test surfaces
