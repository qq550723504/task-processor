# Task Processor Framework Phase 6A ListingKit Platform Finalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split ListingKit platform finalization into smaller feature-owned seams so platform post-processing, deferred asset dispatch, and summary finalization stop remaining coupled inside one `run(...)` body.

**Architecture:** Keep the work fully feature-owned inside `internal/listingkit` and reuse the bounded-seam pattern that already worked in `Phase 4B`, `Phase 5A`, and `Phase 5B`. `workflow_platform_finalize_phase.go` should stay as the orchestration entry for finalization, but delegate the three real behavior groups to narrower helpers instead of remaining the primary home of all downstream logic.

**Tech Stack:** Go, ListingKit workflow layer, existing `workflowRecorder`, asset generation service, SHEIN publishing helpers, `workflow_assets_test.go`, source-boundary tests

---

## Out of Scope For This Slice

- introducing a generic execution-context model
- changing canonical/media/asset phase contracts
- redesigning asset generation or SHEIN review business semantics
- moving workflow concerns into HTTP/runtime/bootstrap layers
- replacing `workflow_platform_adaptation.go` as the public adaptation entry

## File Structure

### Existing hotspots

- [internal/listingkit/workflow_platform_finalize_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_finalize_phase.go:1)
  - current finalization seam that still mixes platform shaping, deferred asset dispatch, and summary/final logging
- [internal/listingkit/workflow_platform_adaptation.go](/D:/code/task-processor/internal/listingkit/workflow_platform_adaptation.go:1)
  - public adaptation entry that should keep delegating finalization rather than regrowing behavior
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)
  - current behavior harness for asset dispatch and end-to-end workflow output
- [internal/listingkit/phase5b_workflow_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase5b_workflow_boundary_test.go:1)
  - existing guardrail for workflow phase delegation

### Planned new files

- `internal/listingkit/workflow_platform_postprocess_phase.go`
  - owns SHEIN/platform post-processing after adaptation assembly
- `internal/listingkit/workflow_platform_asset_dispatch_phase.go`
  - owns deferred platform asset dispatch, generated-asset inventory merge, and generation-task persistence
- `internal/listingkit/workflow_platform_summary_phase.go`
  - owns summary merge/finalize, preview sync, and final finalization logging
- `internal/listingkit/phase6a_platform_finalize_boundary_test.go`
  - source-level guardrails that keep the new seam split stable

Each new file should have one clear responsibility. `workflow_platform_finalize_phase.go` should become orchestration glue, not another large ownership hotspot.

## Task 1: Extract platform post-processing from `workflow_platform_finalize_phase.go`

**Files:**
- Create: `internal/listingkit/workflow_platform_postprocess_phase.go`
- Modify: `internal/listingkit/workflow_platform_finalize_phase.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing tests for platform post-processing**

Extend `workflow_assets_test.go` with a focused test that locks the current post-processing semantics without introducing a new harness:

```go
func TestRunWorkflowAppliesSheinPlatformFinalizationDecorations(t *testing.T) {
\tt.Parallel()

\tproductSvc := &stubWorkflowProductService{
\t\ttask: &productenrich.Task{
\t\t\tID: "product-task-platform-finalize",
\t\t\tRequest: &productenrich.GenerateRequest{
\t\t\t\tImageURLs: []string{"https://example.com/source.jpg"},
\t\t\t\tText:      "gift box",
\t\t\t},
\t\t},
\t\tproduct: &productenrich.ProductJSON{
\t\t\tTitle:      "Gift Box",
\t\t\tCategory:   []string{"Home"},
\t\t\tImages:     []string{"https://example.com/source.jpg"},
\t\t\tAttributes: map[string]string{"brand": "DemoBrand"},
\t\t},
\t}
\tsvc := &service{
\t\tproductSvc:          productSvc,
\t\tassembler:           NewAssemblerWithConfig(AssemblerConfig{SheinBuilder: stubSheinPackageBuilder{}}),
\t\tassetRecipeResolver: newDefaultAssetRecipeResolver(),
\t\tassetBundleBuilder:  newDefaultAssetBundleBuilder(),
\t}
\ttask := &Task{
\t\tID: "listingkit-task-platform-finalize",
\t\tRequest: &GenerateRequest{
\t\t\tImageURLs: []string{"https://example.com/source.jpg"},
\t\t\tText:      "gift box",
\t\t\tPlatforms: []string{"shein"},
\t\t\tCountry:   "US",
\t\t\tLanguage:  "en_US",
\t\t\tOptions:   &GenerateOptions{ProcessImages: false},
\t\t},
\t}

\tresult, err := svc.runWorkflow(context.Background(), task)
\tif err != nil {
\t\tt.Fatalf("runWorkflow() error = %v", err)
\t}
\tif result.Shein == nil {
\t\tt.Fatal("expected shein package")
\t}
\tif result.Summary == nil || !result.Summary.NeedsReview {
\t\tt.Fatalf("summary = %+v, want review-aware finalized summary", result.Summary)
\t}
\tif !hasWorkflowStageStatus(result.WorkflowStages, "shein_review", WorkflowStageStatusCompleted) {
\t\tt.Fatalf("workflow stages = %+v, want completed shein_review", result.WorkflowStages)
\t}
}
```

This test is not meant to prove every helper in isolation. It is meant to lock the final workflow behavior before the refactor.

- [ ] **Step 2: Run focused post-processing verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(RunWorkflowAppliesSheinPlatformFinalizationDecorations|RunWorkflowPersistsAssetInventoryAndBuildsPlatformBundles)" -count=1
```

