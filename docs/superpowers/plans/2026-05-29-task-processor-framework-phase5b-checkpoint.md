# Task Processor Framework Phase 5B Checkpoint

## Status

`Phase 5B` is functionally complete for the intended slice.

This phase was not about inventing a generic workflow engine or redesigning ListingKit business semantics. The goal was narrower:

1. stop canonical acquisition, media/SDS branching, asset planning, and platform finalization from remaining crossed together inside `workflow_standard.go` and `workflow_platform_adaptation.go`
2. make each execution branch flow through an explicit ListingKit-owned phase seam
3. preserve the current workflow recorder, snapshot, and result-assembly model
4. lock the new ownership split so the public workflow entry files do not silently regrow branch-heavy bodies

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Canonical acquisition now flows through a dedicated phase seam

The canonical phase now lives in:

- [internal/listingkit/workflow_standard_canonical_phase.go](/D:/code/task-processor/internal/listingkit/workflow_standard_canonical_phase.go:1)

This seam now owns canonical acquisition across:

- SDS baseline lookup
- studio fallback
- canonical cache reuse
- product-enrich fallback
- canonical cache persistence after enrich

The public standard workflow entry still lives in:

- [internal/listingkit/workflow_standard.go](/D:/code/task-processor/internal/listingkit/workflow_standard.go:1)

but it now delegates canonical acquisition through:

- `buildStandardWorkflowCanonicalPhase(s).run(...)`

That matters because the root ownership bug here was not file size alone. The risk was that baseline precedence, studio fallback, cache reuse, and enrich fallback were still intertwined inside the same workflow entry body that also had to orchestrate later stages.

### 2. Media processing and SDS sync now flow through a dedicated phase seam

The media phase now lives in:

- [internal/listingkit/workflow_standard_media_phase.go](/D:/code/task-processor/internal/listingkit/workflow_standard_media_phase.go:1)

This seam now owns:

- image task creation and processing
- local SDS sync after image processing
- remote SDS sync fallback when image processing is not active
- SDS metadata re-application onto canonical product

`workflow_standard.go` now delegates this branch through:

- `buildStandardWorkflowMediaPhase(s).run(...)`

This is the main execution-branching improvement for the middle of the workflow. Media/SDS logic is still feature-owned inside ListingKit, but it is no longer primarily expressed as one large conditional region embedded beside canonical acquisition and asset planning.

### 3. Asset planning and inventory persistence now flow through a dedicated standard-phase seam

The asset phase now lives in:

- [internal/listingkit/workflow_standard_asset_phase.go](/D:/code/task-processor/internal/listingkit/workflow_standard_asset_phase.go:1)

This seam now owns:

- asset inventory creation and persistence
- baseline asset generation
- platform recipe preparation
- platform asset dispatch planning
- generation-task persistence for the standard workflow path

`workflow_standard.go` now delegates this branch through:

- `buildStandardWorkflowAssetPhase(s).run(...)`

That matters because inventory persistence and asset-generation staging were previously mixed into the same file that also owned canonical and media/SDS branching. They now have one explicit feature-owned home.

### 4. Platform finalization now flows through a dedicated adaptation-phase seam

The platform finalization seam now lives in:

- [internal/listingkit/workflow_platform_finalize_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_finalize_phase.go:1)

This seam now owns:

- SHEIN post-assembly optimization and default pricing shaping
- SDS and official-image post-processing
- review decoration
- deferred platform asset dispatch
- generation metadata and summary finalization

The adaptation entry still lives in:

- [internal/listingkit/workflow_platform_adaptation.go](/D:/code/task-processor/internal/listingkit/workflow_platform_adaptation.go:1)

but it now delegates the heavy finalization branch through:

- `buildPlatformFinalizePhase(s).run(...)`

This is the other half of the ownership fix. Platform adaptation still owns adaptation entry and snapshot application, but the largest finalization body is no longer the implicit home of every downstream platform-specific rule.

### 5. Guardrails now lock the workflow execution phase split

The new boundary protections live in:

- [internal/listingkit/phase5b_workflow_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase5b_workflow_boundary_test.go:1)
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)
- [internal/listingkit/workflow_studio_sds_metadata_test.go](/D:/code/task-processor/internal/listingkit/workflow_studio_sds_metadata_test.go:1)

These checks now explicitly protect both sides of the seam:

1. `workflow_standard.go` continues to delegate canonical/media/asset execution through explicit phase builders
2. `workflow_platform_adaptation.go` continues to delegate finalization through the dedicated phase seam
3. canonical cache reuse, SDS baseline precedence, studio fallback, remote SDS sync, asset persistence, and finalization behavior still match the prior workflow semantics

## Acceptance Check

`Phase 5B` was meant to prove four things:

1. canonical acquisition can move behind one explicit ListingKit-owned seam
2. media/SDS branching can move behind one explicit ListingKit-owned seam
3. asset planning and platform finalization can move behind explicit phase seams
4. the new ownership split can be protected with narrow source and behavior guardrails

All four are now true.

More concretely:

- standard workflow entry files are thinner and more orchestration-focused
- execution branching now has feature-owned seam files
- current workflow behavior stayed stable
- ListingKit package tests still pass without regressions

## What This Phase Did Not Try To Solve

