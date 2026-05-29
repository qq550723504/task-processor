# Task Processor Framework Phase 5B ListingKit Workflow Execution Phases Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make ListingKit workflow execution branch through explicit phase-owned seams so canonical acquisition, media/SDS sync, asset planning, and platform adaptation stop remaining intertwined inside `workflow_standard.go` and `workflow_platform_adaptation.go`.

**Architecture:** Reuse the repo’s existing local patterns instead of inventing a generic workflow framework: keep `newWorkflowRecorder(...)`, `StandardProductSnapshot`, and the current `runWorkflow(...)` shape, but split the largest branching areas into ListingKit-owned phase seams. The slice stays feature-owned inside ListingKit and focuses on execution branching clarity, not workflow behavior redesign.

**Tech Stack:** Go, ListingKit workflow layer, existing workflow recorder/result model, existing `workflow_assets_test.go` and `workflow_studio_sds_metadata_test.go`, asset generation service, SDS sync helpers

**Out of Scope For This Slice:**

- redesigning `ProcessListingKit(...)` or worker retry behavior from `Phase 5A`
- changing submit/runtime context behavior from `Phase 4B`
- replacing `newWorkflowRecorder(...)` with a generic state machine
- changing asset generation or SDS sync business semantics
- moving workflow concerns into HTTP/bootstrap layers

---

## Root Cause This Slice Addresses

After `Phase 5A`, ListingKit still has one large execution hotspot left:

- [internal/listingkit/workflow_standard.go](/D:/code/task-processor/internal/listingkit/workflow_standard.go:1)
- [internal/listingkit/workflow_platform_adaptation.go](/D:/code/task-processor/internal/listingkit/workflow_platform_adaptation.go:1)

Today these files jointly decide:

1. how canonical product input is resolved across SDS baseline, studio fallback, cache, and product enrichment
2. how image processing and SDS sync are sequenced
3. how asset inventory and generation planning/dispatch are staged
4. how platform adaptation finalization and review decoration are applied

The real problem is not just file size. The problem is crossed ownership between:

- canonical acquisition branching
- media/SDS branching
- asset generation branching
- final platform adaptation/post-processing

That makes future changes risky because a behavior change in one workflow branch can easily leak across unrelated stages without one explicit seam to test.

There is already a mature local idea to reuse here:

- keep feature-owned bounded seams, like we did in `Phase 4B` and `Phase 5A`
- keep orchestration readable without introducing a repo-wide engine

`Phase 5B` should therefore split the largest workflow branches into phase-owned helpers, while preserving the existing recorder/result model and current behavior.

---

## Target Outcome

At the end of `Phase 5B`:

- canonical acquisition flows through an explicit phase seam
- media processing and SDS sync flow through an explicit phase seam
- asset planning/dispatch flows through an explicit phase seam
- platform adaptation finalization flows through an explicit seam
- current workflow behavior remains unchanged
- narrow boundary tests lock the new ownership split

---

## Task 1: Extract canonical acquisition phase from `workflow_standard.go`

**Files:**
- Create: `internal/listingkit/workflow_standard_canonical_phase.go`
- Modify: `internal/listingkit/workflow_standard.go`
- Modify: `internal/listingkit/workflow_studio_sds_metadata_test.go`

- [ ] **Step 1: Write failing tests for canonical acquisition precedence**

Extend `workflow_studio_sds_metadata_test.go` so it explicitly locks:

1. SDS baseline still wins before product enrich
2. studio catalog fallback still applies when SDS baseline is missing
3. task tenant still flows into baseline lookup when request tenant is empty
4. canonical cache reuse still bypasses live enrich when available

Prefer extending the existing `RunStandardProductWorkflow...` tests instead of creating a new harness.

- [ ] **Step 2: Run focused canonical-phase verification**

Run:

```powershell
go test ./internal/listingkit -run "TestRunStandardProductWorkflow(UsesSDSBaselineBeforeProductEnrich|UsesTaskTenantIDWhenRequestTenantMissing|FallsBackToStudioCanonicalWhenSDSBaselineMissing|ContinuesWhenSDSBaselineLookupErrors|IgnoresUnavailableOrMalformedSDSBaselineEntries)" -count=1
```

Expected: PASS before the refactor, establishing the behavior baseline.

- [ ] **Step 3: Add a canonical acquisition phase seam**

Create `workflow_standard_canonical_phase.go` with a focused helper such as:

- `type standardWorkflowCanonicalPhase struct { service *service }`
- `func buildStandardWorkflowCanonicalPhase(s *service) *standardWorkflowCanonicalPhase`
- `func (p *standardWorkflowCanonicalPhase) run(ctx context.Context, task *Task, result *ListingKitResult, recorder *workflowRecorder, log *logrus.Entry) (*canonical.Product, error)`

This seam should own:

