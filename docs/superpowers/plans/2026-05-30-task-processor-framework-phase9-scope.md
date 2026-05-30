# Task Processor Framework Phase 9 Scope Recommendation

## Recommendation

`Phase 9` should focus on **ListingKit task-generation retry flow ownership**, not on continuing to sub-split the deferred asset-dispatch workflow path.

The highest-value next hotspot is:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:1)
- specifically the retry path rooted at `RetryTaskGenerationTasks(...)`

In short:

- `Phase 8A` and `Phase 8B` made the deferred asset-dispatch workflow path much clearer
- but the task-generation retry path still directly combines task merge, inventory mutation, durability writes, result rebuild, and platform bundle reattach in one service method
- `Phase 9` should target that remaining mixed-ownership retry pipeline instead of continuing to carve the already-stabilized workflow seam

## Why This Is The Right Next Step

After `Phase 8B`, the biggest remaining asymmetry is no longer:

- deferred asset-dispatch bundle/task trigger ownership
- boundary protection for the workflow dispatch path
- whether `workflow_platform_asset_dispatch_bundle_apply.go` still mixes reshape and merge semantics

Those are now on explicit feature-owned seams with source-boundary and behavior guardrails.

The biggest remaining asymmetry is that the retry path in:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:210)

still directly decides, in one place:

1. which generation tasks are retried
2. how returned dispatch tasks are merged back into persisted generation tasks
3. how generated assets replace prior target assets in inventory
4. when inventory summary is rebuilt
5. when inventory and generation tasks are durably saved
6. how `ListingKitResult` is rebuilt from the updated inventory
7. when platform image bundles are reattached from the updated task state
8. when result decoration and preview sync run

That is the next root-cause hotspot because it mixes:

- retry orchestration
- mutation semantics
- durability semantics
- result projection
- platform bundle shaping

inside one service-side execution path.

The strongest signal is not file size. The stronger signal is that the same semantic cluster we just clarified in workflow dispatch still exists in a neighboring retry path, but without the newer seam boundaries.

## Current Hotspot

The main hotspot is:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:210)

The clearest pressure zone is the block that currently does all of the following inline:

- `mergeGenerationTasks(existingTasks, dispatchResult.Tasks)`  
  [task_generation_service.go:251](/D:/code/task-processor/internal/listingkit/task_generation_service.go:251)
- `replaceGeneratedAssetsForTargets(...)`  
  [task_generation_service.go:257](/D:/code/task-processor/internal/listingkit/task_generation_service.go:257)
- `rebuildInventorySummary(inventory)`  
  [task_generation_service.go:258](/D:/code/task-processor/internal/listingkit/task_generation_service.go:258)
- `SaveInventory(...)` and `SaveGenerationTasks(...)`  
  [task_generation_service.go:260](/D:/code/task-processor/internal/listingkit/task_generation_service.go:260)
- `rebuildBundleFromInventory(...)` and `attachPlatformImageBundles(...)`  
  [task_generation_service.go:268](/D:/code/task-processor/internal/listingkit/task_generation_service.go:268)
- `decorateListingKitResultGeneration(...)` and `syncAssetRenderPreviews(...)`  
  [task_generation_service.go:272](/D:/code/task-processor/internal/listingkit/task_generation_service.go:272)

That is a first-order ownership signal, not just tidiness debt.

## Candidate Phase 9 Directions

There are two realistic directions from the current branch state.

### Option 1: Task-generation retry ownership seam

Keep the work feature-owned inside ListingKit and make the retry pipeline more explicit.

This would likely mean:

- separating retry dispatch-result mutation from retry durability writes
- separating retry result projection from retry orchestration
- reusing existing feature-local helpers and seam patterns where they truly fit
- keeping the retry slice local to `task_generation_service` rather than inventing a generic retry framework

**Pros**

- directly targets the densest remaining mixed-responsibility path near the work we just finished
- addresses a real neighboring ownership hotspot instead of forcing more symmetry into an already-stable workflow seam
- gives retry behavior a clearer local contract if generation retry keeps evolving
- creates a better boundary for future review of retry-specific business rules