### 1. It did not introduce a generic workflow framework

This phase deliberately reused the mature local pattern already present in ListingKit:

- `newWorkflowRecorder(...)`
- `StandardProductSnapshot`
- existing workflow stub services and tests

That is the right tradeoff. The root problem was crossed ownership inside ListingKit workflow execution, not the absence of a repo-wide engine.

### 2. It did not redesign business-stage ordering

The workflow still follows the same high-level sequence:

- canonical acquisition
- media/SDS handling
- asset planning
- platform adaptation and finalization

This phase made those stages explicit without changing their business order or retry semantics.

### 3. It did not move workflow concerns out of ListingKit

The seams remain feature-owned in `internal/listingkit`.

That is intentional. `Phase 5B` was about execution-branch clarity inside ListingKit, not about moving workflow logic into HTTP, runtime, or a shared framework package.

## Residual Responsibilities Still Present

### `workflow_standard.go` and `workflow_platform_adaptation.go` still own public orchestration

These files still coordinate:

- recorder lifecycle
- result/snapshot plumbing
- transition between phase seams
- public feature entry semantics

That is acceptable for this phase because public workflow entry should still have one visible home.

### Phase seams still reuse existing helper behavior internally

The new phase files are better ownership boundaries, but they are not a business rewrite. They still depend on existing service helpers and current collaboration patterns.

That is also acceptable. The goal here was to stop crossed execution ownership first, not to redesign every downstream helper in one slice.

### Platform finalization remains a meaningful hotspot

- [internal/listingkit/workflow_platform_finalize_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_finalize_phase.go:1)

is now the main concentration point for ListingKit’s post-assembly platform shaping.

That is a good outcome for now because this behavior needed one explicit home first. If future changes keep landing there, the next slice should be driven by concrete pressure inside that seam, not by symmetry concerns elsewhere.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “extract more helpers because the workflow files are smaller now.” Better next steps are:

### 1. Watch whether the finalization seam becomes behaviorally overloaded

If future changes keep landing in:

- [internal/listingkit/workflow_platform_finalize_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_finalize_phase.go:1)

then the next slice should be driven by real platform-finalization hotspots, for example whether SHEIN-specific shaping and deferred asset-dispatch concerns need separate collaborators.

### 2. Reassess whether canonical/media/asset seams begin to share a second wave of context pressure

If future changes start crossing:

- canonical acquisition
- SDS/media hydration
- asset planning

again, then the next slice should focus on the shared execution context between seams instead of continuing to thin the entry files for appearance alone.

### 3. Leave this layer alone unless another concrete ownership problem appears

This workflow layer is now in a good enough state:

- explicit phase seams exist
- public entry files are thinner
- guardrails exist
- behavior remained stable

Do not keep editing it for symmetry alone.

## Verification Summary

The final `Phase 5B` verification that passed on this branch was:

```powershell
go test ./internal/listingkit -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Additional focused verification that passed during the phase included:

```powershell
go test ./internal/listingkit -run "TestRunStandardProductWorkflow(UsesSDSBaselineBeforeProductEnrich|UsesTaskTenantIDWhenRequestTenantMissing|FallsBackToStudioCanonicalWhenSDSBaselineMissing|ReusesCanonicalCacheBeforeProductEnrich|ContinuesWhenSDSBaselineLookupErrors|IgnoresUnavailableOrMalformedSDSBaselineEntries)" -count=1
go test ./internal/listingkit -run "Test(RunWorkflowPersistsAssetInventoryAndBuildsPlatformBundles|RunStandardProductWorkflowReappliesSDSMetadataWithoutDroppingProcessedAssets|RunStandardProductWorkflowRunsRemoteSDSSyncWhenImageProcessingIsDisabled|RunWorkflowRecordsDegradedImageStageWhenImageProcessingFails)" -count=1
go test ./internal/listingkit -run "Test(RunWorkflowPersistsAssetInventoryAndBuildsPlatformBundles|RunWorkflowRecordsDeferredAssetGenerationDispatchFailure|RunStandardProductWorkflowRunsRemoteSDSSyncWhenImageProcessingIsDisabled)" -count=1
```

Notes:

- `./internal/listingkit/...` already covers `httpapi` and `temporal`, but the explicit final run was kept as a focused confirmation for the integration seams this phase could have affected.

## Current Branch Notes

The main `Phase 5B` commits are:

- `e861a00b` `docs: add framework phase5b plan`
- `a9e3cf90` `refactor: extract listingkit canonical workflow phase`
- `8ddc03dd` `refactor: extract listingkit media workflow phase`
- `e25b985c` `refactor: extract listingkit workflow asset and finalize phases`
- `9b9bb953` `test: lock listingkit workflow phase boundaries`

## Recommendation

Mark `Phase 5B` complete.

Do not keep working this seam just because the workflow phase files are now easier to edit. The main ownership bug this phase addressed is already fixed:

- ListingKit canonical acquisition, media/SDS branching, asset planning, and platform finalization no longer primarily live as crossed execution bodies inside `workflow_standard.go` and `workflow_platform_adaptation.go`

If we continue, the better next step is to choose a new hotspot based on where behavior-level changes are actually landing, most likely around platform-finalization pressure or cross-phase execution context, not more phase-seam symmetry cleanup.
