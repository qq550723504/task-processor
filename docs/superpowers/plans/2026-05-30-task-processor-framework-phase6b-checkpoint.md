# Task Processor Framework Phase 6B Checkpoint

## Status

`Phase 6B` is functionally complete for the intended slice.

This phase was not about redesigning ListingKit workflow policy or introducing a generic review engine. The goal was narrower:

1. stop [internal/listingkit/workflow_platform_summary_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_summary_phase.go:1) from remaining the mixed home of review-stage issue derivation and durable summary completion
2. make the `shein_review` compatibility rule explicit instead of hiding it behind temporary slice mutation
3. keep workflow-path and read-state refresh semantics aligned
4. lock the new ownership split so review logic does not silently grow back into the summary seam

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Review-stage semantics now flow through a dedicated review seam

The review seam now lives in:

- [internal/listingkit/workflow_platform_review_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_review_phase.go:1)

This seam now owns:

- snapshot warning merge before review-stage issue derivation
- `shein_review` stage lifecycle
- SHEIN inspection review shaping
- coverage-warning review-state shaping before issue derivation
- `shein_review_required` issue injection through the shared review-state helpers

That matters because the root problem here was not file size alone. The risk was that review-stage compatibility behavior was hidden inside the summary seam, while other paths could rebuild review state differently.

### 2. Review issue compatibility is now explicit and SHEIN-owned

The main review-state helpers now live in:

- [internal/listingkit/workflow_review_state.go](/D:/code/task-processor/internal/listingkit/workflow_review_state.go:1)

This seam now explicitly separates:

- `sheinInspectionReviewReasons(...)`
- `applySheinInspectionReviewToSummary(...)`
- `applySheinVariantCoverageReviewToSummary(...)`
- `sheinReviewIssueReasons(...)`
- `addSheinReviewWorkflowIssues(...)`

This is the real root-cause fix for the phase.

Instead of temporarily removing items from:

- `Summary.Warnings`
- `ReviewReasons`
- `Shein.ReviewNotes`

and then restoring them after issue derivation, the code now makes the compatibility rule explicit:

- coverage guard warnings stay in result state
- cookie-unavailable notes still become blocking issues
- only SHEIN-owned inspection reasons become `shein_review_required` issues

### 3. Workflow and read-state refresh now share the same review-state shaping rules

The read-state refresh path still lives in:

- [internal/listingkit/task_result_support.go](/D:/code/task-processor/internal/listingkit/task_result_support.go:1)

but it now rebuilds SHEIN review state through the same explicit helpers used by the workflow path:

- `applySheinInspectionReviewToSummary(...)`
- `applySheinVariantCoverageReviewToSummary(...)`
- `addSheinReviewWorkflowIssues(...)`

That is the other half of the ownership fix. Before this phase, workflow-time behavior and read-time reconstruction could drift because they did not use the same compatibility rule.

### 4. Summary seam is now a pure durable-completion seam

The summary seam still lives in:

- [internal/listingkit/workflow_platform_summary_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_summary_phase.go:1)

It now owns only:

- `FinalizeSummary()`
- platform preview synchronization
- final finalization logging

It no longer owns:

- snapshot warning merge
- review-stage issue derivation
- coverage-warning compatibility shaping

This is the key seam-narrowing outcome of the phase.

### 5. Finalize orchestration now makes review ownership explicit

The orchestration entry still lives in:

- [internal/listingkit/workflow_platform_finalize_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_finalize_phase.go:1)

It now visibly coordinates:

1. post-process seam
2. review seam
3. coverage guard
4. asset-dispatch seam
5. summary completion seam

That is the intended long-term shape for this layer: visible orchestration, but explicit ownership for review semantics and durable completion.

### 6. Guardrails now lock summary/review ownership

The new and updated protections live in:

- [internal/listingkit/phase6a_platform_finalize_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase6a_platform_finalize_boundary_test.go:1)
- [internal/listingkit/phase6b_summary_review_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase6b_summary_review_boundary_test.go:1)
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)
- [internal/listingkit/workflow_review_state_test.go](/D:/code/task-processor/internal/listingkit/workflow_review_state_test.go:1)
- [internal/listingkit/workflow_state_test.go](/D:/code/task-processor/internal/listingkit/workflow_state_test.go:1)

These now protect five things:

1. summary seam continues to own durable completion, not review compatibility
2. finalize orchestration keeps `review prep -> coverage guard -> asset dispatch -> completion`
3. coverage guard stays in finalize, not in postprocess, review, or summary seams
4. other stage issues do not get reclassified into `shein_review`
5. coverage-only review state survives read-state refresh without generating `shein_review_required`

## Acceptance Check

`Phase 6B` was meant to prove four things:

1. review-stage compatibility can move behind one explicit ListingKit-owned seam
2. read-state refresh can reuse the same compatibility rule without drifting
3. summary seam can narrow down to durable completion only
4. the new ownership split can be protected with source and behavior guardrails

All four are now true.

More concretely:

