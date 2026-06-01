# Task Processor Framework Phase 10B Scope Recommendation

## Recommendation

`Phase 10B` should focus on **ListingKit task-generation navigation dispatch ownership**, not on reopening task-generation action seams or jumping sideways into the current submit-path noise.

The highest-value next hotspot is:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:350)
- specifically the navigation path rooted at `DispatchTaskGenerationNavigation(...)`

In short:

- `Phase 10A` already made action execution, refresh, and projection explicit
- the adjacent navigation path still mixes request normalization, primary dispatch selection, optional plan execution, and final response shaping in one service entry
- `Phase 10B` should target that remaining mixed-ownership navigation pipeline instead of immediately reopening deeper mutation seams or chasing the unrelated submit-path churn currently present in the working tree

## Why This Is The Right Next Step

After `Phase 10A`, the biggest remaining asymmetry near task generation is no longer:

- action execution branching
- post-action refresh ownership
- action result projection ownership
- action-side guardrail coverage

Those are now on explicit feature-owned seams with focused behavior tests and source-boundary protections.

The stronger remaining ownership signal is that the navigation path in:

- [internal/listingkit/task_generation_service.go:350](/D:/code/task-processor/internal/listingkit/task_generation_service.go:350)

still directly decides, in one place:

1. how the incoming navigation target is cloned and normalized
2. how response mode and plan mode are derived
3. whether the primary dispatch should route to action, preview, queue, or session
4. when optional dispatch-plan execution should run
5. how executed-plan results should be folded back into the primary response
6. when the final response should be normalized and returned

That is the next root-cause hotspot because it mixes:

- dispatch entry orchestration
- primary route selection
- plan execution handoff
- response assembly/finalization

inside one service-side execution path.

The strongest signal is not file size. The stronger signal is that one method is still the shared home of both “decide what navigation path to take” and “shape the final dispatch response after optional plan execution.”

## Current Hotspot

The main hotspot is:

- [internal/listingkit/task_generation_service.go:350](/D:/code/task-processor/internal/listingkit/task_generation_service.go:350)

The clearest pressure zone is the block that currently does all of the following inline:

- navigation target validation and clone/baseline setup  
  [task_generation_service.go:351](/D:/code/task-processor/internal/listingkit/task_generation_service.go:351)
- response-mode and plan-mode normalization  
  [task_generation_service.go:357](/D:/code/task-processor/internal/listingkit/task_generation_service.go:357)
- primary dispatch handoff  
  [task_generation_service.go:359](/D:/code/task-processor/internal/listingkit/task_generation_service.go:359)
- optional plan execution and response merge  
  [task_generation_service.go:364](/D:/code/task-processor/internal/listingkit/task_generation_service.go:364)
- final response normalization  
  [task_generation_service.go:371](/D:/code/task-processor/internal/listingkit/task_generation_service.go:371)

The adjacent ownership pressure sits in:

- [internal/listingkit/task_generation_service.go:455](/D:/code/task-processor/internal/listingkit/task_generation_service.go:455) `dispatchGenerationNavigationPrimary(...)`
- [internal/listingkit/task_generation_service.go:517](/D:/code/task-processor/internal/listingkit/task_generation_service.go:517) `executeGenerationNavigationDispatchPlan(...)`
- [internal/listingkit/task_generation_service.go:593](/D:/code/task-processor/internal/listingkit/task_generation_service.go:593) `executeGenerationNavigationDispatchPlanStep(...)`

The important nuance is that plan execution and step execution are already somewhat explicit. The remaining first-order hotspot is higher up: the service entry and primary dispatch path still carry too much mixed routing and response-shaping responsibility.

## Candidate Phase 10B Directions

There are two realistic directions from the current branch state.

### Option 1: Navigation dispatch ownership seam

Keep the work feature-owned inside ListingKit and make the navigation pipeline more explicit.

This would likely mean:

- separating entry normalization from primary dispatch orchestration
- separating primary dispatch route selection from dispatch-response assembly
- separating optional plan execution merge from final response normalization
- keeping the slice local to task-generation navigation instead of inventing a generic dispatch framework

**Pros**

- directly targets the densest remaining mixed-responsibility path adjacent to `Phase 10A`
- builds on already-existing local concepts like primary dispatch, plan execution, and finalization instead of creating new abstractions
- gives navigation response assembly a clearer boundary if preview/session/queue variants keep evolving
- leaves deeper step-level concurrency and dedupe semantics alone unless they become the real hotspot later

**Cons**

- needs discipline to avoid over-abstracting action/preview/queue/session routing into a speculative generic dispatcher
- easy to split too mechanically if response assembly and route selection are not kept as separate responsibilities