- SDS baseline lookup
- studio catalog fallback
- canonical cache reuse
- product enrich fallback path
- canonical cache persistence after enrich

Important:

- reuse existing helper functions internally first
- do not change canonical precedence
- do not move image/SDS or asset-generation logic into this seam

- [ ] **Step 4: Rewire `runStandardProductWorkflow(...)` through the canonical phase seam**

Update `workflow_standard.go` so canonical product acquisition delegates to the new phase seam while the outer method remains the workflow-level entry point.

- [ ] **Step 5: Re-run focused canonical verification**

Run:

```powershell
go test ./internal/listingkit -run "TestRunStandardProductWorkflow(UsesSDSBaselineBeforeProductEnrich|UsesTaskTenantIDWhenRequestTenantMissing|FallsBackToStudioCanonicalWhenSDSBaselineMissing)" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/workflow_standard_canonical_phase.go internal/listingkit/workflow_standard.go internal/listingkit/workflow_studio_sds_metadata_test.go
git commit -m "refactor: extract listingkit canonical workflow phase"
```

---

## Task 2: Extract media and SDS sync phase from `workflow_standard.go`

**Files:**
- Create: `internal/listingkit/workflow_standard_media_phase.go`
- Modify: `internal/listingkit/workflow_standard.go`
- Modify: `internal/listingkit/workflow_assets_test.go`
- Modify: `internal/listingkit/workflow_studio_sds_metadata_test.go`

- [ ] **Step 1: Write failing tests for media/SDS branching**

Extend existing workflow tests so they explicitly lock:

1. image processing success still triggers local SDS sync
2. image-processing absence still allows remote SDS sync when configured
3. SDS metadata overlay still re-applies without dropping processed assets
4. image-processing failure still degrades the workflow instead of aborting the whole task

Use the existing stub services in `workflow_assets_test.go` instead of inventing a new fixture layer.

- [ ] **Step 2: Run focused media/SDS verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(RunWorkflowPersistsAssetInventoryAndBuildsPlatformBundles|RunStandardProductWorkflowReappliesSDSMetadataWithoutDroppingProcessedAssets|RunStandardProductWorkflowContinuesWhenSDSBaselineLookupErrors)" -count=1
```

Expected: PASS before the refactor.

- [ ] **Step 3: Add a media/SDS phase seam**

Create `workflow_standard_media_phase.go` with a focused helper such as:

- `type standardWorkflowMediaPhase struct { service *service }`
- `func buildStandardWorkflowMediaPhase(s *service) *standardWorkflowMediaPhase`
- `func (p *standardWorkflowMediaPhase) run(ctx context.Context, task *Task, result *ListingKitResult, canonicalProduct *canonical.Product, recorder *workflowRecorder, log *logrus.Entry) (*productimage.ImageProcessResult, *SDSSyncOptions)`

This seam should own:

- image task creation and processing
- local SDS sync on image results
- remote SDS sync fallback path
- SDS metadata re-application to canonical product

Important:

- reuse `syncSDSDesign(...)` and `syncSDSDesignFromRemote(...)` internally first
- keep current degradation behavior unchanged
- keep asset-generation logic out of this seam

- [ ] **Step 4: Rewire `runStandardProductWorkflow(...)` through the media/SDS phase seam**

Update `workflow_standard.go` so media and SDS branching delegate to the new seam.

- [ ] **Step 5: Re-run focused media/SDS verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(RunWorkflowPersistsAssetInventoryAndBuildsPlatformBundles|RunStandardProductWorkflowReappliesSDSMetadataWithoutDroppingProcessedAssets)" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/workflow_standard_media_phase.go internal/listingkit/workflow_standard.go internal/listingkit/workflow_assets_test.go internal/listingkit/workflow_studio_sds_metadata_test.go
git commit -m "refactor: extract listingkit media workflow phase"
```

---

## Task 3: Extract asset planning and platform adaptation finalization phases

**Files:**
- Create: `internal/listingkit/workflow_standard_asset_phase.go`
- Create: `internal/listingkit/workflow_platform_finalize_phase.go`
- Modify: `internal/listingkit/workflow_standard.go`
- Modify: `internal/listingkit/workflow_platform_adaptation.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write failing tests for asset/finalization behavior**

Extend `workflow_assets_test.go` so it explicitly locks:

1. inventory persistence still happens before platform bundle attachment
2. baseline asset generation and platform dispatch still persist generated assets/tasks
3. deferred platform asset dispatch in adaptation still decorates result generation
4. SHEIN review decoration and pricing/default-image post-processing still run during finalization

Prefer extending the existing workflow asset tests and adaptation-related assertions instead of adding a new end-to-end suite.

- [ ] **Step 2: Run focused asset/finalization verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(RunWorkflowPersistsAssetInventoryAndBuildsPlatformBundles|RunWorkflowDegradesWhenAssetGenerationDispatchFails|ApplySDSOfficialImagesToShein.*)" -count=1
```

