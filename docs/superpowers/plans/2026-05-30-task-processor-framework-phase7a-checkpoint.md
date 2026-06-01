# Task Processor Framework Phase 7A Checkpoint

## Status

`Phase 7A` is functionally complete for the intended slice.

This phase was not about redesigning ListingKit workflow policy, changing deferred generation business rules, or introducing a generic mutation container. The goal was narrower:

1. stop [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1) from remaining the mixed home of dispatch execution, inventory mutation, bundle rebuild, generation-task merge, decoration, and persistence timing
2. make deferred asset-dispatch state transitions explicit while staying fully feature-owned inside ListingKit
3. preserve the already-validated deferred dispatch behavior and finalization ordering
4. lock the new ownership split so mutation and persistence logic do not silently grow back into the parent seam

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Dispatch-result mutation now has its own seam

The mutation seam now lives in:

- [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)

This seam now owns:

- inventory record merge after deferred dispatch returns assets
- inventory summary rebuild
- `final.AssetBundle` rebuild with generated assets
- `final.AssetInventorySummary` refresh
- generation-task merge after dispatch returns task updates
- platform image-bundle reattachment based on returned dispatch tasks

That matters because the root problem here was not file count. The risk was that dispatch-result mutation of:

- `final`
- `inventory`
- persisted generation tasks

was hidden inside the same method body that also performed dispatch execution and persistence timing.

### 2. Decoration and generation-task persistence now have their own seam

The persistence seam now lives in:

- [internal/listingkit/workflow_platform_asset_dispatch_persist.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_persist.go:1)

This seam now owns:

- `decorateListingKitResultGeneration(...)`
- `SaveGenerationTasks(...)`
- warning shaping when generation-task persistence fails
- workflow issue shaping for `asset_generation_task_persistence_failed`

This is the other half of the root-cause fix. Persist timing is no longer mixed into the same block that mutates inventory and platform bundles.

### 3. Parent asset-dispatch phase is now orchestration-focused

The orchestration entry is still:

- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)

It now mainly coordinates:

1. pre-attach bundle setup
2. pending platform-task collection
3. deferred dispatch execution plus mutation handoff
4. persistence/decorate handoff

Concretely, the parent seam now routes work through:

- `p.preAttachBundles(...)`
- `p.dispatchAndApply(...)`
- `p.persistHandoff(...)`

That is the intended shape for this layer: a visible feature-owned orchestration seam, with mutation and persistence side effects living in smaller dedicated homes.

### 4. Behavior coverage stayed attached to the real workflow seam

The behavior harness remains in:

- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)

This coverage now explicitly protects:

- direct mutation behavior after dispatch assets/tasks return
- nil-safe mutation behavior
- decoration and persistence timing
- warning/issue behavior when generation-task persistence fails
- parent dispatch seam orchestration behavior end to end

That matters because source-boundary tests alone would not catch silent behavior drift in deferred dispatch output shaping.

### 5. Source-boundary guardrails now lock the ownership split

The new boundary protections live in:

- [internal/listingkit/phase7a_asset_dispatch_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase7a_asset_dispatch_boundary_test.go:1)

These now protect three things:

1. `workflow_platform_asset_dispatch_phase.go` keeps orchestration order as `pre-attach -> collect -> dispatch/apply -> persist handoff`
2. mutation-only operations stay in `workflow_platform_asset_dispatch_apply.go`
3. decoration/persistence-only operations stay in `workflow_platform_asset_dispatch_persist.go`

This is the main anti-regrowth protection for the phase.

## Acceptance Check

`Phase 7A` was meant to prove four things:

1. deferred dispatch mutation can move behind one explicit ListingKit-owned seam
2. decorate/persist timing can move behind a separate ListingKit-owned seam
3. the parent dispatch seam can shrink to orchestration without changing behavior
4. the new split can be protected with narrow source and behavior guardrails

All four are now true.

More concretely:

- `workflow_platform_asset_dispatch_phase.go` no longer directly owns the heaviest mutation and persistence side effects
- mutation and persistence responsibilities now have feature-local homes with clearer intent
- existing deferred dispatch behavior remains covered by behavior tests
- boundary tests now make ownership drift much harder to reintroduce silently

## What This Phase Did Not Try To Solve

### 1. It did not redesign summary/review or finalization policy

This phase deliberately stayed scoped to deferred asset-dispatch mutation and persistence timing.

It did not re-open:

