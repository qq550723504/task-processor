# Task Processor Framework Phase 13 Scope Recommendation

## Recommendation

`Phase 13` should focus on **ListingKit generation queue read-response ownership**, not on reopening the review-read seams we just stabilized.

The highest-value next hotspot is:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:75)
- specifically the queue-read path rooted at `GetTaskGenerationQueue(...)`

In short:

- `Phase 12` made review snapshot acquisition, session read shaping, and preview read shaping explicit
- but the queue-read path still directly combines current-state acquisition, filter/sort/page shaping, review-summary attachment, delta-token construction, conditional-read short-circuit, and final conditional response decoration inside one service method
- `Phase 13` should target that remaining mixed-ownership queue-read pipeline instead of continuing to polish the now-stable review-read seams

## Why This Is The Right Next Step

After `Phase 12`, the biggest remaining asymmetry is no longer:

- review snapshot acquisition
- review session response shaping
- review preview response shaping
- navigation dispatch plan mechanics

Those are now on explicit feature-owned seams with behavior coverage and boundary guardrails.

The bigger remaining asymmetry is that the queue-read path in:

- [internal/listingkit/task_generation_service.go:75](/D:/code/task-processor/internal/listingkit/task_generation_service.go:75)

still directly decides, in one place:

1. how current task/result/review state is loaded
2. how queue emptiness is handled
3. how filter / sort / pagination are applied
4. how review summary is attached back to the page
5. how delta-token is built
6. when `NotModified` short-circuits the response
7. how conditional metadata and recovery/action summaries are applied to the final response

That is the next root-cause hotspot because it mixes:

- queue snapshot acquisition
- list-response shaping
- conditional-read protocol
- final response decoration

inside one service-side read method.

The strongest signal is not file size. The stronger signal is that one method still acts as the shared home of both “build the queue read model” and “shape the transport response for the caller.”

## Current Hotspot

The main hotspot is:

- [internal/listingkit/task_generation_service.go:75](/D:/code/task-processor/internal/listingkit/task_generation_service.go:75)

The clearest pressure zone is the block that currently does all of the following inline:

- current task/result/review bootstrap via `repo.GetTask(...)`, `listAssetGenerationTasks(...)`, and `listGenerationReviews(...)`
- review-decorated result construction via `withListingKitResultGenerationAndReview(...)`
- queue extraction and empty-response shaping
- filter / sort / pagination through queue-specific helpers
- review-summary attachment via `attachReviewSummaryToGenerationQueuePage(...)`
- delta-token construction via `buildGenerationQueueDeltaToken(...)`
- conditional `NotModified` short-circuit via `isGenerationReviewReadNotModified(...)`
- final response decoration via `applyGenerationConditionalStateToQueuePage(...)`

That is a first-order ownership signal, not just tidiness debt.

## Candidate Phase 13 Directions

There are two realistic directions from the current branch state.

### Option 1: Generation queue read-response ownership seam

Keep the work feature-owned inside ListingKit and make the queue-read pipeline more explicit.

This would likely mean:

- separating queue snapshot acquisition from queue response shaping
- separating queue page assembly from conditional-read short-circuit
- separating queue conditional decoration from service orchestration
- keeping the slice local to `task_generation_service` rather than inventing a generic list-read framework

**Pros**

- directly targets the densest remaining read-path ownership hotspot adjacent to the work just completed
- addresses a real service-layer asymmetry instead of re-polishing already explicit review-read seams
- creates a cleaner base for future queue summary / review-summary / conditional-read changes
- stays away from the currently dirty `studio session` worktree area

**Cons**

- needs discipline to avoid turning queue-read helpers into a speculative generic paging framework
- easy to over-split into filter/sort/page seams that are mechanically tidy but not ownership-relevant

**Recommendation:** `Yes`

This is the best `Phase 13` target.

### Option 2: Studio session ownership / API shaping

There is visible ongoing activity in:

- `internal/listingkit/studio_session_*`
- `internal/listingkit/api/studio_sessions_handler.go`
- `web/listingkit-ui/src/components/listingkit/shein-studio/*`

**Pros**

- there may be real active design pressure there
- it could be a meaningful future hotspot

**Cons**

- the current worktree already contains many unrelated in-progress changes there
- touching that area now would risk colliding with active work and muddying this framework line
- there is no need to mix this framework phase with the user’s other studio-session edits

**Recommendation:** `Not now`

Do not choose this as the next framework slice while the worktree is visibly active there.

## Why Not Reopen Review Reads Again

The review-read path now has clearer seams than the queue-read path:

- shared snapshot acquisition is isolated
- session read shaping is isolated
- preview read shaping is isolated

What remains there is mostly:

- future behavior drift
- boundary hardening
- incremental test strengthening

Those are important, but they are second-order right now.

By contrast, `GetTaskGenerationQueue(...)` still mixes queue snapshot building and final response shaping, which is the stronger next design signal.

## Suggested Phase 13 Goal

The concrete `Phase 13` goal should be:

> Make ListingKit generation queue read ownership more explicit so `task_generation_service.go` stops being the primary shared home of queue snapshot acquisition, queue page shaping, conditional-read handling, and final queue response decoration at the same time.

That goal is specific enough to implement incrementally and narrow enough to stay inside one real hotspot.

## Suggested Phase 13 Success Criteria

`Phase 13` should be considered successful when:

1. the queue-read path no longer mixes snapshot acquisition, queue page shaping, and final conditional decoration inside one undifferentiated service block
2. queue read orchestration becomes more explicit in `GetTaskGenerationQueue(...)`
3. queue filtering/sorting/paging behavior, review-summary attachment, and `NotModified` semantics remain unchanged in business terms
4. queue read assembly does not silently regrow inline in the service entry during the work
5. no generic list-read or query framework is introduced unless a second feature shows the same pressure

## Suggested Non-Goals For Phase 13

To keep the next slice disciplined, `Phase 13` should explicitly avoid:

- redesigning queue business semantics
- reopening the review-read seams for symmetry alone
- mixing studio-session work into the same slice
- introducing a generic paging/filtering framework
- moving queue-read concerns into HTTP/runtime/bootstrap layers

## Expected File Hotspots

If we take the recommended direction, the first likely hotspots are:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:75)
- [internal/listingkit/generation_queue_list.go](/D:/code/task-processor/internal/listingkit/generation_queue_list.go:1)
- [internal/listingkit/generation_conditional_state.go](/D:/code/task-processor/internal/listingkit/generation_conditional_state.go:1)
- [internal/listingkit/service_generation_queue_test.go](/D:/code/task-processor/internal/listingkit/service_generation_queue_test.go:1)

Possible new files, if the split is warranted, would likely stay feature-local, for example:

- `internal/listingkit/task_generation_queue_read_snapshot.go`
- `internal/listingkit/task_generation_queue_read_page.go`
- `internal/listingkit/task_generation_queue_read_response.go`
- `internal/listingkit/phase13_generation_queue_read_boundary_test.go`

The design pressure should be:

- clearer queue-read ownership
- explicit handoff between snapshot acquisition and response shaping
- no speculative shared framework

## Recommendation Summary

Proceed to `Phase 13`, but scope it narrowly:

- choose **ListingKit generation queue read-response ownership** as the next hotspot
- avoid reopening the now-stable review-read seams without new pressure
- avoid the currently dirty studio-session area
- keep the work fully feature-owned inside ListingKit

That keeps the next slice aligned with the strongest remaining adjacent ownership signal in the codebase while respecting the current worktree context.
