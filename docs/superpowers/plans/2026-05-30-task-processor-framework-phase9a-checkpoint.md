# Task Processor Framework Phase 9A Checkpoint

## Status

`Phase 9A` is functionally complete for the intended slice.

This phase was not about redesigning ListingKit task generation as a whole, introducing a generic retry framework, or normalizing every generation-update path around one abstraction. The goal was narrower:

1. stop [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:210) from remaining the implicit shared home of retry mutation, retry persistence, and retry result projection
2. make those three responsibilities explicit through feature-local seams
3. preserve current retry behavior, including nil-dispatch safety and persisted-platform fallback during result rebuild
4. lock the new ownership split so the old inline retry logic does not silently grow back

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Retry mutation now has its own local seam

The retry-mutation seam now lives in:

- [internal/listingkit/task_generation_retry_mutation.go](/D:/code/task-processor/internal/listingkit/task_generation_retry_mutation.go:1)

This seam now owns:

- `mergeGenerationTasks(...)`
- `replaceGeneratedAssetsForTargets(...)`
- `rebuildInventorySummary(...)`
- the nil-dispatch no-op path for inventory mutation

This matters because the root problem here was not file length. The risk was that retry mutation side effects were still bundled into the service entry, making later retry changes easy to pile back into one orchestration method.

### 2. Retry persistence now has its own local seam

The retry-persist seam now lives in:

- [internal/listingkit/task_generation_retry_persist.go](/D:/code/task-processor/internal/listingkit/task_generation_retry_persist.go:1)

This seam now owns:

- `SaveInventory(...)`
- `SaveGenerationTasks(...)`
- the hard-fail behavior when either persistence call fails
- the ordering rule that inventory must persist before generation tasks

The contract is also narrower than the original service shape. The seam only depends on the persistence operations it actually needs, instead of the whole asset repository surface.

### 3. Retry result projection now has its own local seam

The retry-projection seam now lives in:

- [internal/listingkit/task_generation_retry_projection.go](/D:/code/task-processor/internal/listingkit/task_generation_retry_projection.go:1)

This seam now owns:

- `rebuildBundleFromInventory(...)`
- `attachPlatformImageBundles(...)`
- `decorateListingKitResultGeneration(...)`
- `syncAssetRenderPreviews(...)`
- `decorateListingKitResultReview(...)`
- retry page and queue shaping

This is the key ownership split for the read-model side of retry. The service entry no longer rebuilds bundles, platform image previews, review decoration, or queue projections inline.

### 4. Nil-request platform fallback is now aligned with existing ListingKit conventions

The retry-projection path now derives platforms through:

- [internal/listingkit/preview_overview_support.go](/D:/code/task-processor/internal/listingkit/preview_overview_support.go:20)

That means retry projection now follows the same convention as the rest of ListingKit:

1. prefer `task.Result.Platforms`
2. then fall back to `task.Request.Platforms`

This was the most important correctness fix inside the phase. The earlier “do not panic on nil request” behavior was not enough, because it still allowed platform bundles to go stale when persisted platform state existed but request state did not.

### 5. The service entry is now orchestration-focused

The retry entry still lives in:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:210)

It now mainly coordinates:

1. retry target selection
2. dispatch
3. mutation seam handoff
4. persistence seam handoff
5. projection seam handoff
6. `SaveTaskResult(...)`

It no longer directly owns:

- task merge logic
- inventory asset replacement logic
- inventory summary rebuild
- inventory persistence
- generation-task persistence
- bundle rebuild
- platform image-bundle reattach
- retry queue/result shaping

That is the main ownership outcome of this phase.

### 6. Guardrails now lock retry ownership boundaries

The ownership protections now live in:

- [internal/listingkit/phase9_task_generation_retry_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase9_task_generation_retry_boundary_test.go:1)
- [internal/listingkit/service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1)

These now protect four things:

1. `RetryTaskGenerationTasks(...)` must continue delegating to mutation, persistence, and projection seams
2. mutation logic must stay out of the service entry
3. persistence logic must stay out of the service entry
4. projection logic, including bundle rebuild, preview sync, review decoration, and queue shaping, must stay out of the service entry