Expected: PASS before the refactor, establishing the current behavior baseline.

- [ ] **Step 3: Add a dedicated platform post-processing seam**

Create `internal/listingkit/workflow_platform_postprocess_phase.go` with a focused helper such as:

```go
type platformPostprocessPhase struct {
\tservice *service
}

func buildPlatformPostprocessPhase(s *service) *platformPostprocessPhase {
\treturn &platformPostprocessPhase{service: s}
}

func (p *platformPostprocessPhase) run(
\tctx context.Context,
\ttask *Task,
\tfinal *ListingKitResult,
\tsdsOptions *SDSSyncOptions,
) {
\tif final.Shein != nil {
\t\tif err := sheinpub.OptimizePackageReviewContent(ctx, final.Shein, p.service.sheinContentOptimizer); err != nil {
\t\t\tappendWarning(final, "shein content optimization skipped: "+err.Error())
\t\t}
\t}
\tp.service.applyDefaultSheinPricing(task.Request, final.Shein)
\tif shouldUseSDSOfficialImages(task.Request) {
\t\tapplySDSOfficialImagesToShein(final.Shein, task.Request, final.SDSDesignResult, sdsOptions)
\t\tapplySheinSizeReferenceImages(final.Shein, resolveSheinSizeReferenceImages(task.Request, final.SDSDesignResult))
\t}
\tif shouldUseSheinStudioAIImages(task.Request) {
\t\tapplySheinStudioAIImagesToShein(final.Shein, task.Request, final.SDSDesignResult)
\t}
\tapplySheinVariantImageCoverageGuard(final, task.Request, final.Shein)
}
```

Important:

- keep all current helper calls and ordering unchanged
- do not move deferred asset dispatch or summary finalization into this file
- keep the work feature-owned inside ListingKit

- [ ] **Step 4: Rewire `workflow_platform_finalize_phase.go` through the post-processing seam**

Update `workflow_platform_finalize_phase.go` so it delegates the platform shaping portion through:

```go
buildPlatformPostprocessPhase(p.service).run(ctx, task, final, sdsOptions)
```

and keeps only orchestration around summary setup, deferred asset dispatch, and finalization.

- [ ] **Step 5: Re-run focused post-processing verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(RunWorkflowAppliesSheinPlatformFinalizationDecorations|RunWorkflowPersistsAssetInventoryAndBuildsPlatformBundles)" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/workflow_platform_postprocess_phase.go internal/listingkit/workflow_platform_finalize_phase.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: extract listingkit platform postprocess phase"
```

## Task 2: Extract deferred asset dispatch from `workflow_platform_finalize_phase.go`

**Files:**
- Create: `internal/listingkit/workflow_platform_asset_dispatch_phase.go`
- Modify: `internal/listingkit/workflow_platform_finalize_phase.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing tests for deferred asset dispatch ownership**

Extend `workflow_assets_test.go` so it explicitly locks both the degraded path and the successful deferred-dispatch merge path:

