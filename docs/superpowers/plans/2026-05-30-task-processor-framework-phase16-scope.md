# Task Processor Framework Phase 16 Scope Recommendation

## Candidate Directions

After `Phase 15`, two plausible next directions stand out around ListingKit generation flow:

1. `ListingKit action refresh fallback hydration ownership`
2. `ListingKit layer-temporal action branching ownership`

Both sit near the action path, but they are not equally urgent.

## Recommendation

The next phase should focus on **`ListingKit action refresh fallback hydration ownership`**.

That means prioritizing:

- [internal/listingkit/task_generation_action_refresh.go](/D:/code/task-processor/internal/listingkit/task_generation_action_refresh.go:1)

rather than jumping straight to:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:253)

## Why This Is The Right Next Slice

### 1. The current-state prerequisites are now in place

`Phase 15` already split:

- current state acquisition
- current queue/overview/render-preview derivation

That means the next ownership hotspot is no longer “how do we load current state,” but “how do we post-process and hydrate that state for action responses.”

### 2. Refresh still mixes several different responsibilities

Today [task_generation_action_refresh.go](/D:/code/task-processor/internal/listingkit/task_generation_action_refresh.go:19) still jointly decides:

1. how current result is refreshed
2. how current overview is extracted
3. how action-side render previews are derived
4. when render previews fall back to `baseResult`
5. when `currentResult.PlatformAssetRenderPreviews` is mutated
6. when `currentResult.AssetRenderPreviews` is backfilled from `baseResult`

That is now the clearest remaining mixed-responsibility block in this area.

### 3. Temporal branching is narrower right now

`executeLayerTemporalAction(...)` is still a valid future candidate, but at the moment it is relatively compact compared with the refresh/fallback hydration block. The more immediate ownership pressure is not “how many branches exist,” but “where refresh, fallback, and current-result mutation semantics live.”

## Why Not Start With Layer-Temporal Branching

The temporal branch still matters, but it is not the next best slice because:

1. its current shape is still relatively narrow
2. the bigger regrowth risk now sits in refresh-side mutation and fallback policy
3. action refresh has more direct coupling to the newly extracted current-state seams

So temporal branching should stay a candidate, but not the immediate priority.

## Proposed Phase 16 Shape

The next bounded slice should likely target:

1. refreshed current result acquisition handoff
2. overview/render-preview extraction ownership inside refresh
3. fallback hydration ownership for `PlatformAssetRenderPreviews` and `AssetRenderPreviews`
4. boundary guardrails that stop refresh-side shaping from drifting back into one implicit mixed block

## Guardrails For The Next Plan

When writing `Phase 16`, keep these constraints:

1. do not reopen `Phase 10A` action execution seams
2. do not reopen `Phase 15` current-state snapshot/view seams
3. do not introduce a generic refresh framework
4. keep all new helpers feature-local to ListingKit
5. preserve current action refresh behavior first; ownership cleanup second

## Recommended Next Step

Write a `Phase 16` implementation plan for **`ListingKit action refresh fallback hydration ownership`** rather than continuing to polish already-stable current-state seams or jumping straight to temporal-branch symmetry work.
