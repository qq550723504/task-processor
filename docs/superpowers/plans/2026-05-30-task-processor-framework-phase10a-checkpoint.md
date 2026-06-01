# Task Processor Framework Phase 10A Checkpoint

## Status

`Phase 10A` is functionally complete for the intended slice.

This phase was not about redesigning ListingKit task generation as a whole, introducing a generic action framework, or reopening navigation dispatch planning. The goal was narrower:

1. stop [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:281) from remaining the shared home of action execution branching, post-action refresh, and action result projection at the same time
2. make those three responsibilities explicit through feature-local seams
3. preserve current action behavior across retry, queue-only, review refresh, and `patch_only` response paths
4. lock the new ownership split so the old inline action logic does not silently grow back into the service entry

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Action execution branching now has its own local seam

The action-execute seam now lives in:

- [internal/listingkit/task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:1)

This seam now owns:

- `target.InteractionMode` branching
- `RetryTaskGenerationTasks(...)`
- `GetTaskGenerationQueue(...)`
- execution-path-specific `persistenceSession` shaping

This matters because the root problem here was not file length. The risk was that the service entry still directly owned execution branching and could easily keep accumulating queue/retry-specific details.

### 2. Post-action refresh now has its own local seam

The action-refresh seam now lives in:

- [internal/listingkit/task_generation_action_refresh.go](/D:/code/task-processor/internal/listingkit/task_generation_action_refresh.go:1)

This seam now owns:

- `getCurrentListingKitResult(...)`
- overview refresh from the current queue/result state
- platform render-preview hydration
- fallback preview reuse from `baseResult`
- same-snapshot `currentResult` reuse for later projection

The important correctness point in this phase is that refresh now works from one coherent refreshed snapshot. It no longer mixes overview, previews, and current result from separate reads.

### 3. Action result projection now has its own local seam

The action-projection seam now lives in:

- [internal/listingkit/task_generation_action_projection.go](/D:/code/task-processor/internal/listingkit/task_generation_action_projection.go:1)

This seam now owns:

- review-session rebuild
- review-workflow shaping
- review-patch shaping
- delta-token selection
- `patch_only` response trimming
- queue/retry projection handoff into the public action result

This is the main read-model ownership split of the phase. The service entry no longer directly builds review workflow payloads, review patches, or patch-only response semantics inline.

### 4. The service entry is now orchestration-focused

The action entry still lives in:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:281)

It now mainly coordinates:

1. target resolution
2. baseline audit/result setup
3. execute seam handoff
4. persisted review-decision side effect
5. refresh seam handoff
6. projection seam handoff
7. final conditional-state decoration

It no longer directly owns:

- interaction-mode execution branching
- queue vs retry page fetching
- refreshed result snapshot hydration
- review-workflow assembly
- review-patch assembly
- `patch_only` trimming
- delta-token fallback logic

That is the main ownership outcome of this phase.

### 5. Guardrails now lock action ownership boundaries

The ownership protections now live in:

- [internal/listingkit/phase10_task_generation_action_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase10_task_generation_action_boundary_test.go:1)
- [internal/listingkit/service_generation_actions_test.go](/D:/code/task-processor/internal/listingkit/service_generation_actions_test.go:289)

These now protect four things:

1. `ExecuteTaskGenerationAction(...)` must continue delegating to execute, refresh, and projection seams
2. execution branching must stay out of the service entry
3. refresh hydration must stay out of the service entry
4. review-workflow, review-patch, `patch_only`, and delta-token projection must stay out of the service entry

The current guardrails are intentionally narrower than the first attempt in this phase. They now anchor on helper names, literal markers, and occurrence counts instead of depending on local variable names or fragile inline shape.

## Acceptance Check

`Phase 10A` was meant to prove four things:

1. action execution, refresh, and projection can live behind separate explicit ListingKit-owned seams
2. the service entry can become more orchestration-focused without changing action behavior
3. refreshed review payloads can stay snapshot-consistent after the ownership split
4. the new split can be protected with focused behavior tests and source-boundary guardrails

All four are now true.

More concretely:

- `task_generation_service.go` no longer owns the old mixed action body
- execution, refresh, and projection now have separate local homes
- refresh no longer rebuilds response state from multiple independent reads
- tests now protect both runtime behavior and ownership boundaries

## What This Phase Did Not Try To Solve

### 1. It did not introduce a generic action abstraction

This phase deliberately stayed inside ListingKit-local seams.

It did not try to unify action behavior across:

- task generation actions
- review navigation dispatch
- workflow-side action semantics
- any other execution path

That was the right tradeoff. The real hotspot was one service entry still mixing three responsibilities.

### 2. It did not move target resolution or audit setup out of the service layer

The service entry still owns:

- target resolution
- expected-impact fallback
- previous review-session baseline
- action audit construction

That is intentional. The phase was about execution, refresh, and projection ownership, not about eliminating all orchestration from the entrypoint.

### 3. It did not redesign persisted review-decision semantics

This phase did not reopen:

- `persistGenerationReviewDecision(...)`
- review-action persistence policy
- temporal action short-circuit behavior

Those remain intentionally outside this slice.

## Residual Responsibilities Still Present

### Boundary tests remain source-shape guardrails

The ownership tests intentionally check:

- required seam handoffs
- forbidden inline helper calls
- ownership markers in the new seam files

That is pragmatic and consistent with the current testing style, but it is still a source-shape strategy rather than an AST-level contract.

### The projection seam still coordinates several review-facing outputs

The projection seam currently owns:

- review session
- review workflow
- review patch
- delta token
- patch-only trimming

That is acceptable for now because these outputs still change together as one review-facing projection slice. It only becomes the next hotspot if later changes start splitting their policy directions.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “keep carving this exact action seam for symmetry.” Better next steps are:

### 1. Watch whether action-adjacent generation flow pressure keeps growing nearby

If future changes keep landing around:

- action-side review projection
- generation navigation dispatch
- queue/retry read-model shaping

then the next slice should be driven by that concrete pressure, not by the existence of `Phase 10A`.

### 2. Leave this layer alone unless another real ownership hotspot appears

This layer is now in a good enough state:

- execution is explicit
- refresh is explicit
- projection is explicit
- service orchestration is visible
- snapshot consistency is preserved
- guardrails exist

Do not keep editing it for symmetry alone.

## Verification Summary

The final `Phase 10A` focused verification that passed on this branch was:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationAction.*Boundary|TestTaskGenerationActionProjectionServiceDelegatesActionProjection|TestTaskGenerationServiceFileDelegatesActionExecution" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Additional focused verification that passed during the phase included:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRunBranchesByInteractionMode$" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionRefresh(RehydratesOverviewAndRenderPreviews|HydratesCurrentResultFallbacks)$" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionProjection(BuildsReviewSessionAndPatch|SupportsPatchOnlyResponses|ServiceDelegatesActionProjection)$" -count=1
go test ./internal/listingkit -run "TestExecuteTaskGenerationAction" -count=1
```

Broader verification is currently noisy because the working tree already contains unrelated submit-path edits under:

- [internal/listingkit/service_submit.go](/D:/code/task-processor/internal/listingkit/service_submit.go:1)
- [internal/listingkit/task_submission_execution_service.go](/D:/code/task-processor/internal/listingkit/task_submission_execution_service.go:1)
- related submit tests and publishing files

With those unrelated changes present, `go test ./internal/listingkit -count=1` currently fails in `TestTaskSubmissionExecutionServiceExecuteSheinSubmitRemoteRoutesByAction`, which is outside the `Phase 10A` slice.
