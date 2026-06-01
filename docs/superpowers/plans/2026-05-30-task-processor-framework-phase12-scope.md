# Task Processor Framework Phase 12 Scope Recommendation

## Recommendation

`Phase 12` should focus on **ListingKit generation review read-response ownership**, not on reopening navigation dispatch planning again.

The highest-value next hotspot is:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:1)
- specifically the read paths rooted at:
  - [GetTaskGenerationReviewSession(...)](/D:/code/task-processor/internal/listingkit/task_generation_service.go:130)
  - [GetTaskGenerationReviewPreview(...)](/D:/code/task-processor/internal/listingkit/task_generation_service.go:173)

In short:

- `Phase 10B` and `Phase 11` made navigation dispatch entry/routing/projection/plan mechanics much clearer
- but the review read paths still directly combine snapshot acquisition, review-session building, conditional-read semantics, response-mode branching, preview projection, and final conditional response shaping inside service methods
- `Phase 12` should target that remaining mixed-ownership read pipeline instead of immediately carving more seams around already-stable navigation plan execution

## Why This Is The Right Next Step

After `Phase 11`, the biggest remaining asymmetry is no longer:

- navigation entry normalization
- primary navigation routing
- executed-plan projection
- plan orchestration
- parallel dedup scheduling
- step execution / sequential backfill

Those are now on explicit feature-owned seams with behavior coverage and boundary guardrails.

The bigger remaining asymmetry is that the review read paths in:

- [internal/listingkit/task_generation_service.go:130](/D:/code/task-processor/internal/listingkit/task_generation_service.go:130)
- [internal/listingkit/task_generation_service.go:173](/D:/code/task-processor/internal/listingkit/task_generation_service.go:173)

still directly decide, in one place:

1. how current result and generation queue are reloaded
2. how the review session snapshot is constructed
3. how delta-token and `If-Match` semantics are evaluated
4. when `NotModified` short-circuits the response
5. how `patch_only` response-mode changes the session response shape
6. how preview viewer / preview target / toolbar / revision status are projected
7. when conditional-state decoration is applied to the final response

That is the next root-cause hotspot because it mixes:

- read-model acquisition
- conditional-read protocol
- response-mode branching
- preview/session projection
- final response shaping

inside two service-side read methods that are adjacent but not identical responsibilities.

The strongest signal is not file size. The stronger signal is that these methods still act as the shared home of both “load the current review snapshot” and “shape the transport response for the caller.”

## Current Hotspot

The main hotspot is:

- [internal/listingkit/task_generation_service.go:130](/D:/code/task-processor/internal/listingkit/task_generation_service.go:130)
- [internal/listingkit/task_generation_service.go:173](/D:/code/task-processor/internal/listingkit/task_generation_service.go:173)

The clearest pressure zone is the block that currently does all of the following inline:

- current result bootstrap via `getCurrentListingKitResult(...)`
- current queue bootstrap via `getCurrentAssetGenerationQueue(...)`
- session snapshot creation via `buildGenerationReviewSession(...)`
- delta-token construction via `buildGenerationReviewReadDeltaToken(...)`
- conditional `NotModified` short-circuit via `isGenerationReviewReadNotModified(...)`
- session `patch_only` branching via `buildGenerationReviewSessionBaseQuery(...)` and `buildGenerationReviewSessionPatch(...)`
- preview projection via `resolveGenerationReviewPreviewResponse(...)`
- preview revision status resolution via `resolveGenerationReviewPreviewRevisionStatus(...)`
- final response decoration via `applyGenerationConditionalStateToReviewSessionResponse(...)` and `applyGenerationConditionalStateToReviewPreviewResponse(...)`

That is a first-order ownership signal, not just tidiness debt.

## Candidate Phase 12 Directions

There are two realistic directions from the current branch state.

### Option 1: Generation review read-response ownership seam

Keep the work feature-owned inside ListingKit and make the review read pipeline more explicit.

This would likely mean:

- separating review-snapshot acquisition from session/preview response assembly
- separating conditional-read evaluation from response-mode-specific shaping
- separating preview projection from service orchestration
- keeping the slice local to `task_generation_service` rather than inventing a generic read framework

**Pros**

- directly targets the densest remaining mixed-responsibility read path adjacent to the navigation work we just finished
- addresses a real service-layer ownership hotspot instead of continuing to polish already-explicit navigation seams
- creates clearer local contracts for session/preview semantics if review UI behavior keeps evolving
- provides a cleaner base for later queue/session/preview consistency work

