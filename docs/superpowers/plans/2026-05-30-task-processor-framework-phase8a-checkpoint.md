# Task Processor Framework Phase 8A Checkpoint

## Status

`Phase 8A` is functionally complete for the intended slice.

This phase was not about redesigning deferred asset-generation business policy, reopening durability semantics, or introducing a generic mutation framework. The goal was narrower:

1. stop [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1) from remaining the primary shared home of inventory mutation, bundle reshaping, and returned-task merge at the same time
2. make mutation-side shaping more explicit while keeping the work fully feature-owned inside ListingKit
3. preserve the existing deferred-dispatch behavior across `assets-only`, `tasks-only`, and `assets+tasks` result paths
4. lock the new mutation-side ownership split so logic does not silently grow back into the parent apply seam

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Inventory-side apply now has its own seam

The inventory-side apply seam now lives in:

- [internal/listingkit/workflow_platform_asset_dispatch_inventory_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_inventory_apply.go:1)

This seam now owns:

- returned-asset append into `inventory.Records`
- inventory summary rebuild
- generated-asset rebuild into `final.AssetBundle`
- `final.AssetInventorySummary` refresh

That matters because the root problem here was not file size. The risk was that one mutation helper still simultaneously shaped inventory state, display bundle state, and generation-task state.

### 2. Bundle/task-side apply now has its own seam

The bundle/task apply seam now lives in:

- [internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go:1)

This seam now owns:

- `attachPlatformImageBundles(...)`
- `mergeGenerationTasks(...)`

The important final-state fix in this phase is that bundle reshaping is no longer incorrectly tied to “returned tasks exist.” The seam now still reshapes platform bundles for `assets-only` dispatch results, preserving the pre-split behavior.

### 3. The parent apply seam is now orchestration-focused

The apply entry still lives in:

- [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)

It now mainly coordinates:

1. inventory-side apply
2. bundle/task-side apply
3. mutation result assembly

It no longer directly inlines:

- `inventory.Records = append(...)`
- `rebuildInventorySummary(...)`
- `rebuildBundleWithGeneratedAssets(...)`
- `attachPlatformImageBundles(...)`
- `mergeGenerationTasks(...)`

That is the main ownership outcome of the phase.

### 4. Mutation-side behavior is now covered across all three dispatch-result shapes

The behavior harness remains in:

- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)

This coverage now explicitly protects:

- `assets+tasks` mutation behavior
- `tasks-only` mutation behavior
- `assets-only` mutation behavior
- nil/empty dispatch no-op behavior

The `assets-only` path is especially important because it exposed a real regression during final slice review and was corrected before checkpointing.

### 5. Guardrails now lock the mutation-side ownership split

The new and updated protections live in:

- [internal/listingkit/phase7a_asset_dispatch_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase7a_asset_dispatch_boundary_test.go:1)
- [internal/listingkit/phase8a_asset_dispatch_apply_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase8a_asset_dispatch_apply_boundary_test.go:1)

These now protect four things:

1. `workflow_platform_asset_dispatch_apply.go` must delegate to the two sub-seams
2. inventory-side shaping stays inside the inventory-apply seam
3. bundle/task-side shaping stays inside the bundle-apply seam
4. the parent orchestration seam stays free of the old inlined mutation body

This is the main anti-regrowth protection for the phase.

## Acceptance Check

`Phase 8A` was meant to prove four things:

1. mutation-side shaping can split behind explicit local seams
2. inventory shaping and bundle/task shaping no longer need to live inside one undifferentiated helper body
3. deferred dispatch behavior can stay stable across `assets-only`, `tasks-only`, and `assets+tasks`
4. the new ownership split can be protected with source-boundary and behavior tests

All four are now true.

More concretely:

- `workflow_platform_asset_dispatch_apply.go` no longer owns all mutation categories inline
- inventory shaping and bundle/task shaping now have separate local homes
- the `assets-only` reshape regression found during final review has been fixed
- tests now protect both mutation behavior and ownership boundaries

## What This Phase Did Not Try To Solve

