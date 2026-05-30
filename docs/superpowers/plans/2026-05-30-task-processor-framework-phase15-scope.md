# Task Processor Framework Phase 15 Scope Recommendation

## Candidate Directions

After `Phase 14`, two plausible next directions stand out around ListingKit generation state handling:

1. `ListingKit current generation-state acquisition ownership`
2. `ListingKit layer-temporal action branching ownership`

Both sit in or around [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:164), but they are not equally urgent.

## Recommendation

The next phase should focus on **`ListingKit current generation-state acquisition ownership`**.

That means prioritizing the cluster around:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:318)
- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:326)
- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:334)
- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:342)

Specifically:

- `getCurrentAssetGenerationOverview(...)`
- `getCurrentAssetGenerationQueue(...)`
- `getCurrentActionRenderPreviews(...)`
- `getCurrentListingKitResult(...)`

## Why This Is The Right Next Slice

### 1. This is the next real ownership hotspot in the same neighborhood

`Phase 10A` and `Phase 10B` already split:

- action execution branching
- post-action refresh
- action projection
- navigation entry / routing / projection / plan mechanics

`Phase 12` through `Phase 14` then cleaned up the read-side seams around:

- review session / preview reads
- queue reads
- generation-task reads

What remains clustered nearby is not another planner seam. It is the shared “current state” acquisition path that several action/read helpers still consume.

### 2. The pressure is feature-local, not framework-level

This hotspot does **not** justify introducing a generic “current state provider” abstraction.

The actual pressure is narrower:

- load current task/result state
- derive current queue or overview views
- derive current action-side render previews
- reuse those paths across action-side logic without re-growing inline result/queue extraction

That is still a ListingKit-owned concern.

### 3. This slice gets ahead of the next likely regrowth path

If we leave this cluster in place, the most likely regression is:

- new action/read behavior starts calling `getCurrentListingKitResult(...)`
- then derives queue/overview/render previews ad hoc
- then response shaping starts drifting across multiple service helpers again

That is exactly the kind of ownership regrowth the earlier phases were trying to prevent.

## Why Not Start With Layer-Temporal Action Branching

`executeLayerTemporalAction(...)` is still a visible branch point, but it is not the next best slice yet.

Reasons:

1. it is already relatively narrow compared with the current-state helper cluster
2. its branching pressure is easier to evaluate after the shared current-state acquisition path is cleaner
3. right now the more repeated coupling sits in how current result/queue/overview/render-preview state is acquired and reshaped, not in the temporal branch itself

So temporal branching should stay a candidate, but not the immediate priority.

## Proposed Phase 15 Shape

The next bounded slice should likely target:

1. shared current generation-state snapshot/result acquisition
2. queue/overview derivation ownership
3. action-side render-preview derivation ownership
4. boundary guardrails that stop these responsibilities from drifting back into `task_generation_service.go`

## Guardrails For The Next Plan

When writing `Phase 15`, keep these constraints:

1. do not reopen `Phase 10A/10B` action execution or navigation seams
2. do not reopen `Phase 12/13/14` read-side seams
3. do not introduce a generic generation-state framework
4. keep all new helpers feature-local to ListingKit
5. preserve current action/read behavior first; ownership cleanup second

## Recommended Next Step

Write a `Phase 15` implementation plan for **`ListingKit current generation-state acquisition ownership`** rather than continuing to carve the already-stable read seams or jumping straight to temporal-branch symmetry work.
