# Task Processor Framework Phase 12 Checkpoint

## Status

`Phase 12` is functionally complete for the intended slice.

This phase was not about redesigning generation review business semantics, reopening navigation dispatch planning, or introducing a generic read/query framework. The goal was narrower:

1. stop [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:130) and [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:140) from remaining the shared home of current snapshot acquisition, conditional-read handling, session/preview response shaping, and final conditional decoration at the same time
2. make those review-read responsibilities explicit through feature-local seams
3. preserve current `patch_only`, `NotModified`, preview projection, revision-status, scene-preset, and conditional-response behavior
4. lock the new ownership split so the old inline review-read logic does not silently grow back into the service layer

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Shared review snapshot acquisition now has its own local seam

The shared snapshot seam now lives in:

- [internal/listingkit/task_generation_review_read_snapshot.go](/D:/code/task-processor/internal/listingkit/task_generation_review_read_snapshot.go:1)

This seam now owns:

- `getCurrentListingKitResult(...)`
- handoff of the current `ListingKitResult`
- handoff of the current `AssetGenerationQueue`

This matters because the root problem here was not just method length. The risk was that both session and preview reads still directly owned duplicated current-state acquisition concerns that should stay decoupled from response shaping.

### 2. Review session read shaping now has its own local seam

The session-read seam now lives in:

- [internal/listingkit/task_generation_review_session_read.go](/D:/code/task-processor/internal/listingkit/task_generation_review_session_read.go:1)

This seam now owns:

- `buildGenerationReviewSession(...)`
- `buildGenerationReviewReadDeltaToken(...)`
- `ResponseMode` normalization
- `NotModified` short-circuit
- `patch_only` response shaping
- base-session patch construction
- final `applyGenerationConditionalStateToReviewSessionResponse(...)`

This is the main session-read ownership split of the phase. The service entry no longer directly performs those read-response shaping steps inline.

### 3. Review preview read shaping now has its own local seam

The preview-read seam now lives in:

- [internal/listingkit/task_generation_review_preview_read.go](/D:/code/task-processor/internal/listingkit/task_generation_review_preview_read.go:1)

This seam now owns:

- `buildGenerationReviewSession(...)`
- `buildGenerationReviewReadDeltaToken(...)`
- `NotModified` short-circuit
- preview projection via `resolveGenerationReviewPreviewResponse(...)`
- revision-status shaping via `resolveGenerationReviewPreviewRevisionStatus(...)`
- scene-preset shaping
- final `applyGenerationConditionalStateToReviewPreviewResponse(...)`

This is the main preview-read ownership split of the phase. The service entry no longer directly assembles preview response details inline.

### 4. The service review-read methods are now orchestration-focused wrappers

The service read entry points still live in:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:130)

They now mainly coordinate:

1. shared snapshot seam handoff
2. session-read seam handoff
3. preview-read seam handoff

They no longer directly own:

- current result acquisition
- current queue acquisition
- session delta-token shaping
- session `patch_only` shaping
- preview projection
- revision-status shaping
- preview conditional-response shaping

That is the main ownership outcome of this phase.

### 5. Guardrails now lock review-read ownership boundaries

The ownership protections now live in:

- [internal/listingkit/phase12_generation_review_read_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase12_generation_review_read_boundary_test.go:1)
- [internal/listingkit/service_generation_queue_test.go](/D:/code/task-processor/internal/listingkit/service_generation_queue_test.go:1)

These now protect four things:

1. service review-read methods must continue delegating to snapshot/session/preview seams
2. snapshot seam must continue owning current result + queue acquisition, but not conditional-read or response shaping
3. session seam must continue owning `patch_only`, `NotModified`, and session conditional shaping, but not preview projection
4. preview seam must continue owning preview projection, revision-status, scene-preset, and preview conditional shaping, but not snapshot acquisition

The current guardrails intentionally lean on helper names, explicit forbidden calls, and focused source-boundary checks instead of locking branch layout or whitespace-sensitive source shapes.

## Acceptance Check

`Phase 12` was meant to prove four things:

1. shared snapshot acquisition, session read shaping, and preview read shaping can live behind separate explicit ListingKit-owned seams
2. the service read methods can become more orchestration-focused without changing review-read behavior
3. `patch_only`, `NotModified`, preview projection, revision-status, and scene-preset behavior can stay protected after the ownership split
4. the new split can be protected with focused behavior tests and low-fragility boundary guardrails

All four are now true.

More concretely:

- `task_generation_service.go` no longer owns the old mixed review-read blocks
- snapshot, session, and preview responsibilities now have separate local homes
- behavior coverage remains on the most fragile review-read semantics
- guardrails now block the most likely regrowth directions back into the service layer

## What This Phase Did Not Try To Solve

### 1. It did not redesign review business policy

This phase deliberately did not reopen:

- section/slot review semantics
- toolbar behavior
- revision-mismatch business rules
- action-path review refresh behavior

That was the right tradeoff. The actual hotspot was review-read ownership, not review business policy.

### 2. It did not introduce a generic read/query framework

All new seams remain local to ListingKit:

- review snapshot acquisition
- review session read shaping
- review preview read shaping

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
- occurrence counts on ownership signals

That keeps them lower-fragility than line-shape assertions, but they still depend on stable seam/helper naming. A future rename-only refactor will need test updates.

### Seam boundary tests use whole-file checks

For the seam-owned files, the current tests use whole-file checks rather than only extracting a single function body. That gives stronger ownership protection, but it also means unrelated helpers added to the same file could trigger false positives if they cross a forbidden boundary.

That is acceptable for now because these files are intentionally seam-owned files.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “keep carving review-read seams for symmetry.” Better next steps are:

### 1. Watch whether queue-read ownership becomes the next concrete hotspot

If future changes keep landing around:

- queue pagination/filtering/sorting response shaping
- queue conditional-read behavior
- queue summary / review-summary attachment

then the next slice should be driven by that concrete pressure, not by the existence of `Phase 12`.

### 2. Leave this layer alone unless a new ownership hotspot appears

This layer is now in a good enough state:

- snapshot acquisition is explicit
- session read shaping is explicit
- preview read shaping is explicit
- service wrappers are thin
- behavior coverage exists
- ownership guardrails exist

Do not keep editing it for symmetry alone.

## Verification Summary

The final `Phase 12` verification that passed on this branch was:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationReview.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Additional focused verification that passed during the phase included:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationReviewReadSnapshotPhaseRunUsesSingleCurrentSnapshot$|TestTaskGenerationReviewSessionMissingSnapshotUsesCurrentEmptyResponseShape$|TestTaskGenerationReviewReadsPropagateSnapshotLoadErrors$" -count=1
go test ./internal/listingkit -run "TestTaskGenerationReviewSessionReadPhaseRun.*|TestTaskGenerationReviewReadSnapshotPhaseRunUsesSingleCurrentSnapshot|TestTaskGenerationReviewReadsPropagateSnapshotLoadErrors" -count=1
go test ./internal/listingkit -run "TestTaskGenerationReviewPreviewReadPhaseRun.*|TestTaskGenerationReviewSessionReadPhaseRun.*|TestTaskGenerationReviewReadSnapshotPhaseRunUsesSingleCurrentSnapshot|TestTaskGenerationReviewReadsPropagateSnapshotLoadErrors" -count=1
go test ./internal/listingkit -run "TestTaskGenerationReview(Read|Preview|Session).*Boundary" -count=1
```