- review semantics no longer primarily live inside `workflow_platform_summary_phase.go`
- coverage-warning compatibility no longer depends on temporary slice mutation
- workflow and read-state refresh now rebuild SHEIN review state with the same shaping helpers
- summary seam is visibly smaller and more stable

## What This Phase Did Not Try To Solve

### 1. It did not redesign asset-dispatch mutation contracts

This phase stayed focused on summary/review semantics.

It did not try to redefine how deferred asset dispatch mutates:

- `final`
- persisted generation tasks
- inventory state

That remains a separate hotspot if future pressure appears there.

### 2. It did not introduce a generic review framework

This slice deliberately reused mature local ListingKit patterns:

- `newWorkflowRecorder(...)`
- existing `WorkflowIssue` / `GenerationSummary` shaping
- existing SHEIN review-state helpers

That is the right tradeoff. The root problem was local ownership drift, not a missing cross-feature framework.

### 3. It did not remove orchestration from finalize

[internal/listingkit/workflow_platform_finalize_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_finalize_phase.go:1)

still owns seam ordering.

That is intentional. The goal was to make review and summary ownership explicit, not to collapse orchestration into zero lines.

## Residual Responsibilities Still Present

### Review semantics remain feature-specific to SHEIN

The explicit helpers introduced in:

- [internal/listingkit/workflow_review_state.go](/D:/code/task-processor/internal/listingkit/workflow_review_state.go:1)

are intentionally SHEIN-specific.

That is acceptable for now because the actual compatibility pressure came from SHEIN review and coverage guard behavior, not from a generalized multi-platform review model.

### Boundary tests are still string-based

The source-boundary protections intentionally check:

- required seam calls
- forbidden regrowth calls
- ordering by call presence in specific files

That is pragmatic and effective against ownership drift, but harmless renames or structural rewrites will still need test updates.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “more seam symmetry because the files are cleaner now.” Better next steps are:

### 1. Watch whether platform finalization keeps accumulating business policy

If future changes keep landing in:

- [internal/listingkit/workflow_platform_finalize_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_finalize_phase.go:1)

then the next slice should be driven by real finalization-pressure hotspots, not by abstract seam goals.

### 2. Reassess whether summary completion needs richer durable-state contracts

If later changes start crossing:

- summary counts
- preview sync
- downstream durable display state

inside:

- [internal/listingkit/workflow_platform_summary_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_summary_phase.go:1)

then the next slice may need a more explicit completion contract.

### 3. Leave the review seam alone unless a second wave of change pressure appears

This layer is now in a good enough state:

- review compatibility is explicit
- read-state refresh is aligned
- summary seam is narrow
- guardrails exist

Do not keep editing it for symmetry alone.

## Verification Summary

The final `Phase 6B` verification that passed on this branch was:

```powershell
go test ./internal/listingkit -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Additional focused verification that passed during the phase included:

```powershell
go test ./internal/listingkit -run "TestPlatform(SummaryPhaseFinalizesCompletionState|FinalizePhasePreparesReviewBeforeCompletion|ReviewPhasePreparesSheinReview|ReviewPhaseDoesNotConvertCoverageGuardReasonIntoSheinReviewIssue)$" -count=1
go test ./internal/listingkit -run "Test(AddSheinReviewWorkflowIssuesIgnoresOtherStageReasons|RefreshSheinTaskResultStateKeepsCoverageWarningButSkipsReviewIssue|ApplySheinInspectionReviewToSummaryMarksResultNeedsReview)$" -count=1
go test ./internal/listingkit -run "TestWorkflowPlatformSummaryPhaseFileOwnsCompletionNotReviewCompatibility|TestWorkflowPlatformFinalizePhaseFileDelegatesToFinalizeSubSeams|TestWorkflowPlatformFinalizeCoverageGuardStaysInFinalizePhase" -count=1
go test ./internal/listingkit -run "TestGetTaskResultRefreshesStaleSheinCookieReviewState" -count=1
```

## Current Branch Notes

The main `Phase 6B` commits are:

- `7f1ff36a` `docs: add framework phase6b plan`
- `eb79ef0c` `refactor: extract listingkit platform review phase`
- `c8f4dee5` `refactor: clarify listingkit review issue compatibility`
- `50b587fa` `fix: align shein review issue derivation`
- `1fbba8a6` `refactor: narrow listingkit summary completion seam`
- `be3006cb` `test: lock listingkit summary review boundaries`
- `1410cd33` `test: tighten coverage guard seam boundary`

## Recommendation

Mark `Phase 6B` complete.

Do not keep polishing this area just because summary/review ownership is now clearer. The main ownership bug this phase addressed is already fixed:

- ListingKit review-stage compatibility no longer primarily lives as hidden mutation inside the summary seam, and read-state refresh no longer rebuilds a divergent version of that rule

If we continue, the better next step is to choose a new hotspot based on where behavior-level changes are actually landing, most likely around platform-finalization pressure or durable completion contracts, not more summary/review seam symmetry cleanup.
