# Task Processor Framework Phase 11 Scope Recommendation

## Recommendation

`Phase 11` should focus on **ListingKit navigation dispatch plan mechanics ownership**, not on reopening the already-stable entry / routing / projection seams.

The highest-value next hotspot is:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:483)
- specifically the plan-execution path rooted at `executeGenerationNavigationDispatchPlan(...)`

In short:

- `Phase 10B` already made navigation entry normalization, primary dispatch routing, and dispatch projection/finalization explicit
- the remaining mixed-ownership pressure now sits lower in the dispatch-plan engine: sequential vs parallel orchestration, deduplication, step execution, and execution-result aggregation are still clustered inside one service-side slice
- `Phase 11` should target that plan-mechanics hotspot instead of reopening the now-clearer service-entry seams

## Why This Is The Right Next Step

After `Phase 10B`, the biggest remaining asymmetry in the navigation flow is no longer:

- entry normalization ownership
- primary dispatch routing ownership
- executed-plan merge ownership
- final dispatch response normalization ownership

Those are now on explicit feature-owned seams with focused behavior tests and boundary guardrails.

The stronger remaining ownership signal is that the plan path in:

- [internal/listingkit/task_generation_service.go:483](/D:/code/task-processor/internal/listingkit/task_generation_service.go:483)

still directly decides, in one local cluster:

1. whether a dispatch plan exists and should execute
2. whether execution runs sequentially or in parallel
3. how duplicate steps are coalesced
4. how step execution results are recorded and counted
5. when stop conditions and skip propagation trigger
6. when fallback / winner / recovery rules are applied back onto the execution result

That is the next root-cause hotspot because it mixes:

- plan orchestration
- deduplication policy
- step execution scheduling
- execution-result aggregation

inside one feature-local execution layer.

The important nuance is that this is now a second-level hotspot, not a top-level service-entry hotspot. That is exactly why it becomes the natural next bounded slice after `Phase 10B`.

## Current Hotspot

The main hotspot is:

- [internal/listingkit/task_generation_service.go:483](/D:/code/task-processor/internal/listingkit/task_generation_service.go:483)

The clearest pressure zone is the block that currently does all of the following:

- dispatch-plan existence / clone / execution object setup  
  [task_generation_service.go:483](/D:/code/task-processor/internal/listingkit/task_generation_service.go:483)
- sequential vs parallel branch selection  
  [task_generation_service.go:496](/D:/code/task-processor/internal/listingkit/task_generation_service.go:496)
- sequential step loop, stop reasoning, skipped-step backfill  
  [task_generation_service.go:505](/D:/code/task-processor/internal/listingkit/task_generation_service.go:505)
- parallel dedupe entry construction, worker fan-out, result reattachment  
  [task_generation_service.go:528](/D:/code/task-processor/internal/listingkit/task_generation_service.go:528)
- step-level queue / preview / session execution  
  [task_generation_service.go:583](/D:/code/task-processor/internal/listingkit/task_generation_service.go:583)

The helper layer around it is already partially explicit:

- [internal/listingkit/service_generation_navigation_dispatch_helpers.go](/D:/code/task-processor/internal/listingkit/service_generation_navigation_dispatch_helpers.go:1)
- [internal/listingkit/generation_navigation_dispatch_rules.go](/D:/code/task-processor/internal/listingkit/generation_navigation_dispatch_rules.go:1)
- [internal/listingkit/generation_navigation_dispatch_merge.go](/D:/code/task-processor/internal/listingkit/generation_navigation_dispatch_merge.go:1)

That means the next slice should not start from scratch. It should use those existing local concepts and finish separating the still-mixed plan engine responsibilities.

## Candidate Phase 11 Directions

There are two realistic directions from the current branch state.

### Option 1: Dispatch plan mechanics ownership seam

Keep the work feature-owned inside ListingKit and make the plan engine more explicit.

This would likely mean:

- separating plan orchestration from step scheduling
- separating deduplication preparation from parallel execution
- separating step execution from execution-result aggregation
- keeping the slice local to navigation dispatch instead of inventing a generic planner framework

**Pros**

- directly targets the densest remaining mixed-responsibility slice in the navigation subsystem
- builds on already-existing local concepts like helpers, rules, merge, and focused result shaping
- gives future dispatch-plan changes a clearer place to land without regrowing back into one service block
- stays aligned with the bounded, feature-owned refactor strategy used in prior phases

