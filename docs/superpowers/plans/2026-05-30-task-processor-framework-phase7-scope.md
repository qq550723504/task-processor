# Task Processor Framework Phase 7 Scope Recommendation

## Recommendation

`Phase 7` should focus on **ListingKit deferred asset-dispatch mutation contract**, not on further summary/review seam cleanup.

The highest-value next hotspot is:

- clarifying the mutation and persistence contract inside [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)
- reducing how much that seam simultaneously mutates `final`, `inventory`, and persisted generation-task state

In short:

- `Phase 6A` split finalization into explicit seams
- `Phase 6B` made review/summary ownership explicit and stable
- `Phase 7` should target the remaining seam that still carries the densest side-effect surface

## Why This Is The Right Next Step

After `Phase 6B`, the biggest remaining asymmetry is no longer:

- summary/review ownership
- coverage-guard compatibility placement
- read-state refresh drift
- finalization orchestration readability

Those four are now on explicit seams with behavior and source guardrails.

The biggest remaining asymmetry is that one seam still owns several different mutation categories at once:

- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)

This seam currently decides, in one place:

1. when platform image bundles are attached before dispatch
2. when pending generation tasks are collected
3. how deferred dispatch is executed
4. how dispatch-result assets are merged into inventory
5. how `final.AssetBundle` is rebuilt after generated assets return
6. how `final.AssetInventorySummary` is refreshed
7. when inventory persistence happens
8. how platform image bundles are re-attached after dispatch
9. how persisted generation tasks are merged
10. when generation decoration is reapplied
11. when generation-task persistence happens
12. how persistence failures feed back into warnings and workflow issues

That is the next root-cause hotspot because it mixes:

- runtime mutation of in-memory workflow state
- persistence timing
- post-dispatch decoration behavior

inside one seam.

The code is cleaner than before `Phase 6A`, but this seam still concentrates the most side effects per line of code.

## Current Hotspot

The main hotspot is:

- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)

This file is not just “the next big file.” The stronger signal is that it currently owns multiple state transitions across multiple objects:

- `final`
- `inventory`
- `persistedGenerationTasks`
- `assetRepo`

The seam’s `run(...)` signature is also still carrying the pressure:

- it accepts `final`, `inventory`, `recipesByPlatform`, `generationPlan`, `persistedGenerationTasks`, and `enableAssetGeneration`
- it returns only the mutated generation-task slice, even though the real side effects span more than that single value

That mismatch is the strongest design signal in the current branch state.

## Candidate Phase 7 Directions

There are two realistic directions from the current branch state.

### Option 1: Asset-dispatch mutation contract

Keep the work feature-owned inside ListingKit and make the deferred dispatch seam’s mutation/result contract more explicit.

This would likely mean:

- defining a clearer local result or mutation bundle for dispatch-side updates
- separating “pre-dispatch attach”, “dispatch execution”, and “post-dispatch persistence/decorate” responsibilities more explicitly
- reducing how much correctness depends on in-place mutation across `final`, `inventory`, and persisted generation-task slices

**Pros**

- directly targets the seam with the densest remaining side-effect surface
- improves testability of deferred dispatch behavior without touching unrelated workflow semantics
- fits the existing bounded-seam pattern already used in ListingKit
- reduces the chance of silent regressions in inventory/persistence/decorate interactions

**Cons**

- needs careful staging because this seam already preserves several behavior invariants
- easy to over-design a “result object” if we chase neatness instead of concrete mutation clarity

**Recommendation:** `Yes`

This is the best `Phase 7` target.

### Option 2: Platform post-process sub-splitting

Keep the work feature-owned inside ListingKit and split:

- [internal/listingkit/workflow_platform_postprocess_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_postprocess_phase.go:1)

into smaller collaborators for:

- SHEIN content optimization
- pricing shaping
- SDS official-image overlay
- studio AI image overlay

**Pros**

- could make platform-specific shaping feel cleaner
- may help if more post-process rules start landing there

**Cons**

- currently more of a neatness concern than a first-order bug-risk hotspot
- the seam is already fairly bounded
- current tests and ownership are good enough for now

**Recommendation:** `Not yet`

This should wait until a second wave of real post-process changes appears.

## Why Not Continue Summary/Review Work Immediately

`Phase 6B` already solved the real summary/review ownership root cause:

- review compatibility no longer lives as hidden mutation inside the summary seam
- read-state refresh no longer rebuilds a divergent version of SHEIN review state
- summary seam is now a pure durable-completion seam
- coverage guard placement is protected by boundary tests

What remains there is mostly:

- durable completion contract clarity
- future-proofing

Those are second-order concerns right now.

By contrast, asset dispatch still combines mutation, persistence, and decoration in one seam, which is a first-order design pressure point.

## Suggested Phase 7 Goal

The concrete `Phase 7` goal should be:

> Make ListingKit deferred asset-dispatch side effects more explicit so `workflow_platform_asset_dispatch_phase.go` stops being the primary home of inventory mutation, bundle rebuild, generation-task merge, and persistence/decorate timing at the same time.

That goal is specific enough to implement incrementally and narrow enough to stay inside one real hotspot.

## Suggested Phase 7 Success Criteria

`Phase 7` should be considered successful when:

1. `workflow_platform_asset_dispatch_phase.go` has a clearer local mutation/result contract
2. dispatch execution, mutation application, and persistence/decorate concerns are more explicitly separated
3. behavior tests still protect deferred generation success and failure flows unchanged in business terms
4. finalization orchestration does not regrow heavy logic while the seam is clarified
5. no generic workflow state container is introduced unless a second seam shows the same pressure

## Suggested Non-Goals For Phase 7

To keep the next slice disciplined, `Phase 7` should explicitly avoid:

- redesigning summary/review semantics again
- introducing a generic workflow mutation framework
- moving workflow concerns into HTTP/runtime/bootstrap layers
- reworking platform post-process seams unless dispatch work proves it necessary
- changing asset-generation or review business policy

## Expected File Hotspots

If we take the recommended direction, the first likely hotspots are:

- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)
- [internal/listingkit/phase6a_platform_finalize_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase6a_platform_finalize_boundary_test.go:1)
- [internal/listingkit/workflow_platform_finalize_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_finalize_phase.go:1)

Possible new files, if the split is warranted, would likely stay feature-local, for example:

- `internal/listingkit/workflow_platform_asset_dispatch_apply.go`
- `internal/listingkit/workflow_platform_asset_dispatch_persist.go`

The design pressure should be:

- clearer dispatch-side ownership
- explicit mutation/persistence handoff
- no speculative shared workflow framework

## Recommendation Summary

Proceed to `Phase 7`, but scope it narrowly:

- choose **ListingKit deferred asset-dispatch mutation contract** as the next hotspot
- defer more summary/review cleanup until another real pressure point appears there
- defer post-process sub-splitting until platform-shaping rules start changing faster

That keeps the next slice aligned with the most concentrated remaining side-effect surface in the codebase rather than continuing seam cleanup for symmetry alone.