**Cons**

- needs discipline to avoid turning “review read” into a speculative generic query framework
- easy to overfit to current response DTOs if the split is done mechanically

**Recommendation:** `Yes`

This is the best `Phase 12` target.

### Option 2: Generation queue read-response ownership

Keep working inside:

- [internal/listingkit/task_generation_service.go:75](/D:/code/task-processor/internal/listingkit/task_generation_service.go:75)
- especially `GetTaskGenerationQueue(...)`

This would likely mean:

- separating queue snapshot acquisition from pagination/filtering/sorting response shaping
- separating review-summary attachment from conditional-read response construction
- refining queue-specific read contracts first

**Pros**

- would stay in the same general read-model neighborhood
- could improve clarity for queue-specific pagination and conditional-read behavior

**Cons**

- current pressure there is lower-order than session/preview
- queue reads are more list-shaped and less semantically dense than review session/preview reads
- it would leave the more behaviorally mixed review read path as the stronger remaining service hotspot

**Recommendation:** `Not first`

Do not start there unless a queue-specific pressure point becomes more active.

## Why Not Reopen Navigation Planning First

The navigation-dispatch path now has clearer seams than the review read path:

- entry normalization is isolated
- primary routing is isolated
- projection is isolated
- plan orchestration is isolated
- parallel scheduling is isolated
- step execution is isolated

What remains there is mostly:

- future policy changes
- naming churn
- incremental contract hardening

Those are important, but they are second-order right now.

By contrast, `GetTaskGenerationReviewSession(...)` and `GetTaskGenerationReviewPreview(...)` still mix snapshot acquisition and transport-response shaping, which is the stronger next design signal.

## Suggested Phase 12 Goal

The concrete `Phase 12` goal should be:

> Make ListingKit generation review read ownership more explicit so `task_generation_service.go` stops being the primary shared home of snapshot acquisition, conditional-read handling, and review session/preview response assembly at the same time.

That goal is specific enough to implement incrementally and narrow enough to stay inside one real hotspot.

## Suggested Phase 12 Success Criteria

`Phase 12` should be considered successful when:

1. the review read path no longer mixes snapshot acquisition, conditional-read evaluation, and response shaping inside one undifferentiated service block
2. service orchestration becomes more explicit in `GetTaskGenerationReviewSession(...)` and `GetTaskGenerationReviewPreview(...)`
3. `patch_only`, `NotModified`, preview projection, and revision-status behavior remain unchanged in business terms
4. review read assembly does not silently regrow inline in the service entry during the work
5. no generic read/query framework is introduced unless a second feature shows the same pressure

## Suggested Non-Goals For Phase 12

To keep the next slice disciplined, `Phase 12` should explicitly avoid:

- redesigning review-session or preview business semantics
- reopening navigation dispatch plan mechanics for symmetry alone
- merging queue-read refactoring into the same first slice unless needed for a concrete shared seam
- introducing a generic review-read or conditional-read framework
- moving generation review concerns into HTTP/runtime/bootstrap layers

## Expected File Hotspots

If we take the recommended direction, the first likely hotspots are:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:1)
- [internal/listingkit/generation_review_session.go](/D:/code/task-processor/internal/listingkit/generation_review_session.go:1)
- [internal/listingkit/generation_review_patch.go](/D:/code/task-processor/internal/listingkit/generation_review_patch.go:1)
- [internal/listingkit/generation_conditional_state.go](/D:/code/task-processor/internal/listingkit/generation_conditional_state.go:1)

Possible new files, if the split is warranted, would likely stay feature-local, for example:

- `internal/listingkit/task_generation_review_read_snapshot.go`
- `internal/listingkit/task_generation_review_session_read.go`
- `internal/listingkit/task_generation_review_preview_read.go`
- `internal/listingkit/phase12_generation_review_read_boundary_test.go`

The design pressure should be:

- clearer review read ownership
- explicit handoff between snapshot acquisition and response assembly
- no speculative shared framework

## Recommendation Summary

Proceed to `Phase 12`, but scope it narrowly:

- choose **ListingKit generation review read-response ownership** as the next hotspot
- avoid reopening the more explicit navigation plan layer without new pressure
- keep the work fully feature-owned inside ListingKit

That keeps the next slice aligned with the strongest remaining adjacent ownership signal in the codebase rather than continuing seam cleanup in a path that is already comparatively explicit.
