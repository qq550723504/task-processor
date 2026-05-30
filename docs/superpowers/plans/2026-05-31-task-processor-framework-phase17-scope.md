# Task Processor Framework Phase 17 Scope Recommendation

## Candidate Directions

After `Phase 16`, two plausible next directions stand out around the ListingKit action flow:

1. `ListingKit action projection and finalization ownership`
2. `ListingKit layer-temporal action branching ownership`

Both are still in the same neighborhood, but they are not equally urgent.

## Recommendation

The next phase should focus on **`ListingKit action projection and finalization ownership`**.

That means prioritizing:

- [internal/listingkit/task_generation_action_projection.go](/D:/code/task-processor/internal/listingkit/task_generation_action_projection.go:1)

rather than jumping straight to:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:253)

## Why This Is The Right Next Slice

### 1. The upstream prerequisites are now in place

At this point the action flow already has explicit seams for:

- execution
- refresh extraction
- refresh fallback hydration

That means the next ownership hotspot is no longer “how do we run or refresh an action,” but “how do we finalize the response after execution and refresh are done.”

### 2. Projection still mixes several different responsibilities

Today [task_generation_action_projection.go](/D:/code/task-processor/internal/listingkit/task_generation_action_projection.go:19) still jointly decides:

1. how retry vs queue execution results are surfaced
2. how refreshed overview/render previews are attached
3. how the review queue is selected for session assembly
4. how review session and workflow results are built
5. how workflow results are applied back into the session
6. how patch generation and delta-token finalization work
7. how `patch_only` response shaping is enforced

That is now the clearest remaining mixed-responsibility block in this area.

### 3. Temporal branching is narrower right now

`executeLayerTemporalAction(...)` is still a future candidate, but it is currently narrower than the projection/finalization block. The more immediate ownership pressure is not branch count; it is that response-finalization policy still clusters several distinct concerns together.

## Why Not Start With Layer-Temporal Branching

Temporal branching still matters, but it is not the next best slice because:

1. the branch set is relatively compact
2. the bigger regrowth risk now sits in projection/finalization semantics
3. action projection has more direct coupling to the seams extracted in `Phase 10A`, `Phase 15`, and `Phase 16`

So temporal branching should stay a candidate, but not the immediate priority.

## Proposed Phase 17 Shape

The next bounded slice should likely target:

1. review queue selection and session assembly ownership
2. workflow/patch/delta-token finalization ownership
3. `patch_only` response shaping ownership
4. guardrails that stop projection/finalization behavior from drifting back into one implicit mixed block

## Guardrails For The Next Plan

When writing `Phase 17`, keep these constraints:

1. do not reopen `Phase 10A` action execution seams
2. do not reopen `Phase 15/16` current-state and refresh seams
3. do not introduce a generic response-finalization framework
4. keep all new helpers feature-local to ListingKit
5. preserve current action projection behavior first; ownership cleanup second

## Recommended Next Step

Write a `Phase 17` implementation plan for **`ListingKit action projection and finalization ownership`** rather than continuing to polish already-stable refresh seams or jumping straight to temporal-branch symmetry work.
