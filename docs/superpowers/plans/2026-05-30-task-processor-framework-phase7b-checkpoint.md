# Task Processor Framework Phase 7B Checkpoint

## Status

`Phase 7B` is functionally complete for the intended slice.

This phase was not about redesigning ListingKit deferred generation policy, splitting mutation semantics again, or introducing a generic durability framework. The goal was narrower:

1. stop [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1) from remaining the primary home of the “returned assets should now be durably saved” decision
2. make inventory durability after deferred dispatch explicit and feature-owned inside ListingKit
3. preserve the current best-effort `SaveInventory(...)` behavior
4. lock the new ownership split so inventory persistence does not silently grow back into the parent seam

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Inventory durability now has its own seam

The new inventory-persist seam lives in:

- [internal/listingkit/workflow_platform_asset_dispatch_inventory_persist.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_inventory_persist.go:1)

This seam now owns:

- the post-dispatch durability decision for inventory persistence
- the best-effort `SaveInventory(ctx, inventory)` call
- the gating rule that durability only runs when returned asset count is greater than zero

That matters because the root problem here was not line count. The risk was that the parent dispatch seam still directly owned a durability decision even after mutation and generation-task persistence had already moved behind separate seams.

### 2. Parent asset-dispatch phase is now more purely orchestration-focused

The parent seam still lives in:

- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)

It now visibly coordinates:

1. `preAttachBundles(...)`
2. pending platform-task collection
3. `dispatchAndApply(...)`
4. `persistInventory(...)`
5. `persistHandoff(...)`

The parent seam no longer directly contains:

- `_ = p.service.assetRepo.SaveInventory(ctx, inventory)`

That is the key ownership outcome of this phase.

### 3. Inventory-persist contract is now narrower than dispatch-result shape

The inventory durability seam now takes:

- `inventory`
- `returnedAssetCount`

instead of the whole `*assetgeneration.Result`.

This is the real contract improvement of the phase. The persistence seam no longer depends on task payloads or full dispatch-result structure just to decide whether to save inventory.

### 4. Existing mutation and generation-task persistence seams stayed separate

The mutation seam still lives in:

- [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)

and the generation-task persist seam still lives in:

- [internal/listingkit/workflow_platform_asset_dispatch_persist.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_persist.go:1)

After `Phase 7B`, the split is now:

- apply seam: in-memory mutation only
- inventory-persist seam: inventory durability only
- persist seam: generation-task decoration and generation-task persistence only

That is the intended bounded shape for this workflow slice.

### 5. Guardrails now lock inventory-persist ownership

The new and updated protections live in:

- [internal/listingkit/phase7a_asset_dispatch_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase7a_asset_dispatch_boundary_test.go:1)
- [internal/listingkit/phase7b_asset_dispatch_persist_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase7b_asset_dispatch_persist_boundary_test.go:1)
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)

These now protect three things:

1. parent dispatch orchestration must route durability through `p.persistInventory(...)`
2. `SaveInventory(ctx, inventory)` ownership stays inside the inventory-persist seam
3. inventory durability only triggers when returned assets are present

This is the main anti-regrowth protection for the phase.

## Acceptance Check

`Phase 7B` was meant to prove four things:

1. inventory durability after deferred dispatch can move behind one explicit ListingKit-owned seam
2. the parent dispatch seam can become more orchestration-focused without changing business behavior
3. the durability seam can depend on a narrower contract than the full dispatch result
4. the new ownership split can be protected with source-boundary and behavior tests

All four are now true.

More concretely:

- parent dispatch no longer directly calls `SaveInventory(...)`
- inventory durability has one explicit local home
- generation-task persistence and dispatch-result mutation remain separate concerns
- tests now cover both durability behavior and ownership boundaries

## What This Phase Did Not Try To Solve

### 1. It did not redesign dispatch-result mutation semantics

This phase deliberately left:

- inventory record merge
- bundle rebuild
- generation-task merge
- platform image-bundle reattach

inside the existing apply seam.

That was the right tradeoff. The real hotspot after `Phase 7A` was the remaining durability decision in the parent seam, not mutation semantics themselves.

### 2. It did not change best-effort inventory persistence policy

The new seam still swallows persistence errors and does not emit new warnings or workflow issues.

That is intentional. This phase was about ownership clarity, not policy redesign.

### 3. It did not eliminate orchestration from the parent dispatch phase

[internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)

still owns:

- dispatch timing
- mutation handoff timing
- inventory durability handoff timing
- generation-task persistence handoff timing

That is acceptable. The goal was to stop it being the primary home of durability logic, not to reduce it to zero lines.

## Residual Responsibilities Still Present

### Inventory durability still depends on returned asset count as a local proxy

The inventory-persist seam gates on:

- `returnedAssetCount > 0`

That is a good contract for this slice, but it is still a local proxy rather than a richer domain signal.

That is acceptable for now because the existing behavior already uses “dispatch returned assets” as the durability trigger, and no second seam currently needs the same abstraction.

### Boundary tests remain string-based

The source-boundary protections intentionally check:

- required handoff calls
- forbidden inline `SaveInventory(...)`
- required ownership markers in the new seam file

That is pragmatic and consistent with the existing test style, but harmless renames will still require test updates.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “keep carving this same seam for symmetry.” Better next steps are:

### 1. Watch whether mutation-side shaping becomes the next real hotspot

If future changes keep landing across:

- bundle rebuild
- image-bundle reattach
- generation-task merge

inside:

- [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)

then the next slice should be driven by concrete mutation-side pressure, not by aesthetics.

### 2. Reassess whether best-effort inventory durability policy itself becomes unstable

If later changes start touching:

- persistence failure handling
- inventory durability timing
- partial durability semantics

then the next slice may need to focus on policy, not just ownership.

### 3. Leave this layer alone unless another concrete pressure point appears

This layer is now in a good enough state:

- mutation is explicit
- inventory durability is explicit
- generation-task persistence is explicit
- orchestration is visible
- guardrails exist

Do not keep editing it for symmetry alone.

## Verification Summary

The final `Phase 7B` verification that passed on this branch was:

```powershell
go test ./internal/listingkit -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Additional focused verification that passed during the phase included:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchInventoryPersistPhaseRun(PersistsReturnedAssets|SkipsWhenNoReturnedAssets|KeepsBestEffortPersistence)" -count=1
go test ./internal/listingkit -run "TestPlatformAssetDispatch(InventoryPersistPhaseRun|PhaseRunOrchestratesDispatchMutationAndPersistence|PhaseSourceRoutesInventoryPersistenceThroughHandoff)" -count=1
go test ./internal/listingkit -run "TestWorkflowPlatformAssetDispatch|TestPlatformAssetDispatchInventoryPersistPhaseRun" -count=1
go test ./internal/listingkit -run "TestPlatformAssetDispatch|TestWorkflowPlatformAssetDispatch|TestRunWorkflowPersistsAssetInventoryAndBuildsPlatformBundles" -count=1
```

## Recommended Status

`Phase 7B` should be considered complete.

The durability-ownership problem that motivated the phase has been addressed, the parent seam is thinner, behavior stayed green, and the new split is now protected. If we continue, the next step should begin with a new scope decision, not with more opportunistic seam carving inside this same slice.