**Cons**

- needs discipline to avoid blindly reusing workflow dispatch seams where retry semantics actually differ
- easy to over-abstract if we try to unify service retry flow and workflow dispatch flow too early

**Recommendation:** `Yes`

This is the best `Phase 9` target.

### Option 2: Continue splitting the workflow asset-dispatch layer

Keep working inside:

- [internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go:1)
- [internal/listingkit/workflow_platform_asset_dispatch_inventory_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_inventory_apply.go:1)

and continue looking for more internal symmetry.

This would likely mean:

- reopening bundle-side helper boundaries
- reopening inventory-side shaping even though no new pressure was shown there
- further refining source-boundary tests for helper ownership

**Pros**

- would keep work inside a context we already understand well
- might produce smaller helper files or stricter guardrails

**Cons**

- current pressure there is now second-order, not first-order
- risks optimizing for symmetry instead of real design pressure
- would leave the retry path as the more meaningful neighboring hotspot

**Recommendation:** `Not yet`

Do not continue there unless a new workflow-specific pressure point appears.

## Why Not Keep Carving The Workflow Dispatch Path First

`Phase 8A` and `Phase 8B` already solved the real ownership root cause in the deferred asset-dispatch workflow path:

- mutation-side shaping is explicit
- durability handoff is explicit
- bundle reshaping and task merge have separate local homes
- boundary tests now protect the split

What remains there is mostly:

- helper-internal symmetry
- further guardrail tightening
- future-proofing

Those are important, but they are second-order right now.

By contrast, the retry path still reassembles several of the same semantic categories inline, which is a stronger next design signal.

## Suggested Phase 9 Goal

The concrete `Phase 9` goal should be:

> Make ListingKit task-generation retry ownership more explicit so `task_generation_service.go` stops being the primary shared home of retry mutation, durability writes, and result projection at the same time.

That goal is specific enough to implement incrementally and narrow enough to stay inside one real hotspot.

## Suggested Phase 9 Success Criteria

`Phase 9` should be considered successful when:

1. the retry path no longer mixes mutation, durability, and rebuilt-result projection inside one undifferentiated service block
2. retry orchestration becomes more explicit in `task_generation_service.go`
3. behavior tests still protect retry success semantics unchanged in business terms
4. the workflow asset-dispatch seams from `Phase 8A/8B` do not regrow mixed responsibilities during the work
5. no generic retry or asset-generation framework is introduced unless a second feature shows the same pressure

## Suggested Non-Goals For Phase 9

To keep the next slice disciplined, `Phase 9` should explicitly avoid:

- redesigning retry business policy
- forcing the retry path to exactly match workflow asset-dispatch seam names
- reopening summary/review/finalization seams
- introducing a generic retry orchestration framework
- moving retry workflow concerns into HTTP/runtime/bootstrap layers

## Expected File Hotspots

If we take the recommended direction, the first likely hotspots are:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:1)
- [internal/listingkit/asset_workflow.go](/D:/code/task-processor/internal/listingkit/asset_workflow.go:1)
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)

Possible new files, if the split is warranted, would likely stay feature-local, for example:

- `internal/listingkit/task_generation_retry_mutation.go`
- `internal/listingkit/task_generation_retry_persist.go`
- `internal/listingkit/task_generation_retry_projection.go`
- `internal/listingkit/phase9_task_generation_retry_boundary_test.go`

The design pressure should be:

- clearer retry ownership
- explicit handoff between retry mutation, persistence, and result projection
- no speculative shared framework

## Recommendation Summary

Proceed to `Phase 9`, but scope it narrowly:

- choose **ListingKit task-generation retry flow ownership** as the next hotspot
- avoid reopening the already-stabilized workflow asset-dispatch seam without new pressure
- keep the work fully feature-owned inside ListingKit

That keeps the next slice aligned with the strongest remaining adjacent ownership signal in the codebase rather than continuing seam cleanup for symmetry alone.
