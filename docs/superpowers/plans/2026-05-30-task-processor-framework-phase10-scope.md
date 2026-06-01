# Task Processor Framework Phase 10 Scope Recommendation

## Recommendation

`Phase 10` should focus on **ListingKit task-generation action execution and response assembly ownership**, not on reopening generation navigation planning yet.

The highest-value next hotspot is:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:1)
- specifically the action path rooted at `ExecuteTaskGenerationAction(...)`

In short:

- `Phase 9A` made the retry path much clearer
- but the action-execution path still directly combines action resolution, execution branching, persistence side effects, read-model refresh, preview fallback hydration, and review-patch/result assembly in one service method
- `Phase 10` should target that remaining mixed-ownership action pipeline instead of immediately diving into generation navigation plan mechanics

## Why This Is The Right Next Step

After `Phase 9A`, the biggest remaining asymmetry is no longer:

- retry mutation ownership
- retry persistence ownership
- retry projection correctness under nil-dispatch or nil-request edges
- retry-boundary protection

Those are now on explicit feature-owned seams with behavior and source-boundary guardrails.

The biggest remaining asymmetry is that the action path in:

- [internal/listingkit/task_generation_service.go:281](/D:/code/task-processor/internal/listingkit/task_generation_service.go:281)

still directly decides, in one place:

1. whether the action is handled by a temporal side-entry or by local generation state
2. how queue/result state is loaded before action execution
3. how the resolved action target and expected impact are derived
4. how retryable vs non-retryable execution branches run
5. when generation review decisions are durably persisted
6. how overview, render previews, and current result are reloaded after action execution
7. when platform render previews fall back to base result state
8. how review session, workflow summary, patch, and delta token are assembled

That is the next root-cause hotspot because it mixes:

- action execution branching
- persistence timing
- read-state refresh
- preview hydration
- response projection

inside one service-side execution path.

The strongest signal is not file size. The stronger signal is that one method still acts as the shared home of both “do the action” and “shape the response model after the action,” which are adjacent but not identical responsibilities.

## Current Hotspot

The main hotspot is:

- [internal/listingkit/task_generation_service.go:281](/D:/code/task-processor/internal/listingkit/task_generation_service.go:281)

The clearest pressure zone is the block that currently does all of the following inline:

- temporal short-circuit via `executeLayerTemporalAction(...)`  
  [task_generation_service.go:282](/D:/code/task-processor/internal/listingkit/task_generation_service.go:282)
- queue/result bootstrap and target resolution  
  [task_generation_service.go:285](/D:/code/task-processor/internal/listingkit/task_generation_service.go:285)
- retryable vs queue-path execution branching  
  [task_generation_service.go:315](/D:/code/task-processor/internal/listingkit/task_generation_service.go:315)
- optional persisted review decision handoff  
  [task_generation_service.go:329](/D:/code/task-processor/internal/listingkit/task_generation_service.go:329)
- overview and preview refresh  
  [task_generation_service.go:341](/D:/code/task-processor/internal/listingkit/task_generation_service.go:341)
- current-result fallback hydration for render previews  
  [task_generation_service.go:352](/D:/code/task-processor/internal/listingkit/task_generation_service.go:352)
- review session, workflow result, review patch, and delta-token assembly  
  [task_generation_service.go:362](/D:/code/task-processor/internal/listingkit/task_generation_service.go:362)

That is a first-order ownership signal, not just tidiness debt.

## Candidate Phase 10 Directions

There are two realistic directions from the current branch state.

### Option 1: Task-generation action execution and response assembly seam

Keep the work feature-owned inside ListingKit and make the action pipeline more explicit.

This would likely mean:

- separating action execution branching from post-execution response shaping
- separating persisted review-decision handoff from read-model refresh
- separating preview/review-session/result projection from service orchestration
- keeping the slice local to `task_generation_service` rather than inventing a generic action framework

**Pros**

- directly targets the densest remaining mixed-responsibility path adjacent to the retry work we just finished
- addresses a real service-layer ownership hotspot instead of pushing deeper into a planner path that is already more orchestration-shaped
- gives action execution and response assembly clearer local contracts if generation actions keep evolving
- creates a better boundary for future review of review-session and preview semantics

**Cons**