```go
func TestRunWorkflowPersistsDeferredPlatformDispatchOutputs(t *testing.T) {
\tt.Parallel()

\tassetGenerator := &stubWorkflowAssetGenerator{
\t\tplanResult: &assetgeneration.Result{
\t\t\tTasks: []assetgeneration.Task{{
\t\t\t\tTaskID:          "asset-task-1",
\t\t\t\tID:              "asset-task-1",
\t\t\t\tPlatform:        "amazon",
\t\t\t\tExecutionStatus: "queued",
\t\t\t\tCanExecute:      true,
\t\t\t}},
\t\t},
\t\tdispatchResult: &assetgeneration.Result{
\t\t\tTasks: []assetgeneration.Task{{
\t\t\t\tTaskID:          "asset-task-1",
\t\t\t\tID:              "asset-task-1",
\t\t\t\tPlatform:        "amazon",
\t\t\t\tExecutionMode:   "deferred_stub",
\t\t\t\tExecutionStatus: "completed",
\t\t\t}},
\t\t\tAssets: []asset.Record{{
\t\t\t\tKind: asset.KindGalleryImage,
\t\t\t\tURL:  "https://cdn.example.com/generated-gallery.jpg",
\t\t\t}},
\t\t},
\t}

\tresult, repo := runWorkflowWithDeferredDispatchFixture(t, assetGenerator)

\tif result.AssetInventorySummary == nil || result.AssetInventorySummary.GeneratedRecords == 0 {
\t\tt.Fatalf("asset inventory summary = %+v, want generated records", result.AssetInventorySummary)
\t}
\ttasks, err := repo.ListGenerationTasks(context.Background(), "listingkit-task-deferred-success")
\tif err != nil {
\t\tt.Fatalf("ListGenerationTasks() error = %v", err)
\t}
\tif len(tasks) == 0 {
\t\tt.Fatal("expected persisted generation tasks")
\t}
}
```

Use a small local fixture helper in the test file if needed, but keep reusing existing stub services and repository setup.

- [ ] **Step 2: Run focused deferred-dispatch verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(RunWorkflowPersistsDeferredPlatformDispatchOutputs|RunWorkflowRecordsDeferredAssetGenerationDispatchFailure|RunWorkflowPersistsAssetInventoryAndBuildsPlatformBundles)" -count=1
```

Expected: PASS before the refactor.

- [ ] **Step 3: Add a dedicated deferred asset-dispatch seam**

Create `internal/listingkit/workflow_platform_asset_dispatch_phase.go` with a focused helper such as:

```go
type platformAssetDispatchPhase struct {
\tservice *service
}

func buildPlatformAssetDispatchPhase(s *service) *platformAssetDispatchPhase {
\treturn &platformAssetDispatchPhase{service: s}
}

func (p *platformAssetDispatchPhase) run(
\tctx context.Context,
\ttask *Task,
\tfinal *ListingKitResult,
\tinventory *asset.Inventory,
\trecipesByPlatform map[string][]assetrecipe.AssetRecipe,
\tgenerationPlan *assetgeneration.Result,
\tpersistedGenerationTasks []assetgeneration.Task,
\tenableAssetGeneration bool,
) []assetgeneration.Task {
\tif inventory == nil {
\t\treturn persistedGenerationTasks
\t}
\tif enableAssetGeneration {
\t\tattachPlatformImageBundles(final, inventory, recipesByPlatform, generationPlan, p.service.assetBundleBuilder)
\t}
\tpendingTasks := collectPlatformGenerationTasks(final)
\t// keep the current dispatch, inventory merge, generation decoration,
\t// and generation-task persistence semantics here
\treturn persistedGenerationTasks
}
```

Important:

- keep the existing dispatch ordering and persistence behavior unchanged
- keep `decorateListingKitResultGeneration(...)` in this seam for now, because it is part of generation completion behavior rather than final summary ownership
- do not pull summary finalization into this file

- [ ] **Step 4: Rewire `workflow_platform_finalize_phase.go` through the deferred-dispatch seam**

Update `workflow_platform_finalize_phase.go` so it delegates asset-dispatch behavior through:

```go
persistedGenerationTasks = buildPlatformAssetDispatchPhase(p.service).run(
\tctx,
\ttask,
\tfinal,
\tinventory,
\trecipesByPlatform,
\tgenerationPlan,
\tpersistedGenerationTasks,
\tenableAssetGeneration,
)
```

- [ ] **Step 5: Re-run focused deferred-dispatch verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(RunWorkflowPersistsDeferredPlatformDispatchOutputs|RunWorkflowRecordsDeferredAssetGenerationDispatchFailure|RunWorkflowPersistsAssetInventoryAndBuildsPlatformBundles)" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/workflow_platform_asset_dispatch_phase.go internal/listingkit/workflow_platform_finalize_phase.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: extract listingkit platform asset dispatch phase"
```