- review compatibility semantics
- summary completion semantics
- finalize orchestration ordering beyond the dispatch seam itself

That was the right tradeoff. The real hotspot after `Phase 6B` was local dispatch-side ownership, not finalization policy.

### 2. It did not introduce a generic mutation/result framework

The new seams are intentionally local to ListingKit:

- one mutation seam
- one persistence seam
- one parent orchestration seam

That is appropriate here. The actual pressure was concentrated in one feature seam, not spread across the codebase.

### 3. It did not eliminate orchestration from the parent dispatch phase

[internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)

still owns:

- pre-attach timing
- dispatch execution timing
- mutation/persist handoff order

That is intentional. The goal was to stop it being the primary home of all downstream side effects, not to collapse it into zero lines.

## Residual Responsibilities Still Present

### Inventory persistence after returned assets still happens in the parent seam

The parent phase still decides when to call:

- `p.service.assetRepo.SaveInventory(...)`

after dispatch assets are applied.

That is acceptable for now because this slice focused on separating:

- state mutation
- generation-task decoration/persistence

not every possible persistence branch.

### Mutation seam still coordinates both inventory and bundle shaping

[internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)

now owns multiple mutation-side effects at once:

- inventory updates
- bundle rebuild
- task merge

That is still a good tradeoff for this phase because those changes represent one cohesive “apply returned dispatch result” step. It only becomes a problem if later changes start applying pressure to those sub-responsibilities separately.

### Boundary tests are still string-based

The source-boundary protections intentionally check:

- required orchestration calls
- forbidden regrowth calls
- file ownership of key side-effect operations

That is pragmatic and effective against ownership drift, but harmless renames or structural rewrites will still require test updates.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “split more lines just because the seam is cleaner now.” Better next steps are:

### 1. Reassess the remaining inventory persistence decision

If future changes keep landing around:

- `SaveInventory(...)`
- dispatch asset persistence timing
- inventory durability rules

inside:

- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)

then the next slice may need a more explicit inventory-persist handoff.

### 2. Watch whether mutation-side shaping becomes a second hotspot

If later changes keep crossing:

- asset bundle rebuild
- platform image-bundle reattachment
- generation-task merge

inside:

- [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)

then the next slice should be driven by concrete mutation-pressure evidence, not by symmetry alone.

### 3. Leave the dispatch seam alone unless another real pressure point appears

This layer is now in a good enough state:

- mutation is explicit
- persistence timing is explicit
- orchestration is visible
- guardrails exist

Do not keep editing it for symmetry alone.

## Verification Summary

The final `Phase 7A` verification that passed on this branch was:

```powershell
go test ./internal/listingkit -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Additional focused verification that passed during the phase included:

```powershell
go test ./internal/listingkit -run "TestApplyPlatformAssetDispatchMutation|TestRunWorkflowPersistsAssetInventoryAndBuildsPlatformBundles" -count=1
go test ./internal/listingkit -run "TestPlatformAssetDispatchPersistPhaseRun|TestApplyPlatformAssetDispatchMutation|TestRunWorkflowPersistsAssetInventoryAndBuildsPlatformBundles" -count=1
go test ./internal/listingkit -run "TestPlatformAssetDispatchPhaseRunOrchestratesDispatchMutationAndPersistence|TestWorkflowPlatformAssetDispatchPhaseFileDelegatesToOrchestrationHelpers|TestPlatformAssetDispatchPersistPhaseRun|TestApplyPlatformAssetDispatchMutation|TestRunWorkflowPersistsDeferredPlatformDispatchOutputs|TestRunWorkflowRecordsDeferredAssetGenerationDispatchFailure" -count=1
go test ./internal/listingkit -run "TestWorkflowPlatformAssetDispatchPhaseFileDelegatesToOrchestrationHelpers|TestWorkflowPlatformAssetDispatchMutationAndPersistFilesOwnTheirSideEffects|TestPlatformAssetDispatchPhaseRunOrchestratesDispatchMutationAndPersistence|TestPlatformAssetDispatchPersistPhaseRunDecoratesAndPersistsGenerationTasks|TestApplyPlatformAssetDispatchMutationMergesDispatchArtifacts" -count=1
```

## Recommended Status

`Phase 7A` should be considered complete.

The mutation/persist ownership problem that motivated the phase has been addressed, the parent seam is thinner, behavior stayed green, and the new split is now protected. If we continue, the next step should begin with a new scope decision, not with more opportunistic seam carving inside this same slice.
