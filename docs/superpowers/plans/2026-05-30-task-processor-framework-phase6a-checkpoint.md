# Task Processor Framework Phase 6A Checkpoint

## Status

`Phase 6A` is functionally complete for the intended slice.

This phase was not about redesigning ListingKit workflow semantics or introducing a generic execution-context model. The goal was narrower:

1. stop [internal/listingkit/workflow_platform_finalize_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_finalize_phase.go:1) from remaining the primary home of platform post-processing, deferred asset dispatch, and summary/finalization completion at the same time
2. keep finalization feature-owned inside ListingKit
3. preserve current behavior, especially around SHEIN review sequencing and deferred asset-generation side effects
4. lock the new ownership split with source-boundary and behavior guardrails

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Platform post-processing now has its own seam

The new post-processing seam lives in:

- [internal/listingkit/workflow_platform_postprocess_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_postprocess_phase.go:1)

This seam now owns:

- SHEIN content optimization
- default SHEIN pricing shaping
- SDS official-image overlay
- SHEIN Studio AI image overlay

The orchestration entry in:

- [internal/listingkit/workflow_platform_finalize_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_finalize_phase.go:1)

now delegates that block through:

- `buildPlatformPostprocessPhase(p.service).run(...)`

That matters because the previous finalization seam mixed post-assembly platform shaping with dispatch and final summary completion in one body.

### 2. Deferred platform asset dispatch now has its own seam

The new asset-dispatch seam lives in:

- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)

This seam now owns:

- deferred platform asset dispatch
- dispatch-result inventory merge
- asset-bundle rebuild after generated assets return
- generation-task merge and persistence
- generation decoration after dispatch

The orchestration entry now delegates that block through:

- `buildPlatformAssetDispatchPhase(p.service).run(...)`

This is the main behavioral split for the deferred generation side of finalization. Dispatch no longer shares the same primary method body with platform post-processing and summary completion.

### 3. Summary/review completion now has its own seam

The new summary seam lives in:

- [internal/listingkit/workflow_platform_summary_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_summary_phase.go:1)

This seam now owns:

- snapshot warning merge
- `shein_review` stage lifecycle
- review issue generation
- summary finalization
- preview synchronization
- final finalization logging

To preserve the already-validated ordering semantics, this seam now exposes:

- `prepareReview(...)`
- `complete(...)`
- `run(...)`

and the orchestrator uses the split form:

- prepare review
- run `applySheinVariantImageCoverageGuard(...)`
- run deferred asset dispatch
- complete summary/finalization

That split is important. A fully bundled `run(...)` call would have silently changed the timing of `shein_review` issue generation relative to the coverage guard, which would have altered `WorkflowIssues`, `IssueCount`, and downstream review signals.

### 4. Finalization orchestration is now thinner and more explicit

The remaining orchestration home is still:

- [internal/listingkit/workflow_platform_finalize_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_finalize_phase.go:1)

but it now mainly coordinates:

- post-process seam
- summary review preparation
- variant coverage guard
- asset-dispatch seam
- summary completion seam

That is the desired long-term shape for this layer: one visible feature-owned orchestration entry, with narrower seams for the heavy downstream behavior groups.

### 5. Guardrails now lock the finalization ownership split

The new and updated boundary protections live in:

- [internal/listingkit/phase6a_platform_finalize_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase6a_platform_finalize_boundary_test.go:1)
- [internal/listingkit/phase5b_workflow_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase5b_workflow_boundary_test.go:1)
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)

These now protect four things:

1. `workflow_platform_adaptation.go` still delegates finalization through `buildPlatformFinalizePhase(s).run(...)`
2. `workflow_platform_finalize_phase.go` still delegates the heavy bodies through the three feature-owned seams
3. `applySheinVariantImageCoverageGuard(...)` stays in the finalize orchestrator, not in post-process or summary seams
4. behavior-level regression tests keep deferred dispatch and SHEIN review sequencing from drifting

## Acceptance Check

`Phase 6A` was meant to prove five things:

1. platform post-processing can move behind a dedicated ListingKit-owned seam
2. deferred platform asset dispatch can move behind a dedicated ListingKit-owned seam
3. summary/finalization completion can move behind a dedicated ListingKit-owned seam
4. `workflow_platform_finalize_phase.go` can become orchestration-focused without breaking behavior
5. the new ownership split can be protected with narrow source and behavior guardrails

All five are now true.

More concretely:

- platform finalization no longer has one monolithic behavior body
- the biggest downstream responsibilities now have separate feature-owned homes
- the variant-coverage / SHEIN review ordering bug was detected and corrected during the phase
- fresh ListingKit package verification passes on the current branch

## What This Phase Did Not Try To Solve

### 1. It did not introduce a generic workflow or context framework

This phase deliberately stayed inside ListingKit and reused the existing bounded-seam approach.

That is the right tradeoff. The root problem here was concentrated ownership inside one finalization seam, not the absence of a shared execution framework.

### 2. It did not eliminate all orchestration from `workflow_platform_finalize_phase.go`

That file still coordinates:

- seam ordering
- the placement of `applySheinVariantImageCoverageGuard(...)`
- cross-seam handoff

