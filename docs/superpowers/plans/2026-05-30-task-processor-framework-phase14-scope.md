# Task Processor Framework Phase 14 Scope Recommendation

## Recommendation

`Phase 14` should focus on **ListingKit generation task read-response ownership**, not on reopening the queue-read seams we just stabilized.

The highest-value next hotspot is:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:60)
- specifically the generation-task read path rooted at `GetTaskGenerationTasks(...)`

In short:

- `Phase 13` made queue snapshot acquisition, queue page shaping, and queue response finalization explicit
- but the generation-task read path still directly combines current task acquisition, generation-task acquisition, filter/sort/page shaping, and final page assembly inside one service method
- `Phase 14` should target that remaining mixed-ownership generation-task read pipeline instead of continuing to polish the now-stable queue-read seams

## Why This Is The Right Next Step

After `Phase 13`, the biggest remaining asymmetry inside `task_generation_service.go` is no longer:

- queue snapshot acquisition
- queue page shaping
- queue response finalization
- review-read ownership

Those are now on explicit feature-owned seams with behavior coverage and boundary guardrails.

The bigger remaining asymmetry is that the generation-task read path in:

- [internal/listingkit/task_generation_service.go:60](/D:/code/task-processor/internal/listingkit/task_generation_service.go:60)

still directly decides, in one place:

1. how the current task snapshot is loaded
2. how persisted generation tasks are loaded
3. how filtering is applied
4. how sorting is applied
5. how pagination is applied
6. how the final generation-task page and summary are assembled

That is the next root-cause hotspot because it still mixes:

- current-state acquisition
- list shaping
- final page assembly

inside one service-side read method.

This is not just “do the symmetric thing after queue-read.” The stronger signal is that `GetTaskGenerationTasks(...)` is still the last prominent read entry in this service layer that acts as both snapshot owner and response-model builder.

## Current Hotspot

The main hotspot is:

- [internal/listingkit/task_generation_service.go:60](/D:/code/task-processor/internal/listingkit/task_generation_service.go:60)

The clearest pressure zone is the block that currently does all of the following inline:

- task bootstrap via `repo.GetTask(...)`
- persisted generation-task acquisition via `listAssetGenerationTasks(...)`
- filter shaping via `filterGenerationTasks(...)`
- sort shaping via `sortGenerationTasks(...)`
- pagination via `paginateGenerationTasks(...)`
- final page assembly via `buildGenerationTaskPage(...)`

That is a first-order ownership signal, not just tidiness debt.

## Candidate Phase 14 Directions

There are two realistic directions from the current branch state.

### Option 1: Generation task read-response ownership seam

Keep the work feature-owned inside ListingKit and make the generation-task read pipeline more explicit.

This would likely mean:

- separating task/task-list snapshot acquisition from task-page shaping
- separating task-page assembly from service orchestration
- keeping the slice local to `task_generation_service` rather than inventing a generic list-read framework

**Pros**

- directly targets the clearest remaining read-path asymmetry adjacent to `Phase 13`
- addresses a real service-layer ownership hotspot instead of re-polishing already explicit queue seams
- reuses the same seam vocabulary already proven on review-read and queue-read slices
- stays away from unrelated submit or studio-session work

**Cons**

- it is a smaller slice than queue-read, so the team has to stay disciplined about not over-engineering it
- easy to over-split into tiny helpers that are mechanically tidy but not ownership-relevant

**Recommendation:** `Yes`

This is the best `Phase 14` target.

### Option 2: Reopen action-side or navigation-side shaping near `taskGenerationService`

There are still adjacent explicit seams around:

- [internal/listingkit/task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:1)
- [internal/listingkit/task_generation_navigation_dispatch_plan.go](/D:/code/task-processor/internal/listingkit/task_generation_navigation_dispatch_plan.go:1)
- [internal/listingkit/task_generation_navigation_dispatch_primary.go](/D:/code/task-processor/internal/listingkit/task_generation_navigation_dispatch_primary.go:1)

**Pros**

- these areas are still architecturally important
- future pressure there could justify another slice

**Cons**

- they already have explicit seams and checkpoints from earlier phases
- current ownership pressure there is weaker than the still-inline `GetTaskGenerationTasks(...)`
- reopening them now would be more about symmetry than about the strongest live hotspot

**Recommendation:** `Not now`

Do not choose this as the next framework slice unless new changes start concentrating there again.

## Why Not Reopen Queue Reads Again

The queue-read path now has clearer seams than the task-read path:

- shared queue snapshot acquisition is isolated
- queue page shaping is isolated
- queue response finalization is isolated
- deferred-only token invalidation is covered
- guardrails block service-layer regrowth

What remains there is mostly:

- future behavior drift
- descriptor contract evolution
- incremental boundary hardening

Those are important, but they are second-order right now.

By contrast, `GetTaskGenerationTasks(...)` still mixes task acquisition and final page shaping inside one service entry, which is the stronger next design signal.

## Suggested Phase 14 Goal

The concrete `Phase 14` goal should be:

> Make ListingKit generation-task read ownership more explicit so `task_generation_service.go` stops being the primary shared home of task snapshot acquisition, generation-task list shaping, and final task-page assembly at the same time.

That goal is specific enough to implement incrementally and narrow enough to stay inside one real hotspot.

## Suggested Phase 14 Success Criteria

`Phase 14` should be considered successful when:

1. the generation-task read path no longer mixes snapshot acquisition and task-page shaping inside one undifferentiated service block
2. generation-task read orchestration becomes more explicit in `GetTaskGenerationTasks(...)`
3. filtering, sorting, paging, and task-summary behavior remain unchanged in business terms
4. task-read assembly does not silently regrow inline in the service entry during the work
5. no generic generation-list or query framework is introduced unless a second feature shows the same pressure

## Suggested Non-Goals For Phase 14

To keep the next slice disciplined, `Phase 14` should explicitly avoid:

- redesigning generation-task business semantics
- reopening queue-read seams for symmetry alone
- touching retry projection or action-side execution paths
- introducing a generic task-list or paging abstraction
- moving generation-task read concerns into HTTP/runtime/bootstrap layers

## Expected File Hotspots

If we take the recommended direction, the first likely hotspots are:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:60)
- [internal/listingkit/generation_task_list.go](/D:/code/task-processor/internal/listingkit/generation_task_list.go:1)
- [internal/listingkit/service_generation_tasks_test.go](/D:/code/task-processor/internal/listingkit/service_generation_tasks_test.go:1)
- [internal/listingkit/task_generation_service_test.go](/D:/code/task-processor/internal/listingkit/task_generation_service_test.go:1)

Possible new files, if the split is warranted, would likely stay feature-local, for example:

- `internal/listingkit/task_generation_tasks_read_snapshot.go`
- `internal/listingkit/task_generation_tasks_read_page.go`
- `internal/listingkit/phase14_generation_tasks_read_boundary_test.go`

The design pressure should be:

- clearer generation-task read ownership
- explicit handoff between snapshot acquisition and task-page shaping
- no speculative shared framework

## Recommendation Summary

Proceed to `Phase 14`, but scope it narrowly:

- choose **ListingKit generation task read-response ownership** as the next hotspot
- avoid reopening the now-stable queue-read seams without new pressure
- avoid adjacent action/navigation areas unless fresh pressure appears there
- keep the work fully feature-owned inside ListingKit

That keeps the next slice aligned with the strongest remaining adjacent ownership signal in the codebase without drifting into symmetry-driven refactoring.
