# Task Processor Framework Phase 8B Checkpoint

## Status

`Phase 8B` is functionally complete for the intended slice.

This phase was not about reopening inventory-side shaping, durability policy, or broader workflow retry semantics. The goal was narrower:

1. stop [internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go:1) from remaining the shared implicit home of both platform bundle reshaping and returned-task merge semantics
2. make those two trigger paths explicit through separate feature-local seams
3. preserve existing `assets-only`, `tasks-only`, and `assets+tasks` deferred-dispatch behavior
4. lock the new ownership split so the old inline trigger logic does not silently grow back

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Bundle reshaping now has its own local seam

The bundle-reshape seam now lives in:

- [internal/listingkit/workflow_platform_asset_dispatch_bundle_reshape.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_bundle_reshape.go:1)

This seam now owns:

- `attachPlatformImageBundles(...)`
- the trigger path for rebuilding platform image bundles from deferred dispatch results

This matters because the root problem here was not file size. The risk was that one bundle-apply helper still carried both “reshape platform bundles” and “merge returned tasks” behind one implicit trigger model.

### 2. Returned-task merge now has its own local seam

The task-merge seam now lives in:

- [internal/listingkit/workflow_platform_asset_dispatch_task_merge.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_task_merge.go:1)

This seam now owns:

- `mergeGenerationTasks(...)`
- the no-op handoff when no returned tasks are present

That gives returned-task merge a clear local home instead of keeping it bundled together with platform bundle reshaping semantics.

### 3. The bundle-apply seam is now orchestration-focused

The bundle-apply entry still lives in:

- [internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go:1)

It now mainly coordinates:

1. bundle reshaping handoff
2. returned-task merge handoff

It no longer directly inlines:

- `attachPlatformImageBundles(...)`
- `mergeGenerationTasks(...)`

That is the main ownership outcome of this phase.

### 4. Runtime behavior remains covered across all three deferred-dispatch shapes

The behavior harness remains in:

- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)

This coverage still explicitly protects:

- `assets+tasks`
- `tasks-only`
- `assets-only`
- seam-level bundle reshape behavior
- seam-level task merge behavior

This matters because the point of the split was to clarify ownership without changing dispatch-result behavior.

### 5. Guardrails now lock the bundle/task-side trigger split

The ownership protections now live in:

- [internal/listingkit/phase8a_asset_dispatch_apply_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase8a_asset_dispatch_apply_boundary_test.go:1)
- [internal/listingkit/phase8b_asset_dispatch_bundle_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase8b_asset_dispatch_bundle_boundary_test.go:1)

These now protect four things:

1. `workflow_platform_asset_dispatch_bundle_apply.go` must delegate to the reshape and task-merge seams
2. `workflow_platform_asset_dispatch_bundle_apply.go` must not inline the old trigger logic
3. bundle reshaping stays inside the bundle-reshape seam
4. returned-task merge stays inside the task-merge seam

This is the main anti-regrowth protection for the phase.

## Acceptance Check

`Phase 8B` was meant to prove four things:

1. bundle reshaping and returned-task merge can live behind separate explicit local seams
2. the parent bundle-apply seam no longer needs one implicit trigger model for both responsibilities
3. deferred dispatch behavior can stay stable across `assets-only`, `tasks-only`, and `assets+tasks`
4. the new ownership split can be protected with source-boundary and behavior tests

All four are now true.

More concretely:

- `workflow_platform_asset_dispatch_bundle_apply.go` no longer owns the old inline trigger body
- bundle reshaping and returned-task merge now have separate local homes
- the runtime behavior harness stayed green across all three dispatch-result shapes
- boundary tests now protect both delegation and ownership

## What This Phase Did Not Try To Solve

### 1. It did not reopen inventory-side shaping

This phase deliberately did not revisit:

- inventory record mutation
- inventory summary rebuild
- generated-asset bundle refresh owned by the inventory-side seam

That was the right tradeoff. The concrete hotspot here was bundle/task-side trigger ownership, not the inventory seam.

### 2. It did not redesign durability policy

This phase did not reopen:

- `SaveInventory(...)`
- generation-task persistence policy
- broader deferred-dispatch persistence semantics

Those remain intentionally outside this slice.

### 3. It did not normalize every similar code path in the feature

The workflow deferred-dispatch path is now cleaner, but not every nearby path has been rewritten around these exact seams. In particular, retry-oriented generation paths still compose merge and bundle refresh logic more directly. That is a watchpoint for a future slice, not a failure of this one.

## Residual Responsibilities Still Present

### Boundary tests remain source-string guardrails

The ownership tests intentionally check:

- required delegation calls
- forbidden inline helper calls
- ownership markers in the new seam files

That is pragmatic and consistent with the current testing style, but helper-internal semantic drift can still require human review attention even when string guards stay green.

### Similar semantics still exist outside this exact workflow path

The `Phase 8B` split cleans up the deferred-dispatch workflow path. It does not yet guarantee that every retry or regeneration path in ListingKit now routes through the same seam structure.

That means the next slice should only continue here if we see concrete pressure around those neighboring paths, not because “Phase 8B” exists.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “keep carving this same seam for symmetry.” Better next steps are:

### 1. Watch whether bundle-side shaping pressure moves into retry or regeneration paths

If future changes keep landing around bundle/task-side shaping outside this deferred-dispatch workflow path, the next slice should be driven by that concrete pressure.

### 2. Leave this layer alone unless another real ownership hotspot appears

This layer is now in a good enough state:

- the parent apply seam delegates
- bundle reshaping is explicit
- task merge is explicit
- behavior stayed stable
- guardrails exist

Do not keep editing it for symmetry alone.

## Verification Summary

The final `Phase 8B` verification that passed on this branch was:

```powershell
go test ./internal/listingkit -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Additional focused verification that passed during the phase included:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchBundleReshapePhaseRun(ReshapesBundlesWithoutReturnedTasks|ReshapesBundlesWithReturnedTasks)$" -count=1
go test ./internal/listingkit -run "TestPlatformAssetDispatchTaskMergePhaseRun(MergesReturnedTasks|SkipsWhenNoReturnedTasks)$" -count=1
go test ./internal/listingkit -run "Test(WorkflowPlatformAssetDispatchBundleApplyFileDelegatesToTriggerSubSeams|PlatformAssetDispatchBundleApplyPhaseRun.*|ApplyPlatformAssetDispatchMutationShapesBundlesWhenDispatchReturns(TasksOnly|AssetsOnly))" -count=1
go test ./internal/listingkit -run "TestWorkflowPlatformAssetDispatch(BundleReshapeFileOwnsBundleReshaping|TaskMergeFileOwnsReturnedTaskMerge|ApplyFileDelegatesToMutationSubSeams|MutationSubSeamFilesOwnShapingResponsibilities|BundleApplyFileDelegatesToTriggerSubSeams)" -count=1
```

## Recommended Status

`Phase 8B` should be considered complete.

The bundle/task-side trigger ownership problem that motivated the phase has been addressed, the runtime behavior stayed green, the old inline trigger body has been split into explicit local seams, and the new ownership split is now protected. If we continue, the next step should begin with a new scope decision, not with more seam carving inside this same slice.