Expected: PASS before the refactor.

- [ ] **Step 3: Add an asset planning phase seam**

Create `workflow_standard_asset_phase.go` with a focused helper such as:

- `type standardWorkflowAssetPhase struct { service *service }`
- `func buildStandardWorkflowAssetPhase(s *service) *standardWorkflowAssetPhase`
- `func (p *standardWorkflowAssetPhase) run(ctx context.Context, task *Task, result *ListingKitResult, canonicalProduct *canonical.Product, recorder *workflowRecorder) (*asset.Inventory, map[string][]assetrecipe.AssetRecipe, *assetgeneration.Result, []assetgeneration.Task, bool)`

This seam should own:

- inventory creation/persistence
- baseline asset generation
- platform asset plan/dispatch
- generation-task persistence for the standard phase

- [ ] **Step 4: Add a platform finalization seam**

Create `workflow_platform_finalize_phase.go` with a focused helper such as:

- `type platformFinalizePhase struct { service *service }`
- `func buildPlatformFinalizePhase(s *service) *platformFinalizePhase`
- `func (p *platformFinalizePhase) run(ctx context.Context, task *Task, final *ListingKitResult, inventory *asset.Inventory, recipesByPlatform map[string][]assetrecipe.AssetRecipe, generationPlan *assetgeneration.Result, persistedGenerationTasks []assetgeneration.Task, enableAssetGeneration bool, sdsOptions *SDSSyncOptions) *ListingKitResult`

This seam should own:

- SHEIN post-assembly optimization/defaults
- review decoration
- deferred platform asset dispatch
- generation-task persistence and summary finalization

Important:

- reuse existing helper functions internally first
- do not change result assembly order
- do not move assembler invocation itself out of `runPlatformAdaptation(...)` in this step

- [ ] **Step 5: Rewire `workflow_standard.go` and `workflow_platform_adaptation.go`**

Update both workflow files so they delegate to the new asset/finalization seams while keeping `runStandardProductWorkflow(...)` and `runPlatformAdaptation(...)` as public feature-owned entry points.

- [ ] **Step 6: Re-run focused asset/finalization verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(RunWorkflowPersistsAssetInventoryAndBuildsPlatformBundles|RunWorkflowDegradesWhenAssetGenerationDispatchFails)" -count=1
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/listingkit/workflow_standard_asset_phase.go internal/listingkit/workflow_platform_finalize_phase.go internal/listingkit/workflow_standard.go internal/listingkit/workflow_platform_adaptation.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: extract listingkit workflow asset and finalize phases"
```

---

## Task 4: Lock workflow execution phase ownership boundaries

**Files:**
- Create: `internal/listingkit/phase5b_workflow_boundary_test.go`
- Modify: `internal/listingkit/workflow_assets_test.go`
- Modify: `internal/listingkit/workflow_studio_sds_metadata_test.go`

- [ ] **Step 1: Add boundary guardrails**

Lock two things:

1. `workflow_standard.go` stops being the primary home of canonical/media/asset branch bodies
2. `workflow_platform_adaptation.go` stops being the primary home of finalization/deferred-dispatch bodies

Suggested checks:

- `workflow_standard.go` should delegate through explicit phase builders
- `workflow_platform_adaptation.go` should delegate through a finalization phase seam
- the dedicated phase files remain the ownership homes of those decisions

- [ ] **Step 2: Run full ListingKit verification**

Run:

```powershell
go test ./internal/listingkit -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/listingkit/phase5b_workflow_boundary_test.go internal/listingkit/workflow_assets_test.go internal/listingkit/workflow_studio_sds_metadata_test.go
git commit -m "test: lock listingkit workflow phase boundaries"
```

---

## Self-Review

### Spec coverage

This plan intentionally covers one bounded hotspot:

- canonical acquisition branching
- media/SDS branching
- asset planning branching
- platform adaptation finalization branching
- workflow boundary tests

It does not mix in process entry, worker retry behavior, submit/runtime context, or HTTP/bootstrap work.

### Reuse check

This slice explicitly reuses mature local seams already present in ListingKit:

- `newWorkflowRecorder(...)`
- `StandardProductSnapshot`
- existing workflow stub services and tests

It does not invent a generic workflow engine.

### Root-cause check

The problem being addressed is crossed ownership between:

- canonical acquisition
- media/SDS processing
- asset planning
- final platform adaptation

The plan therefore focuses on:

- extracting phase-owned execution seams
- keeping current recorder/result behavior
- reusing existing helper functions internally first
- locking boundaries with narrow source and behavior guardrails

### Scope discipline

This is a bounded slice:

- no workflow business redesign
- no process/worker retry changes
- no repo-wide workflow abstraction
- no return to submit/runtime context work

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-29-task-processor-framework-phase5b-workflow-execution-phases.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