## Task 3: Extract summary finalization and preview sync from `workflow_platform_finalize_phase.go`

**Files:**
- Create: `internal/listingkit/workflow_platform_summary_phase.go`
- Modify: `internal/listingkit/workflow_platform_finalize_phase.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing tests for summary/finalization ownership**

Extend `workflow_assets_test.go` with a test that locks the summary merge/finalization result after platform finalization:

```go
func TestRunWorkflowFinalizesSummaryAfterPlatformDispatch(t *testing.T) {
\tt.Parallel()

\tresult, _ := runWorkflowWithDeferredDispatchFixture(t, &stubWorkflowAssetGenerator{
\t\tplanResult: &assetgeneration.Result{
\t\t\tTasks: []assetgeneration.Task{{
\t\t\t\tTaskID:          "asset-task-1",
\t\t\t\tID:              "asset-task-1",
\t\t\t\tPlatform:        "amazon",
\t\t\t\tExecutionStatus: "queued",
\t\t\t\tCanExecute:      true,
\t\t\t}},
\t\t},
\t})

\tif result.Summary == nil {
\t\tt.Fatal("expected summary")
\t}
\tif result.Summary.WarningCount < 0 || result.Summary.IssueCount < 0 {
\t\tt.Fatalf("summary = %+v, want finalized counts", result.Summary)
\t}
\tif result.StandardProductSnapshot == nil {
\t\tt.Fatalf("standard snapshot = %+v, want preserved snapshot", result.StandardProductSnapshot)
\t}
}
```

The purpose here is not to test every field individually. It is to lock that final summary shaping still happens after dispatch and after warning merges.

- [ ] **Step 2: Run focused summary/finalization verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(RunWorkflowFinalizesSummaryAfterPlatformDispatch|RunWorkflowRecordsDeferredAssetGenerationDispatchFailure|RunWorkflowAppliesSheinPlatformFinalizationDecorations)" -count=1
```

Expected: PASS before the refactor.

- [ ] **Step 3: Add a dedicated summary/finalization seam**

Create `internal/listingkit/workflow_platform_summary_phase.go` with a focused helper such as:

```go
type platformSummaryPhase struct{}

func buildPlatformSummaryPhase() *platformSummaryPhase {
\treturn &platformSummaryPhase{}
}

func (p *platformSummaryPhase) run(
\ttask *Task,
\tfinal *ListingKitResult,
\tsnapshot *StandardProductSnapshot,
) *ListingKitResult {
\tif final.Summary == nil {
\t\tfinal.Summary = &GenerationSummary{}
\t}
\tif snapshot != nil && snapshot.Summary != nil {
\t\tfinal.Summary.Warnings = uniqueStrings(append(final.Summary.Warnings, snapshot.Summary.Warnings...))
\t}
\tsheinReviewStage := newWorkflowRecorder(final).Start("shein_review", "")
\tapplySheinInspectionReviewToSummary(final)
\taddSheinReviewWorkflowIssues(final)
\tsheinReviewStage.Complete()
\tnewWorkflowRecorder(final).FinalizeSummary()
\tsyncAssetRenderPreviews(final)
\tlogrus.WithFields(logrus.Fields{
\t\t"component":     "listingkit/platform_adaptation_finalize",
\t\t"task_id":       task.ID,
\t\t"needs_review":  final.Summary != nil && final.Summary.NeedsReview,
\t\t"warning_count": processWarningCount(final),
\t}).Info("listing kit platform adaptation finalized")
\treturn final
}
```

Important:

- keep review-stage semantics unchanged
- keep final logging here because it belongs to finalization completion, not deferred dispatch
- do not move platform post-processing or asset dispatch into this file

- [ ] **Step 4: Rewire `workflow_platform_finalize_phase.go` through the summary seam**