**Recommendation:** `Yes`

This is the best `Phase 10B` target.

### Option 2: Reopen submit-path or deeper dispatch-plan mechanics first

This would mean choosing one of these instead:

- pivot to the currently dirty submit-path files under `internal/listingkit/service_submit*.go` and `internal/listingkit/task_submission*.go`
- or dive past the entry hotspot into `executeGenerationNavigationDispatchPlan(...)` parallelism, dedupe, and step execution semantics

**Pros**

- submit-path work would reduce broad test noise once that line is ready to stabilize
- deeper plan-mechanics work could improve explicitness around dedupe/concurrency behavior

**Cons**

- submit-path changes in the working tree are currently separate in-flight work, not the clearest next framework-ownership slice
- deeper plan mechanics are second-order right now because `executeGenerationNavigationDispatchPlan(...)` and step execution already have visible local concepts
- both choices would skip the more meaningful remaining service-entry hotspot

**Recommendation:** `Not yet`

Do not lead with either of these until the navigation entry/primary-dispatch seam is clearer.

## Why Not Reopen Submit Or Plan Parallelism First

The submit-path issue is currently real, but it is not the next framework-evolution slice.

What we see in the working tree today is:

- [internal/listingkit/service_submit.go](/D:/code/task-processor/internal/listingkit/service_submit.go:1)
- [internal/listingkit/task_submission_execution_service.go](/D:/code/task-processor/internal/listingkit/task_submission_execution_service.go:1)
- related submit and publishing files

That matters operationally for broad green tests, but it is a different line of change. It should not be silently folded into the next framework-ownership phase just because it is nearby.

Likewise, navigation dispatch plan execution already has clearer internal concepts:

- plan cloning
- sequential vs parallel execution
- deduplication keys
- per-step execution
- execution stats and stop rules

What remains there is mostly:

- contract explicitness
- future-proofing around concurrency/dedupe policy

Those are important, but they are second-order compared with the still-mixed entry/primary-dispatch ownership.

## Suggested Phase 10B Goal

The concrete `Phase 10B` goal should be:

> Make ListingKit task-generation navigation dispatch ownership more explicit so `DispatchTaskGenerationNavigation(...)` stops being the primary shared home of navigation normalization, primary dispatch routing, optional plan merge, and final response shaping at the same time.

That goal is specific enough to implement incrementally and narrow enough to stay inside one real hotspot.

## Suggested Phase 10B Success Criteria

`Phase 10B` should be considered successful when:

1. the navigation path no longer mixes entry normalization, primary dispatch, and final response shaping inside one undifferentiated service block
2. navigation orchestration becomes more explicit in `task_generation_service.go`
3. action/preview/queue/session routing behavior stays unchanged in business terms
4. executed-plan merge and final response normalization do not silently regrow inline in the service entry during the work
5. no generic dispatch framework is introduced unless a second feature shows the same pressure

## Suggested Non-Goals For Phase 10B

To keep the next slice disciplined, `Phase 10B` should explicitly avoid:

- redesigning navigation business policy
- reopening `Phase 10A` action seams for symmetry alone
- changing dedupe or concurrency semantics in dispatch-plan execution unless a failing test proves a concrete need
- pulling submit-path stabilization into the same phase
- introducing a generic navigation or dispatch orchestration framework

## Expected File Hotspots

If we take the recommended direction, the first likely hotspots are:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:350)
- [internal/listingkit/service_generation_navigation_dispatch.go](/D:/code/task-processor/internal/listingkit/service_generation_navigation_dispatch.go:1)
- any existing helper files already carrying navigation dispatch support logic

Possible new files, if the split is warranted, would likely stay feature-local, for example:

- `internal/listingkit/task_generation_navigation_dispatch_entry.go`
- `internal/listingkit/task_generation_navigation_dispatch_primary.go`
- `internal/listingkit/task_generation_navigation_dispatch_projection.go`
- `internal/listingkit/phase10b_generation_navigation_boundary_test.go`

The design pressure should be:

- clearer navigation dispatch ownership
- explicit handoff between primary routing and response shaping
- no speculative shared framework

## Recommendation Summary

Proceed to `Phase 10B`, but scope it narrowly:

- choose **ListingKit task-generation navigation dispatch ownership** as the next hotspot
- avoid folding in the current submit-path churn
- avoid diving into deeper dispatch-plan concurrency/dedupe mechanics without new pressure
- keep the work fully feature-owned inside ListingKit

That keeps the next slice aligned with the strongest remaining adjacent ownership signal in the codebase rather than mixing framework evolution with a separate submit-path stabilization line.
