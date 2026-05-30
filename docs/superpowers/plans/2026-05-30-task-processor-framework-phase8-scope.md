# Task Processor Framework Phase 8 Scope Recommendation

## Recommendation

`Phase 8` should focus on **ListingKit asset-dispatch mutation-side shaping pressure**, not on inventory durability policy redesign yet.

The highest-value next hotspot is:

- reassessing [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)
- reducing how much that seam still simultaneously owns inventory mutation, bundle rebuild, platform image-bundle reattach, and generation-task merge

In short:

- `Phase 7A` made mutation and persistence explicit
- `Phase 7B` pulled durability ownership out of the parent seam
- `Phase 8` should target the densest remaining in-memory mutation surface, not reopen durability policy

## Why This Is The Right Next Step

After `Phase 7B`, the biggest remaining asymmetry is no longer:

- parent dispatch durability ownership
- inventory persistence handoff clarity
- source-boundary protection for deferred asset-dispatch durability

Those are now on explicit seams with behavior and boundary guardrails.

The biggest remaining asymmetry is that one seam still applies several different mutation categories at once:

- [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)

This seam currently still decides, in one place:

1. how returned dispatch assets are merged into inventory
2. when inventory summary is rebuilt
3. how `final.AssetBundle` is rebuilt after returned assets appear
4. when `final.AssetInventorySummary` is refreshed
5. how platform image bundles are reattached using returned task state
6. how persisted generation tasks are merged with returned dispatch tasks

That is the next root-cause hotspot because it mixes several in-memory state transitions across:

- `final`
- `inventory`
- persisted generation tasks

inside one mutation seam.

The code is cleaner than before `Phase 7A`, but this remaining seam still concentrates the most state shaping per line in the asset-dispatch path.

## Current Hotspot

The main hotspot is:

- [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)

The strongest signal is not file size. The stronger signal is that the seam still couples:

- inventory mutation
- durable-display bundle shaping
- task-state merge

behind one return object:

- `platformAssetDispatchMutation`

That return object is good enough for `Phase 7A`, but it now acts as a local bucket for several mutation categories that may evolve at different speeds.

## Candidate Phase 8 Directions

There are two realistic directions from the current branch state.

### Option 1: Mutation-side shaping seam

Keep the work feature-owned inside ListingKit and make the dispatch-result mutation responsibilities more explicit.

This would likely mean:

- separating inventory record/summary mutation from bundle/task shaping
- defining a clearer local handoff between “returned assets changed inventory” and “returned tasks changed platform bundle state”
- reducing how much correctness depends on one helper mutating `final`, `inventory`, and task state together
- keeping the result local to ListingKit instead of introducing a generic mutation framework

**Pros**

- directly targets the densest remaining in-memory state-shaping seam
- fits the bounded-seam pattern already used across recent phases
- improves testability of mutation-side behavior without reopening durability policy or review/finalization semantics
- creates a more stable local contract if asset-dispatch behavior keeps evolving

**Cons**

- needs discipline to avoid over-splitting a seam that may still be cohesive enough
- easy to chase symmetry instead of waiting for concrete pressure inside bundle/task shaping

**Recommendation:** `Yes`

This is the best `Phase 8` target.

### Option 2: Inventory durability policy

Keep the work feature-owned inside ListingKit and revisit the current best-effort persistence policy introduced into the inventory-persist seam:

- [internal/listingkit/workflow_platform_asset_dispatch_inventory_persist.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_inventory_persist.go:1)

This would likely mean:

- deciding whether inventory persistence failures should remain silent
- deciding whether durability should key off a richer signal than `returnedAssetCount`
- deciding whether partial durability deserves warnings or workflow issues

**Pros**

- could improve explicitness around failure handling and persistence semantics
- may matter if production signals show drift or silent durability failures

**Cons**

- currently more of a policy question than a proven code-ownership hotspot
- `Phase 7B` just clarified this seam and preserved existing behavior intentionally
- no evidence yet that the current best-effort policy is causing immediate design pressure

**Recommendation:** `Not yet`

This should wait until a real policy-change signal appears.

## Why Not Reopen Durability Policy First

`Phase 7B` already solved the real durability ownership root cause:

- the parent seam no longer owns `SaveInventory(...)`
- the durability handoff is explicit
- the gating signal is local and understandable
- boundary tests now protect the ownership line

What remains there is mostly:

- policy clarity
- richer failure semantics
- future-proofing

Those are important, but they are second-order right now.

By contrast, the mutation seam still combines several state-shaping responsibilities in one place, which is the stronger next design signal.

## Suggested Phase 8 Goal

The concrete `Phase 8` goal should be:

> Make ListingKit deferred asset-dispatch mutation shaping more explicit so `workflow_platform_asset_dispatch_apply.go` stops being the primary shared home of inventory mutation, bundle rebuild, and returned-task merge at the same time.

That goal is specific enough to implement incrementally and narrow enough to stay inside one real hotspot.

## Suggested Phase 8 Success Criteria

`Phase 8` should be considered successful when:

1. the mutation-side responsibilities in `workflow_platform_asset_dispatch_apply.go` are split or clarified behind more explicit local seams
2. inventory mutation and bundle/task shaping no longer rely on one undifferentiated mutation body
3. behavior tests still protect deferred dispatch success semantics unchanged in business terms
4. `workflow_platform_asset_dispatch_phase.go` does not regrow direct mutation logic during the work
5. no generic mutation framework is introduced unless another seam shows the same pressure

## Suggested Non-Goals For Phase 8

To keep the next slice disciplined, `Phase 8` should explicitly avoid:

- redesigning inventory durability policy
- changing deferred asset-generation business behavior
- revisiting summary/review/finalization seams
- introducing a generic workflow mutation abstraction
- moving workflow concerns into HTTP/runtime/bootstrap layers

## Expected File Hotspots

If we take the recommended direction, the first likely hotspots are:

- [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)
- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)
- [internal/listingkit/phase7a_asset_dispatch_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase7a_asset_dispatch_boundary_test.go:1)
- [internal/listingkit/phase7b_asset_dispatch_persist_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase7b_asset_dispatch_persist_boundary_test.go:1)

Possible new files, if the split is warranted, would likely stay feature-local, for example:

- `internal/listingkit/workflow_platform_asset_dispatch_inventory_apply.go`
- `internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go`

The design pressure should be:

- clearer mutation-side ownership
- explicit handoff between inventory and bundle/task shaping
- no speculative shared workflow framework

## Recommendation Summary

Proceed to `Phase 8`, but scope it narrowly:

- choose **ListingKit asset-dispatch mutation-side shaping pressure** as the next hotspot
- defer durability policy redesign until it shows stronger real change pressure
- keep the work fully feature-owned inside ListingKit

That keeps the next slice aligned with the strongest remaining in-memory state-shaping signal in the codebase rather than reopening a just-stabilized durability seam.
