# Task Processor Framework Phase 13 Checkpoint

## Status

`Phase 13` is functionally complete for the intended slice.

This phase was not about redesigning ListingKit generation semantics, reopening review-read ownership, or introducing a generic queue/query framework. The goal was narrower:

1. stop [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:75) from remaining the shared home of queue snapshot acquisition, queue page shaping, conditional-read handling, and final queue response decoration at the same time
2. make those queue-read responsibilities explicit through feature-local seams
3. preserve current filtering, sorting, paging, review-summary, `NotModified`, and conditional-response behavior
4. lock the new ownership split so the old inline queue-read logic does not silently grow back into the service layer

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Shared queue snapshot acquisition now has its own local seam

The shared snapshot seam now lives in:

- [internal/listingkit/task_generation_queue_read_snapshot.go](/D:/code/task-processor/internal/listingkit/task_generation_queue_read_snapshot.go:1)

This seam now owns:

- `repo.GetTask(...)`
- `listAssetGenerationTasks(...)`
- `listGenerationReviews(...)`
- `withListingKitResultGenerationAndReview(...)`
- handoff of the decorated `ListingKitResult`
- handoff of the decorated `AssetGenerationQueue`

This matters because the original hotspot was not just method length. The real risk was that current task/result/review acquisition stayed tangled with queue-response shaping in one service entry.

### 2. Queue page shaping now has its own local seam

The queue-page seam now lives in:

- [internal/listingkit/task_generation_queue_read_page.go](/D:/code/task-processor/internal/listingkit/task_generation_queue_read_page.go:1)

This seam now owns:

- empty queue page shaping
- `filterGenerationQueueItems(...)`
- `sortGenerationQueueItems(...)`
- `paginateGenerationQueueItems(...)`
- `buildGenerationQueuePage(...)`
- `attachReviewSummaryToGenerationQueuePage(...)`

This is the main list-shaping ownership split of the phase. The service entry no longer directly owns page construction and review-summary attachment inline.

### 3. Queue response finalization now has its own local seam

The queue-response seam now lives in:

- [internal/listingkit/task_generation_queue_read_response.go](/D:/code/task-processor/internal/listingkit/task_generation_queue_read_response.go:1)

This seam now owns:

- `buildGenerationQueueDeltaToken(...)`
- `isGenerationReviewReadNotModified(...)`
- `applyGenerationConditionalStateToQueuePage(...)`
- the final not-modified queue response shape

This is the main transport-facing ownership split of the phase. The service entry no longer directly mixes page assembly with delta-token and conditional-response logic.

### 4. Queue delta-token semantics are now aligned with exposed review-summary fields

The queue delta-token behavior was tightened in:

- [internal/listingkit/generation_review_delta.go](/D:/code/task-processor/internal/listingkit/generation_review_delta.go:64)

The important outcome is:

- `DeferredSections` is now part of the queue summary signature used by `buildGenerationQueueDeltaToken(...)`

That fix matters because `DeferredSections` was already exposed in the final queue summary. Before this change, a deferred-only review change could leave the old queue token valid and incorrectly hit `not_modified`.

### 5. The service queue-read entry is now orchestration-focused

The queue-read service entry still lives in:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:75)

It now mainly coordinates:

1. queue snapshot seam handoff
2. queue page seam handoff
3. queue response seam handoff

It no longer directly owns:

- task/result/review acquisition
- queue extraction from the decorated result
- empty-page shaping
- filtering, sorting, and paging
- review-summary attachment
- delta-token construction
- `NotModified` short-circuit
- final queue conditional decoration

That is the main ownership outcome of the phase.

### 6. Guardrails now lock queue-read ownership boundaries

The ownership protections now live in:

- [internal/listingkit/phase13_generation_queue_read_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase13_generation_queue_read_boundary_test.go:1)
- [internal/listingkit/service_generation_queue_test.go](/D:/code/task-processor/internal/listingkit/service_generation_queue_test.go:1)

These now protect four things:

1. `GetTaskGenerationQueue(...)` must continue delegating to snapshot/page/response seams in order
2. snapshot seam must continue owning task/result/review acquisition and queue handoff, but not page shaping or response finalization
3. page seam must continue owning queue page shaping and review-summary attachment, but not delta-token or conditional finalization
4. response seam must continue owning delta-token, `NotModified`, and final conditional decoration, but not snapshot acquisition or page shaping

The current guardrails intentionally lean on helper names, occurrence counts, explicit forbidden calls, and focused method-body checks instead of line-shape or whitespace-sensitive assertions.

## Acceptance Check

`Phase 13` was meant to prove four things:

1. queue snapshot acquisition, queue page shaping, and queue response finalization can live behind separate explicit ListingKit-owned seams
2. the queue-read service entry can become more orchestration-focused without changing queue-read behavior
3. review-summary and conditional-read semantics can stay protected after the ownership split
4. the new split can be protected with focused behavior tests and low-fragility boundary guardrails

All four are now true.

More concretely:

- `task_generation_service.go` no longer owns the old mixed queue-read block
- snapshot, page, and response responsibilities now have separate local homes
- `DeferredSections` now participates in queue token invalidation consistently with exposed summary fields
- behavior coverage remains on the most fragile queue-read semantics
- guardrails now block the most likely regrowth directions back into the service layer

## What This Phase Did Not Try To Solve

### 1. It did not redesign queue business semantics

This phase deliberately did not reopen:

- queue item state semantics
- quality/execution filtering rules
- review action business policy
- preview/resource descriptor contract design

That was the right tradeoff. The actual hotspot was queue-read ownership, not queue business policy.

### 2. It did not introduce a generic queue/query framework

All new seams remain local to ListingKit:

- queue snapshot acquisition
- queue page shaping
- queue response finalization

That is appropriate here. The pressure was concentrated inside one feature path, not spread across the repo.

### 3. It did not fold unrelated working-tree changes into this slice

The working tree still contains an unrelated change in:

- [data/sensitive_words_shein.json](/D:/code/task-processor/data/sensitive_words_shein.json:1)

This phase intentionally left that alone.

## Residual Responsibilities Still Present

### Boundary tests still rely on stable seam/helper names

The ownership tests intentionally check:

- seam handoff markers
- explicit forbidden helper calls
- focused source-boundary signals

That keeps them lower-fragility than line-shape assertions, but they still depend on stable seam/helper naming. A future rename-only refactor will need test updates.

### Queue response descriptor assertions are still contract-oriented, not exhaustive

The queue-read tests now prove:

- final queue responses carry `ResourceDescriptors`
- those descriptors line up with the final queue item contract
- `not_modified` and review-summary-driven token changes work as expected

They do not attempt to exhaustively freeze every descriptor field combination. That is acceptable for this slice because the main ownership risk was “did final conditional decoration still happen,” not “freeze the entire descriptor schema.”

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “keep carving queue-read seams for symmetry.” Better next steps are:

### 1. Watch whether queue-side shaping pressure grows somewhere else nearby

If future changes keep landing around:

- queue descriptor shaping
- queue summary semantics
- read-side contract compatibility

then the next slice should be driven by that concrete pressure, not by the existence of `Phase 13`.

### 2. Leave this layer alone unless a new ownership hotspot appears

This layer is now in a good enough state:

- snapshot acquisition is explicit
- page shaping is explicit
- response finalization is explicit
- service orchestration is thin
- deferred-only token invalidation is covered
- guardrails exist

Do not keep editing it for symmetry alone.

## Verification Summary

The final `Phase 13` verification that passed on this branch was:

```powershell
go test ./internal/listingkit -run "TestGetTaskGenerationQueue.*|TestTaskGenerationQueueRead.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Additional focused verification that passed during the phase included:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationQueueReadSnapshot.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationQueueReadPage.*|TestGetTaskGenerationQueue.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationQueueReadResponse.*|TestGetTaskGenerationQueue.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationQueueRead.*Boundary" -count=1
```