- needs discipline to avoid over-abstracting queue-only and retryable branches into a speculative generic executor
- easy to overfit to current response fields if the seam is split too mechanically

**Recommendation:** `Yes`

This is the best `Phase 10` target.

### Option 2: Generation navigation dispatch plan mechanics

Keep working inside:

- [internal/listingkit/task_generation_service.go:385](/D:/code/task-processor/internal/listingkit/task_generation_service.go:385)
- especially `DispatchTaskGenerationNavigation(...)` and `executeGenerationNavigationDispatchPlan(...)`

This would likely mean:

- revisiting deduplication and plan-step concurrency behavior
- refining queue/session/preview dispatch plan contracts
- further separating plan execution from plan result aggregation

**Pros**

- would keep work in a nearby area that already has explicit plan semantics
- might improve clarity for navigation-specific caching and deduplication behavior

**Cons**

- current pressure there is second-order, not first-order
- the navigation path is already more explicitly orchestration-oriented than `ExecuteTaskGenerationAction(...)`
- it would leave the action path as the more meaningful remaining service hotspot

**Recommendation:** `Not yet`

Do not continue there unless a new navigation-specific pressure point appears.

## Why Not Reopen Navigation Planning First

The navigation-dispatch path already has clearer seams than the action path:

- primary dispatch is isolated
- plan execution is isolated
- step execution is isolated
- deduplication and response finalization are already visible as local concepts

What remains there is mostly:

- contract explicitness
- concurrency/dedup semantics
- future-proofing

Those are important, but they are second-order right now.

By contrast, `ExecuteTaskGenerationAction(...)` still mixes action execution and post-action response shaping, which is the stronger next design signal.

## Suggested Phase 10 Goal

The concrete `Phase 10` goal should be:

> Make ListingKit task-generation action execution ownership more explicit so `task_generation_service.go` stops being the primary shared home of action branching, post-action refresh, and review/preview response assembly at the same time.

That goal is specific enough to implement incrementally and narrow enough to stay inside one real hotspot.

## Suggested Phase 10 Success Criteria

`Phase 10` should be considered successful when:

1. the action path no longer mixes execution branching, refresh, and response projection inside one undifferentiated service block
2. action orchestration becomes more explicit in `task_generation_service.go`
3. behavior tests still protect retryable and queue-only action semantics unchanged in business terms
4. preview/review-session assembly does not silently regrow inline in the service entry during the work
5. no generic action or navigation execution framework is introduced unless a second feature shows the same pressure

## Suggested Non-Goals For Phase 10

To keep the next slice disciplined, `Phase 10` should explicitly avoid:

- redesigning generation action business policy
- changing navigation dispatch behavior for symmetry alone
- reopening retry mutation/persistence/projection seams
- introducing a generic action orchestration framework
- moving generation action concerns into HTTP/runtime/bootstrap layers

## Expected File Hotspots

If we take the recommended direction, the first likely hotspots are:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:1)
- [internal/listingkit/generation_review_session.go](/D:/code/task-processor/internal/listingkit/generation_review_session.go:1)
- [internal/listingkit/generation_review_patch.go](/D:/code/task-processor/internal/listingkit/generation_review_patch.go:1)
- [internal/listingkit/generation_review_workflow.go](/D:/code/task-processor/internal/listingkit/generation_review_workflow.go:1)

Possible new files, if the split is warranted, would likely stay feature-local, for example:

- `internal/listingkit/task_generation_action_execute.go`
- `internal/listingkit/task_generation_action_refresh.go`
- `internal/listingkit/task_generation_action_projection.go`
- `internal/listingkit/phase10_task_generation_action_boundary_test.go`

The design pressure should be:

- clearer action execution ownership
- explicit handoff between action branch execution and response assembly
- no speculative shared framework

## Recommendation Summary

Proceed to `Phase 10`, but scope it narrowly:

- choose **ListingKit task-generation action execution and response assembly ownership** as the next hotspot
- avoid reopening the more orchestration-shaped navigation planning path without new pressure
- keep the work fully feature-owned inside ListingKit

That keeps the next slice aligned with the strongest remaining adjacent ownership signal in the codebase rather than continuing seam cleanup in a path that is already comparatively explicit.