**Cons**

- needs discipline to avoid over-splitting the plan engine into too many tiny helpers with weak ownership
- easy to drift into “generic planner” abstraction if the slice is scoped too broadly

**Recommendation:** `Yes`

This is the best `Phase 11` target.

### Option 2: Leave navigation and jump to another nearby ListingKit hotspot

This would mean switching away from navigation plan mechanics to another feature area, such as:

- revisiting task-generation action/read-model behavior
- switching to a submit-path hotspot
- revisiting workflow-side behavior elsewhere in ListingKit

**Pros**

- could reduce fatigue from staying too long in one subsystem
- might help if an entirely new business-pressure hotspot has overtaken navigation

**Cons**

- would leave the strongest remaining navigation ownership cluster unresolved
- risks creating a half-finished architecture story where entry/routing/projection are split, but the plan engine remains the one dense exception

**Recommendation:** `Not yet`

Do not switch away unless a new concrete hotspot has clearly overtaken dispatch-plan mechanics.

## Why Not Reopen The Earlier Navigation Seams

The newly introduced navigation seams are now in a good state:

- entry normalization has its own seam
- primary routing has its own seam
- projection/finalization has its own seam
- guardrails exist for service-entry regrowth

The residual pressure is not there anymore. Reopening those seams now would mostly be symmetry work, not hotspot-driven refactoring.

## Suggested Phase 11 Goal

The concrete `Phase 11` goal should be:

> Make ListingKit navigation dispatch plan ownership more explicit so `executeGenerationNavigationDispatchPlan(...)` stops being the shared home of plan orchestration, deduplication preparation, step scheduling, and execution-result aggregation at the same time.

That goal is specific enough to implement incrementally and narrow enough to stay inside one real hotspot.

## Suggested Phase 11 Success Criteria

`Phase 11` should be considered successful when:

1. plan orchestration no longer mixes sequential/parallel branch selection, dedupe preparation, and result aggregation inside one undifferentiated block
2. step execution remains behaviorally unchanged for queue / preview / session reads
3. stop / skip / dedupe semantics stay unchanged in business terms
4. new seams are protected with focused behavior tests and low-fragility boundary guardrails
5. no generic planner framework is introduced unless a second feature shows the same pressure

## Suggested Non-Goals For Phase 11

To keep the next slice disciplined, `Phase 11` should explicitly avoid:

- redesigning dispatch-plan business policy
- changing fallback / recovery semantics unless a failing test proves a concrete need
- reopening entry / routing / projection seams for symmetry alone
- introducing a repo-wide planner abstraction
- folding submit-path stabilization into the same phase

## Expected File Hotspots

If we take the recommended direction, the first likely hotspots are:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:483)
- [internal/listingkit/service_generation_navigation_dispatch_helpers.go](/D:/code/task-processor/internal/listingkit/service_generation_navigation_dispatch_helpers.go:1)
- [internal/listingkit/generation_navigation_dispatch_rules.go](/D:/code/task-processor/internal/listingkit/generation_navigation_dispatch_rules.go:1)
- [internal/listingkit/service_generation_navigation_dispatch_test.go](/D:/code/task-processor/internal/listingkit/service_generation_navigation_dispatch_test.go:1)

Possible new files, if the split is warranted, would likely stay feature-local, for example:

- `internal/listingkit/task_generation_navigation_dispatch_plan.go`
- `internal/listingkit/task_generation_navigation_dispatch_plan_parallel.go`
- `internal/listingkit/task_generation_navigation_dispatch_step_execution.go`
- `internal/listingkit/phase11_generation_navigation_plan_boundary_test.go`

The design pressure should be:

- clearer dispatch-plan engine ownership
- explicit handoff between orchestration, scheduling, and aggregation
- no speculative shared framework

## Recommendation Summary

Proceed to `Phase 11`, but scope it narrowly:

- choose **ListingKit navigation dispatch plan mechanics ownership** as the next hotspot
- avoid reopening the already-stable entry / routing / projection seams
- avoid switching to unrelated submit-path work in the same slice
- keep the work fully feature-owned inside ListingKit

That keeps the next slice aligned with the strongest remaining adjacent ownership signal in the navigation subsystem rather than drifting into symmetry work or a broader framework abstraction.
