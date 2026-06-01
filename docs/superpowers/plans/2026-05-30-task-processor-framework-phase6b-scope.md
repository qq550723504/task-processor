# Task Processor Framework Phase 6B Scope Recommendation

## Recommendation

`Phase 6B` should focus on **ListingKit summary/review seam semantics**, not on asset-dispatch mutation contract extraction yet.

The highest-value next hotspot is:

- continuing to narrow [internal/listingkit/workflow_platform_summary_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_summary_phase.go:1)
- reducing how much review sequencing and warning suppression logic is hidden inside `withSheinVariantCoverageReviewSuppressed(...)`

In short:

- `Phase 6A` already split platform finalization into the right three seams
- `Phase 6B` should address the most delicate behavior now concentrated inside the summary seam

## Why This Is The Right Next Step

After `Phase 6A`, the biggest remaining asymmetry is no longer:

- platform post-processing ownership
- deferred asset dispatch ownership
- source-boundary protection for finalization

Those three are now on explicit feature-owned seams.

The biggest remaining asymmetry is that one seam now hides a subtle behavior-preservation mechanism:

- [internal/listingkit/workflow_platform_summary_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_summary_phase.go:1)

This seam currently decides, in one place:

1. snapshot warning merge
2. `shein_review` stage lifecycle
3. review issue generation
4. temporary suppression of coverage-guard warnings during review issue derivation
5. summary finalization
6. preview synchronization
7. final finalization logging

That is the next root-cause hotspot because it mixes two different categories of behavior:

- durable finalization behavior
- a fragile compatibility rule that exists only to preserve historical issue-ordering semantics

The code is still better than before `Phase 6A`, but the most failure-prone logic is now concentrated here.

## Current Hotspot

The main hotspot is:

- [internal/listingkit/workflow_platform_summary_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_summary_phase.go:1)

The strongest signal is not file size. The stronger signal is that the seam currently needs:

- `prepareReview(...)`
- `complete(...)`
- `run(...)`
- `withSheinVariantCoverageReviewSuppressed(...)`

to preserve one specific behavior:

- coverage-guard warnings must affect summary/review state
- but must not be reclassified into `shein_review` issues during review issue generation

That means this seam is doing more than “summary finalization.” It is also acting as a behavior-compatibility adapter.

## Candidate Phase 6B Directions

There are two realistic directions from the current branch state.

### Option 1: Summary/review seam semantics

Keep the work feature-owned inside ListingKit and make the summary/review semantics more explicit.

This would likely mean:

- isolating review-issue derivation from generic summary finalization
- making coverage-guard suppression a clearer collaborator or helper contract
- reducing the amount of temporary state mutation needed inside `withSheinVariantCoverageReviewSuppressed(...)`
- keeping `prepareReview(...)` and `complete(...)` only if they still express the clearest ownership line

**Pros**

- directly targets the most delicate logic introduced in `Phase 6A`
- lowers the chance of another silent `WorkflowIssues` / `ReviewCount` regression
- stays aligned with the “fix root cause, not surface symmetry” principle
- preserves the current feature-owned workflow shape

**Cons**

- requires careful behavior tests because this area is sequencing-sensitive
- easy to over-abstract if we chase elegance instead of explicit semantics

**Recommendation:** `Yes`

This is the best `Phase 6B` target.

### Option 2: Asset-dispatch mutation contract

Keep the work feature-owned inside ListingKit and clarify the mutation/result surface of:

- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)

This would likely mean:

- turning the seam’s many mutations into a more explicit result object or mutation bundle
- clarifying which side effects belong to inventory, result bundle, and persisted generation tasks
- reducing implicit coupling across `final`, `inventory`, and persisted task state

**Pros**

- improves local clarity in a seam with many side effects
- could make deferred dispatch easier to reason about and test
- may help if more asset-generation behavior lands here soon

**Cons**

- currently more of a second-order design issue than a first-order bug risk
- current tests already protect the important visible behavior reasonably well
- easy to spend a lot of effort on contract tidiness without relieving the most fragile semantics

**Recommendation:** `Not yet`

This should wait unless a second wave of real dispatch-side behavior changes starts landing there.

## Why Not Prioritize Asset-Dispatch Contract First

`Phase 6A` already solved the real deferred-dispatch ownership root cause:

- dispatch no longer lives inline with platform post-processing
- dispatch-result inventory merge has one explicit home
- generation-task persistence has one explicit home

What remains there is mostly:

- mutation clarity
- contract explicitness
- future-proofing

Those are important, but they are second-order right now.

By contrast, the summary seam already contains a delicate compatibility mechanism whose failure mode is a silent behavior regression in:

- `WorkflowIssues`
- `ReviewCount`
- review-stage signaling

That makes it the more urgent hotspot.

## Suggested Phase 6B Goal

The concrete `Phase 6B` goal should be:

> Make ListingKit review/finalization semantics more explicit so `workflow_platform_summary_phase.go` stops relying on an opaque temporary-warning suppression helper as the main way to preserve `shein_review` issue ordering.

That goal is specific enough to implement incrementally and narrow enough to stay inside one real hotspot.

## Suggested Phase 6B Success Criteria

`Phase 6B` should be considered successful when:

1. `workflow_platform_summary_phase.go` expresses review-stage semantics more directly
2. the current coverage-guard / `shein_review` ordering rule is preserved with clearer ownership
3. behavior tests explicitly protect `WorkflowIssues`, `ReviewCount`, and review-summary interactions
4. `workflow_platform_finalize_phase.go` does not regrow heavy finalization logic during the work
5. no generic workflow/context abstraction is introduced unless another concrete hotspot appears

## Suggested Non-Goals For Phase 6B

To keep the next slice disciplined, `Phase 6B` should explicitly avoid:

- redesigning deferred asset dispatch contract at the same time
- introducing a generic workflow state machine
- changing platform post-processing behavior
- changing coverage-guard business policy
- moving workflow concerns into HTTP/runtime/bootstrap layers

## Expected File Hotspots

If we take the recommended direction, the first likely hotspots are:

- [internal/listingkit/workflow_platform_summary_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_summary_phase.go:1)
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)
- [internal/listingkit/workflow_review_state.go](/D:/code/task-processor/internal/listingkit/workflow_review_state.go:1)
- [internal/listingkit/phase6a_platform_finalize_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase6a_platform_finalize_boundary_test.go:1)

Possible new files, if the split is warranted, would likely stay feature-local, for example:

- `internal/listingkit/workflow_platform_review_phase.go`
- `internal/listingkit/workflow_platform_summary_finalize.go`

The design pressure should be:

- clearer review-stage semantics
- explicit compatibility ownership
- no speculative framework work

## Recommendation Summary

Proceed to `Phase 6B`, but scope it narrowly:

- choose **ListingKit summary/review seam semantics** as the next hotspot
- defer asset-dispatch mutation contract cleanup until it shows stronger real change pressure
- keep the work feature-owned inside ListingKit

That keeps the next slice aligned with the most fragile real behavior in the codebase rather than continuing structural cleanup for its own sake.