That is intentional. The aim was to stop it being the primary home of all heavy finalization logic, not to collapse it into zero lines.

### 3. It did not solve every possible source-boundary or sequence expression problem

The new boundary tests are source-string guardrails plus behavior tests. They are good at preventing regrowth and obvious ownership drift, but they are not a full sequence-modeling framework.

That is acceptable for this slice.

## Residual Responsibilities Still Present

### `workflow_platform_finalize_phase.go` still owns one important coordination decision

It still directly owns:

- `applySheinVariantImageCoverageGuard(final, task.Request, final.Shein)`

This is not accidental leftover code. It is the coordination point that had to remain outside the pure summary seam in order to preserve the previously validated `shein_review` issue ordering.

### `workflow_platform_summary_phase.go` now carries a subtle suppression helper

- [internal/listingkit/workflow_platform_summary_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_summary_phase.go:1)

now contains the review-suppression helper that prevents coverage-guard warnings from being reclassified into `shein_review` review issues during review-stage issue generation.

That is acceptable for now, but it is the most delicate logic introduced in this slice.

### Source-boundary tests remain string-based

The new boundary tests intentionally check for:

- required seam calls
- forbidden regrowth calls
- guard placement by file

That is a pragmatic choice, but it means future harmless renames or structural refactors will need corresponding test updates.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “extract even more lines from finalization because the seams are clearer now.” Better next steps are:

### 1. Observe whether the summary seam’s suppression logic becomes a new hotspot

If future changes keep landing in:

- [internal/listingkit/workflow_platform_summary_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_summary_phase.go:1)

then the next slice should be driven by concrete review/finalization semantics, not by aesthetic symmetry.

### 2. Watch whether the asset-dispatch seam starts needing a clearer mutation contract

If future changes keep crossing:

- `final`
- `inventory`
- persisted generation tasks

inside:

- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)

then the next slice may need a more explicit local mutation/result contract for that seam.

### 3. Leave this layer alone unless another concrete ownership problem appears

This layer is now in a good enough state:

- three heavy behavior groups have separate seams
- orchestration is visible
- behavior stayed stable
- boundary tests exist

Do not keep editing it for symmetry alone.

## Verification Summary

The final `Phase 6A` verification that passed on this branch was:

```powershell
go test ./internal/listingkit -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Additional focused verification that passed during the phase included:

```powershell
go test ./internal/listingkit -run "Test(RunWorkflowAppliesSheinPlatformFinalizationDecorations|RunWorkflowPersistsAssetInventoryAndBuildsPlatformBundles)" -count=1
go test ./internal/listingkit -run "Test(RunWorkflowPersistsDeferredPlatformDispatchOutputs|RunWorkflowRecordsDeferredAssetGenerationDispatchFailure|RunWorkflowPersistsAssetInventoryAndBuildsPlatformBundles)" -count=1
go test ./internal/listingkit -run "Test(RunWorkflowFinalizesSummaryAfterPlatformDispatch|PlatformSummaryPhaseFinalizesReviewAndPreviewState|PlatformSummaryPhaseDoesNotConvertCoverageGuardReasonIntoSheinReviewIssue|RunWorkflowAppliesVariantCoverageGuardAfterSheinReview|RunWorkflowAppliesSheinPlatformFinalizationDecorations)" -count=1
go test ./internal/listingkit -run "Test(WorkflowPlatformFinalizePhaseFileDelegatesToFinalizeSubSeams|WorkflowPlatformFinalizeCoverageGuardStaysInFinalizePhase|WorkflowPlatformAdaptationFileDelegatesFinalizationToPhaseSeam|PlatformSummaryPhaseDoesNotConvertCoverageGuardReasonIntoSheinReviewIssue|ApplySheinVariantImageCoverageGuardMarksProvidedResultNeedsReview)" -count=1
```

Notes:

- During Task 4, one subagent observed a single transient failure on one `go test ./internal/listingkit -count=1` run, but the final fresh verification above passed cleanly on this branch and is the only evidence counted for completion.

## Current Branch Notes

The main `Phase 6A` commits are:

- `dbbc9bc9` `docs: add framework phase6a plan`
- `c4c861ab` `refactor: extract listingkit platform postprocess phase`
- `1b75d4e2` `fix: preserve listingkit postprocess timing`
- `39ea1abf` `test: cover variant guard workflow timing`
- `add36b39` `refactor: extract listingkit platform asset dispatch phase`
- `823505de` `refactor: extract listingkit platform summary phase`
- `75484cb8` `fix: restore listingkit shein review issue ordering`
- `b958602b` `test: lock listingkit platform finalization boundaries`
- `3c2a14ef` `test: lock finalize coverage guard boundary`

## Recommendation

Mark `Phase 6A` complete.

Do not keep polishing this seam just because the files are now smaller and more structured. The main ownership bug this phase addressed is already fixed:

- ListingKit platform post-processing, deferred platform asset dispatch, and summary/finalization completion no longer primarily live as one crossed behavior body inside `workflow_platform_finalize_phase.go`

If we continue, the better next step is to choose a new hotspot based on real change pressure, most likely around the now-explicit summary seam or asset-dispatch mutation seam, not more finalize-layer symmetry cleanup.