Update `workflow_platform_finalize_phase.go` so the remaining orchestration becomes:

```go
buildPlatformPostprocessPhase(p.service).run(ctx, task, final, sdsOptions)
persistedGenerationTasks = buildPlatformAssetDispatchPhase(p.service).run(...)
return buildPlatformSummaryPhase().run(task, final, snapshot)
```

- [ ] **Step 5: Re-run focused summary/finalization verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(RunWorkflowFinalizesSummaryAfterPlatformDispatch|RunWorkflowRecordsDeferredAssetGenerationDispatchFailure|RunWorkflowAppliesSheinPlatformFinalizationDecorations)" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/workflow_platform_summary_phase.go internal/listingkit/workflow_platform_finalize_phase.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: extract listingkit platform summary phase"
```

## Task 4: Lock platform finalization ownership boundaries

**Files:**
- Create: `internal/listingkit/phase6a_platform_finalize_boundary_test.go`
- Modify: `internal/listingkit/phase5b_workflow_boundary_test.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Add boundary guardrails**

Create a new source-boundary test file that locks two things:

1. `workflow_platform_finalize_phase.go` becomes orchestration-only and delegates to the three new seams
2. `workflow_platform_adaptation.go` continues to delegate to `buildPlatformFinalizePhase(s).run(...)` rather than regrowing finalization logic

Suggested checks:

```go
func TestWorkflowPlatformFinalizePhaseFileDelegatesToSubSeams(t *testing.T) {
\tt.Parallel()

\tsrc, err := os.ReadFile("workflow_platform_finalize_phase.go")
\tif err != nil {
\t\tt.Fatalf("ReadFile(workflow_platform_finalize_phase.go) error = %v", err)
\t}
\tcontent := string(src)

\tfor _, needle := range []string{
\t\t"buildPlatformPostprocessPhase(p.service).run(",
\t\t"buildPlatformAssetDispatchPhase(p.service).run(",
\t\t"buildPlatformSummaryPhase().run(",
\t} {
\t\tif !strings.Contains(content, needle) {
\t\t\tt.Fatalf("workflow_platform_finalize_phase.go should contain %q", needle)
\t\t}
\t}

\tfor _, needle := range []string{
\t\t"sheinpub.OptimizePackageReviewContent(",
\t\t"applySDSOfficialImagesToShein(",
\t\t"applySheinInspectionReviewToSummary(",
\t\t"s.assetGenerator.Dispatch(",
\t\t"decorateListingKitResultGeneration(",
\t\t"syncAssetRenderPreviews(",
\t} {
\t\tif strings.Contains(content, needle) {
\t\t\tt.Fatalf("workflow_platform_finalize_phase.go should not contain %q", needle)
\t\t}
\t}
}
```

Also tighten `phase5b_workflow_boundary_test.go` so the older guardrail now points at the new finalization sub-seams instead of only the first-level seam.

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
git add internal/listingkit/phase6a_platform_finalize_boundary_test.go internal/listingkit/phase5b_workflow_boundary_test.go internal/listingkit/workflow_assets_test.go
git commit -m "test: lock listingkit platform finalization boundaries"
```

## Self-Review

### Spec coverage

This plan intentionally covers one bounded hotspot:

- platform post-processing
- deferred platform asset dispatch
- summary/finalization completion
- boundary guardrails for the new split

It does not mix in execution-context redesign, canonical/media/asset seam changes, or HTTP/runtime work.

### Reuse check

This slice explicitly reuses mature local patterns already present in ListingKit:

- keep feature-owned bounded seams
- keep `workflow_platform_adaptation.go` as the public adaptation entry
- keep `workflowRecorder` and existing test stubs

It does not invent a generic workflow or context framework.

### Root-cause check

The problem being addressed is mixed ownership inside one finalization seam across:

- platform-specific post-processing
- deferred asset-generation completion behavior
- summary and preview finalization

The plan therefore focuses on:

- extracting narrower feature-owned sub-seams
- preserving current finalization order and semantics
- locking the new boundaries with narrow source and behavior guardrails

### Scope discipline

This is a bounded slice:

- no generic execution-context model
- no business-semantics redesign
- no bootstrap/runtime work
- no return to canonical/media/asset seam work

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-29-task-processor-framework-phase6a-platform-finalization.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