### 1. It did not redesign durability policy

This phase deliberately did not reopen:

- inventory persistence policy
- best-effort `SaveInventory(...)` semantics
- generation-task persistence policy

That was the right tradeoff. The actual hotspot was mutation-side ownership, not durability policy.

### 2. It did not introduce a generic mutation framework

All new seams remain local to ListingKit:

- inventory-apply
- bundle-apply
- parent apply orchestration

That is appropriate here. The pressure was concentrated inside one feature seam, not spread across the repo.

### 3. It did not eliminate orchestration from the parent apply seam

[internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)

still owns:

- dispatch-result handoff ordering
- mutation result assembly

That is intentional. The goal was to stop it being the primary home of all mutation categories, not to reduce it to zero lines.

## Residual Responsibilities Still Present

### Inventory-side shaping still owns both inventory mutation and generated-asset bundle refresh

The inventory-apply seam currently owns:

- inventory record mutation
- inventory summary mutation
- `final.AssetBundle` refresh for returned assets

That is acceptable for now because these changes are still tightly coupled to “returned assets changed concrete inventory state.” It only becomes a new hotspot if later changes start pulling `final.AssetBundle` refresh away from the rest of returned-asset shaping.

### Boundary tests remain string-based

The source-boundary protections intentionally check:

- required delegation calls
- forbidden inlined mutation snippets
- ownership markers in the new seam files

That is pragmatic and consistent with the existing testing style, but harmless renames or structural rewrites will still require test updates.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “keep carving this exact seam for symmetry.” Better next steps are:

### 1. Watch whether inventory-side shaping becomes a new micro-hotspot

If future changes keep landing around:

- returned-asset bundle refresh
- inventory summary shaping
- inventory-side result projection

inside:

- [internal/listingkit/workflow_platform_asset_dispatch_inventory_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_inventory_apply.go:1)

then the next slice should be driven by concrete new pressure there, not by symmetry alone.

### 2. Reassess whether bundle/task-side shaping starts carrying more platform-specific logic

If later changes start pushing more platform-specific decisions into:

- [internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go:1)

then the next slice may need to separate bundle reshaping from task merge semantics more explicitly.

### 3. Leave this layer alone unless another concrete mutation-side pressure point appears

This layer is now in a good enough state:

- parent apply orchestration is explicit
- inventory-side shaping is explicit
- bundle/task-side shaping is explicit
- durability and generation-task persistence seams remain separate
- guardrails exist

Do not keep editing it for symmetry alone.

## Verification Summary

The final `Phase 8A` verification that passed on this branch was:

```powershell
go test ./internal/listingkit -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Additional focused verification that passed during the phase included:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchInventoryApplyPhaseRun(MergesReturnedAssets|SkipsWhenNoReturnedAssets)$" -count=1
go test ./internal/listingkit -run "TestPlatformAssetDispatchBundleApplyPhaseRun(ReattachesBundlesAndMergesTasks|SkipsWhenNoReturnedTasks|ReattachesBundlesWhenNoReturnedTasks)$" -count=1
go test ./internal/listingkit -run "TestApplyPlatformAssetDispatchMutation(MergesDispatchArtifacts|KeepsGenerationTasksWhenDispatchResultNil|ShapesBundlesWhenDispatchReturnsTasksOnly|ShapesBundlesWhenDispatchReturnsAssetsOnly)$" -count=1
go test ./internal/listingkit -run "TestWorkflowPlatformAssetDispatch(ApplyFileDelegatesToMutationSubSeams|MutationSubSeamFilesOwnShapingResponsibilities|PhaseFileDelegatesToOrchestrationHelpers|MutationAndPersistFilesOwnTheirSideEffects)$" -count=1
```

## Recommended Status

`Phase 8A` should be considered complete.

The mutation-side ownership problem that motivated the phase has been addressed, the remaining seams are clearer, the real regression uncovered during final review has been fixed, behavior stayed green, and the new split is now protected. If we continue, the next step should begin with a new scope decision, not with more opportunistic seam carving inside this same slice.
