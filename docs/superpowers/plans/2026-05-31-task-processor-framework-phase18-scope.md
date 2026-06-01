# Task Processor Framework Phase 18 Scope Recommendation

## Candidate Directions

After `Phase 17`, two plausible next directions stand out around the ListingKit action flow:

1. `ExecuteTaskGenerationAction(...)` service-entry orchestration ownership
2. `layer-temporal action branching ownership`

Both are still in the same neighborhood, but they are not equally urgent.

## Recommendation

`Phase 18` should focus on **ListingKit action service-entry orchestration ownership**.

That means prioritizing:

- [internal/listingkit/task_generation_service.go:164](/D:/code/task-processor/internal/listingkit/task_generation_service.go:164)

rather than jumping straight to:

- [internal/listingkit/task_generation_service.go:253](/D:/code/task-processor/internal/listingkit/task_generation_service.go:253)

## Why This Is The Right Next Slice

### 1. The lower action seams are now explicit

At this point the action flow already has explicit seams for:

- execution
- refresh
- projection session assembly
- projection finalization

That changes the ownership picture. The next pressure is no longer “how do we split projection internals,” because `Phase 17` already did that. The next pressure is that the service entry still acts as the shared home of too many orchestration responsibilities above those seams.

### 2. The service entry still mixes several distinct decisions

Today [task_generation_service.go:164](/D:/code/task-processor/internal/listingkit/task_generation_service.go:164) still directly decides, in one method:

1. whether layer-temporal handling short-circuits the action path
2. how current queue/result state is bootstrapped
3. how the target and expected impact are resolved
4. how action audit metadata is constructed
5. when persisted review decisions are durably recorded
6. how projection output is copied back into the service result
7. when conditional-state finalization is applied

That is now the clearest remaining mixed-responsibility block in this area.

The strongest signal is not just method length. The stronger signal is that one entry still owns both pre-execution setup and post-projection finalization even though the middle of the pipeline is now explicitly phased.

### 3. This is the root-cause hotspot, not just leftover glue

The Phase 17 split actually makes the next hotspot easier to see:

- execute work is delegated
- refresh work is delegated
- projection work is delegated

But the service entry still decides how those phases are stitched together, when persistence happens relative to execution, and how the final outward result is assembled. That is the next place where future changes are most likely to regrow mixed ownership.

## Why Not Start With Layer-Temporal Branching

Layer-temporal branching still matters, but it is not the next best slice because:

1. [executeLayerTemporalAction(...)](/D:/code/task-processor/internal/listingkit/task_generation_service.go:253) is already isolated behind a dedicated helper
2. the current branch set is compact compared with the service-entry orchestration above it
3. the bigger regrowth risk now sits where temporal short-circuiting, local execution setup, persistence timing, and response finalization meet

In other words, temporal branching is a narrower secondary candidate. The first-order ownership pressure is still the service entry that surrounds it.

## Proposed Phase 18 Shape

The next bounded slice should likely target:

1. current queue/result bootstrap ownership
2. target resolution and expected-impact ownership
3. action audit construction ownership
4. persisted-review decision timing ownership
5. projection application and conditional-state finalization ownership

The point is not to redesign action business rules. The point is to stop [ExecuteTaskGenerationAction(...)](/D:/code/task-processor/internal/listingkit/task_generation_service.go:164) from remaining the primary shared home of all those orchestration choices at once.

## Guardrails For The Next Plan

When writing `Phase 18`, keep these constraints:

1. do not reopen the Phase 17 session/finalization seam internals
2. do not redesign execute or refresh business behavior
3. do not broaden the slice into HTTP/bootstrap/runtime changes
4. do not introduce a generic action orchestration framework unless another feature shows the same pressure
5. keep new helpers feature-local to ListingKit

## Why This Is Better Than Temporal-First Symmetry

A temporal-first slice would improve symmetry, but it would not reduce the bigger ownership accumulation point that still exists in the service entry. If we skip directly to temporal branching now, we leave the main orchestration hotspot in place and risk future edits continuing to pile audit, persistence, projection copy-back, and final conditional-state behavior into the same method.

That is the key reason `Phase 18` should stay focused on service-entry orchestration first.

## Recommended Next Step

Write a `Phase 18` implementation plan for **ListingKit action service-entry orchestration ownership** rooted at:

- [internal/listingkit/task_generation_service.go:164](/D:/code/task-processor/internal/listingkit/task_generation_service.go:164)

Keep `executeLayerTemporalAction(...)` as a follow-on candidate, not the immediate priority.
