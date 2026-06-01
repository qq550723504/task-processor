# Task Processor Framework Phase 8B Scope Recommendation

## Recommendation

`Phase 8B` should focus on **ListingKit bundle/task-side shaping pressure**, not on inventory-side shaping yet.

The highest-value next hotspot is:

- reassessing [internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go:1)
- reducing how much that seam still couples platform bundle reshaping and returned-task merge behind one local helper

In short:

- `Phase 8A` already split mutation-side shaping into `inventory-apply` and `bundle-apply`
- the only real regression uncovered during final review came from `bundle-apply`
- `Phase 8B` should target the seam that has already demonstrated the stronger behavior-pressure signal

## Why This Is The Right Next Step

After `Phase 8A`, the biggest remaining asymmetry is no longer:

- parent apply ownership
- mutation vs durability ownership
- inventory-side mutation extraction

Those are now on explicit seams with behavior and boundary guardrails.

The biggest remaining asymmetry is that one seam still combines two different concerns:

- [internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go:1)

This seam currently still decides, in one place:

1. when platform image bundles are reshaped after dispatch
2. when returned dispatch tasks should merge back into generation-task state

That is the next root-cause hotspot because those two behaviors have already shown different triggering semantics:

- bundle reshaping must still happen for `assets-only` dispatch results
- generation-task merge only matters when returned tasks exist

The last phase’s real regression happened exactly because those triggers were too tightly coupled.

## Current Hotspot

The main hotspot is:

- [internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go:1)

The stronger signal is not line count. The stronger signal is that:

- platform bundle reshaping is driven by the broader notion of “dispatch changed effective asset state”
- task merge is driven by the narrower notion of “dispatch returned task state”

Those are related, but not identical, and `Phase 8A` already proved they can drift if one seam owns both triggers without an explicit contract.

## Candidate Phase 8B Directions

There are two realistic directions from the current branch state.

### Option 1: Bundle/task-side shaping seam

Keep the work feature-owned inside ListingKit and make bundle reshaping and task merge triggers more explicit.

This would likely mean:

- separating bundle reshaping trigger semantics from returned-task merge semantics
- making it clearer why `assets-only`, `tasks-only`, and `assets+tasks` all still pass through bundle shaping differently
- reducing the chance that future edits accidentally re-bind bundle reshaping to task presence again
- preserving the current bounded-seam approach instead of introducing a generic platform bundle framework

**Pros**

- directly targets the seam that already produced a real behavior regression
- aligns with the “fix the demonstrated pressure point” principle instead of symmetrical cleanup
- should improve confidence in platform-side mutation semantics without reopening inventory or durability layers
- keeps work scoped to one feature-local seam

**Cons**

- needs care not to oversplit a seam that may still be acceptable after a smaller contract clarification
- easy to slip into platform-specific abstraction work that the codebase does not yet need

**Recommendation:** `Yes`

This is the best `Phase 8B` target.

### Option 2: Inventory-side shaping seam

Keep the work feature-owned inside ListingKit and further split:

- [internal/listingkit/workflow_platform_asset_dispatch_inventory_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_inventory_apply.go:1)

This would likely mean:

- separating inventory record mutation from `final.AssetBundle` refresh
- splitting returned-asset durability-facing projection from in-memory inventory mutation
- clarifying whether `final.AssetBundle` really belongs with inventory-side shaping long term

**Pros**

- could make ownership more explicit if returned-asset bundle projection starts changing rapidly
- may help if `final.AssetBundle` refresh becomes a separate hotspot later

**Cons**

- currently more of a potential micro-hotspot than a demonstrated regression hotspot
- `Phase 8A` explicitly found the real failure mode on the bundle/task side, not here
- easy to spend a lot of effort on symmetry before there is concrete pressure

**Recommendation:** `Not yet`

This should wait until inventory-side shaping shows stronger real change pressure.

## Why Not Prioritize Inventory-Side Shaping First

`Phase 8A` already left inventory-side shaping in a reasonably stable state:

- returned assets mutate inventory
- summary is rebuilt
- generated-asset bundle projection is refreshed

That grouping is still somewhat broad, but it has not yet demonstrated the same trigger mismatch that bundle/task-side shaping already did.

By contrast, `bundle-apply` already caused a production-path behavior regression during final review, which is a much stronger signal than “this seam might become awkward later.”

## Suggested Phase 8B Goal

The concrete `Phase 8B` goal should be:

> Make ListingKit bundle/task-side shaping semantics more explicit so `workflow_platform_asset_dispatch_bundle_apply.go` no longer relies on one implicit trigger model for both platform bundle reshaping and returned-task merge.

That goal is specific enough to implement incrementally and narrow enough to stay inside one real hotspot.

## Suggested Phase 8B Success Criteria

`Phase 8B` should be considered successful when:

1. the trigger semantics for bundle reshaping and task merge are more explicit
2. `assets-only`, `tasks-only`, and `assets+tasks` bundle-side behavior remain stable and clearly protected by tests
3. `workflow_platform_asset_dispatch_apply.go` does not regrow inlined bundle/task shaping logic
4. no generic platform bundle abstraction is introduced unless another seam shows the same pressure
5. inventory-side and durability-side seams remain untouched unless truly needed

## Suggested Non-Goals For Phase 8B

To keep the next slice disciplined, `Phase 8B` should explicitly avoid:

- redesigning inventory-side shaping
- redesigning durability policy
- revisiting summary/review/finalization seams
- introducing a generic mutation or bundle framework
- moving workflow concerns into HTTP/runtime/bootstrap layers

## Expected File Hotspots

If we take the recommended direction, the first likely hotspots are:

- [internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go:1)
- [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)
- [internal/listingkit/phase8a_asset_dispatch_apply_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase8a_asset_dispatch_apply_boundary_test.go:1)

Possible new files, if the split is warranted, would likely stay feature-local, for example:

- `internal/listingkit/workflow_platform_asset_dispatch_bundle_trigger.go`
- `internal/listingkit/workflow_platform_asset_dispatch_task_merge.go`

The design pressure should be:

- clearer bundle-side trigger ownership
- explicit distinction between bundle reshaping and task merge
- no speculative shared framework work

## Recommendation Summary

Proceed to `Phase 8B`, but scope it narrowly:

- choose **ListingKit bundle/task-side shaping pressure** as the next hotspot
- defer inventory-side reshaping until it shows stronger real change pressure
- keep the work fully feature-owned inside ListingKit

That keeps the next slice aligned with the strongest demonstrated mutation-side regression signal in the codebase rather than reopening a seam that is currently stable enough.
