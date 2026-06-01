# Task Processor Framework Phase 7B Scope Recommendation

## Recommendation

`Phase 7B` should focus on **ListingKit asset-dispatch inventory persistence ownership**, not on further mutation-seam sub-splitting yet.

The highest-value next hotspot is:

- continuing to narrow [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)
- reducing how much that seam still directly decides about `SaveInventory(...)` timing after dispatch assets return

In short:

- `Phase 7A` already made mutation and generation-task persistence explicit
- `Phase 7B` should address the one remaining durability decision that still lives in the parent dispatch seam

## Why This Is The Right Next Step

After `Phase 7A`, the biggest remaining asymmetry is no longer:

- dispatch-result mutation ownership
- generation-task decorate/persist timing ownership
- source-boundary protection for asset-dispatch orchestration

Those three are now on explicit feature-owned seams.

The biggest remaining asymmetry is that one seam still directly decides when dispatched assets become durable inventory state:

- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)

This seam currently still decides, in one place:

1. whether deferred dispatch should run
2. when dispatch-result mutation is applied
3. whether returned assets justify inventory persistence
4. when `assetRepo.SaveInventory(...)` is attempted
5. how that durability decision relates to the later generation-task persistence handoff

That is the next root-cause hotspot because it mixes:

- orchestration timing
- inventory durability policy
- post-dispatch handoff sequencing

inside the same parent seam.

The code is cleaner than before `Phase 7A`, but this remaining persistence branch is still a first-order ownership signal, not just a tidiness concern.

## Current Hotspot

The main hotspot is:

- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)

The strongest signal is not file length. The stronger signal is that:

- dispatch-result mutation now lives in [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)
- generation-task decoration and persistence now live in [internal/listingkit/workflow_platform_asset_dispatch_persist.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_persist.go:1)

but the parent seam still owns:

- `if len(dispatchResult.Assets) > 0 && p.service.assetRepo != nil { _ = p.service.assetRepo.SaveInventory(ctx, inventory) }`

That means the parent seam is no longer the home of most side effects, but it still owns one important durability decision that is tightly coupled to dispatch-result asset mutation.

## Candidate Phase 7B Directions

There are two realistic directions from the current branch state.

### Option 1: Inventory-persist ownership seam

Keep the work feature-owned inside ListingKit and make the post-dispatch inventory durability decision explicit.

This would likely mean:

- separating “apply returned assets” from “durably persist returned assets”
- defining a clearer local handoff for when inventory persistence should run
- reducing how much the parent seam knows about asset-count checks and repo-write timing
- preserving current nil-safe and best-effort persistence behavior unless a real policy bug is discovered

**Pros**

- directly targets the last remaining first-order side effect still owned by the parent seam
- completes the main ownership line that `Phase 7A` started
- improves durability-path testability without reopening review/finalization semantics
- keeps the work aligned with the existing bounded-seam pattern already used in ListingKit

**Cons**

- needs careful scoping so we do not accidentally redesign persistence policy instead of ownership
- easy to over-abstract if we invent a general durability framework rather than a local seam

**Recommendation:** `Yes`

This is the best `Phase 7B` target.

### Option 2: Mutation-seam sub-splitting

Keep the work feature-owned inside ListingKit and split:

- [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)

more finely across:

- inventory record/summary mutation
- asset-bundle rebuild
- platform image-bundle reattach
- generation-task merge

**Pros**

- could make the mutation seam feel even more explicit
- may help if different mutation categories start changing independently

**Cons**

- currently more of a second-order explicitness concern than a first-order bug-risk hotspot
- `Phase 7A` already made this seam understandable and testable enough for now
- easy to spend effort on symmetry without relieving the most meaningful remaining ownership pressure

**Recommendation:** `Not yet`

This should wait until a second wave of real mutation-side change pressure appears.

## Why Not Prioritize Mutation-Seam Sub-Splitting First

`Phase 7A` already solved the real mutation/persist ownership root cause:

- dispatch-result mutation no longer lives inline with dispatch execution and decorate/persist timing
- generation-task persistence has one explicit home
- parent orchestration is already visibly thinner

What remains inside the apply seam is mostly:

- local mutation grouping
- contract explicitness
- future-proofing

Those are important, but they are second-order right now.

By contrast, inventory persistence still lives in the parent seam as an actual durability decision, which is a stronger ownership signal and a more natural next slice.

## Suggested Phase 7B Goal

The concrete `Phase 7B` goal should be:

> Make ListingKit deferred asset-dispatch inventory persistence more explicit so `workflow_platform_asset_dispatch_phase.go` stops being the primary home of the “returned assets should now be durably saved” decision.

That goal is specific enough to implement incrementally and narrow enough to stay inside one real hotspot.

## Suggested Phase 7B Success Criteria

`Phase 7B` should be considered successful when:

1. the inventory durability decision after deferred dispatch is moved behind one explicit ListingKit-owned seam or handoff
2. `workflow_platform_asset_dispatch_phase.go` becomes more purely orchestration-focused
3. behavior tests still protect deferred dispatch success/failure and persisted inventory outcomes unchanged in business terms
4. mutation and generation-task persistence seams do not regrow mixed responsibilities during the work
5. no generic persistence framework is introduced unless another feature shows the same pressure

## Suggested Non-Goals For Phase 7B

To keep the next slice disciplined, `Phase 7B` should explicitly avoid:

- redesigning mutation-seam internals just for neatness
- changing deferred asset-generation business policy
- revisiting summary/review/finalization semantics
- introducing a generic workflow durability abstraction
- moving workflow concerns into HTTP/runtime/bootstrap layers

## Expected File Hotspots

If we take the recommended direction, the first likely hotspots are:

- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)
- [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)
- [internal/listingkit/phase7a_asset_dispatch_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase7a_asset_dispatch_boundary_test.go:1)

Possible new files, if the split is warranted, would likely stay feature-local, for example:

- `internal/listingkit/workflow_platform_asset_dispatch_inventory_persist.go`
- `internal/listingkit/phase7b_asset_dispatch_persist_boundary_test.go`

The design pressure should be:

- clearer post-dispatch inventory durability ownership
- explicit handoff from mutation to inventory persistence
- no speculative shared workflow framework

## Recommendation Summary

Proceed to `Phase 7B`, but scope it narrowly:

- choose **ListingKit asset-dispatch inventory persistence ownership** as the next hotspot
- defer further mutation-seam sub-splitting until it shows stronger real change pressure
- keep the work fully feature-owned inside ListingKit

That keeps the next slice aligned with the strongest remaining durability-ownership signal in the codebase rather than continuing seam cleanup for symmetry alone.