This is the main anti-regrowth protection for the phase.

## Acceptance Check

`Phase 9A` was meant to prove four things:

1. retry mutation, persistence, and result projection can live behind separate explicit ListingKit-owned seams
2. the service entry can become more orchestration-focused without changing retry behavior
3. nil-dispatch and nil-request edges can stay safe without reopening ownership boundaries
4. the new retry split can be protected with focused behavior tests and source-boundary guardrails

All four are now true.

More concretely:

- retry mutation, retry persistence, and retry projection now have separate local homes
- `task_generation_service.go` no longer owns the old inline retry body
- nil-dispatch remains safe and nil-request now still rebuilds platform bundles from persisted platform state
- tests now cover both runtime behavior and ownership boundaries

## What This Phase Did Not Try To Solve

### 1. It did not introduce a generic retry abstraction

This phase deliberately stayed inside ListingKit-local seams.

It did not try to unify retry behavior across:

- workflow deferred dispatch
- task-generation retry
- other regeneration or execution paths

That was the right tradeoff. The real hotspot was one service entry still mixing three retry responsibilities.

### 2. It did not move `SaveTaskResult(...)` out of the service layer

The service entry still owns:

- `SaveTaskResult(ctx, task.ID, rebuiltResult)`

That is intentional. The phase was about splitting mutation, persistence, and projection ownership, not about redefining where task-level result commits belong.

### 3. It did not redesign retry selection or dispatch policy

This phase did not reopen:

- retry candidate selection policy
- dispatch request shaping
- review-decision persistence
- broader task-generation workflow semantics

Those remain intentionally outside this slice.

## Residual Responsibilities Still Present

### Boundary tests remain source-shape guardrails

The ownership tests intentionally check:

- required seam handoffs
- forbidden inline helper calls
- ownership markers in the new seam files

That is pragmatic and consistent with the current testing style, but it is still a source-shape strategy rather than an AST-level contract.

### Some delegation assertions are still file-level

The `required` delegation checks in the retry boundary test still match source at the file level, while the forbidden checks are narrowed to the `RetryTaskGenerationTasks(...)` function body.

That is acceptable for this slice because the highest-value risk here was ownership regrowth, not every possible equivalent refactor.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “keep carving retry seams for symmetry.” Better next steps are:

### 1. Watch whether retry-side shaping pressure keeps growing nearby

If future changes keep landing around:

- retry result shaping
- retry persistence policy
- retry/task-generation projection semantics

then the next slice should be driven by that concrete pressure, not by the existence of `Phase 9A`.

### 2. Leave this layer alone unless another real ownership hotspot appears

This layer is now in a good enough state:

- mutation is explicit
- persistence is explicit
- projection is explicit
- service orchestration is visible
- nil edges are covered
- guardrails exist

Do not keep editing it for symmetry alone.

## Verification Summary

The final `Phase 9A` verification that passed on this branch was:

```powershell
go test ./internal/listingkit -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Additional focused verification that passed during the phase included:

```powershell
go test ./internal/listingkit -run "TestRetryGeneration(ResultProjectionRebuildsListingKitResult|ResultProjectionBuildsQueues|ResultProjectionHandlesNilTaskRequest|TaskGenerationServiceFileDelegatesRetryProjection)$" -count=1
go test ./internal/listingkit -run "TestRetryTaskGenerationTasks" -count=1
go test ./internal/listingkit -run "Test.*Retry.*Boundary|TestTaskGenerationServiceFileDelegatesRetryProjection" -count=1
```

## Recommended Status

`Phase 9A` should be considered complete.

The retry-ownership problem that motivated the phase has been addressed, the service entry is thinner, nil-request projection semantics are now correct, behavior stayed green, and the new split is protected by both focused behavior tests and boundary guardrails. If we continue, the next step should begin with a new scope decision, not with more opportunistic seam carving inside this same retry slice.
