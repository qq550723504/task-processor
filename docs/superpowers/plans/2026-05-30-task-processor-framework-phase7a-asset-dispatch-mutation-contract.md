# Task Processor Framework Phase 7A Asset Dispatch Mutation Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make ListingKit deferred asset-dispatch side effects more explicit so `workflow_platform_asset_dispatch_phase.go` stops being the primary home of inventory mutation, bundle rebuild, generation-task merge, and persistence/decorate timing at the same time.

**Architecture:** Keep the work fully feature-owned inside `internal/listingkit`. Split the current dispatch seam into explicit local phases for mutation application and persistence/decorate timing, but do not introduce a generic workflow framework or a repo-wide mutation model. Preserve current deferred dispatch behavior and existing finalization orchestration.

**Tech Stack:** Go, ListingKit workflow layer, `assetgeneration`, in-memory inventory mutation, existing `workflow_assets_test.go` regression harness, source-boundary tests

---

## Out of Scope For This Slice

- redesigning review/summary semantics again
- changing asset-generation business policy
- changing platform post-process behavior
- introducing a generic workflow mutation container
- moving workflow concerns into HTTP/runtime/bootstrap layers

## File Structure

### Existing hotspots

- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)
  - current seam that mixes pre-dispatch attach, dispatch execution, inventory mutation, bundle rebuild, task merge, decorate, and persistence
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)
  - current behavior harness for deferred dispatch success/failure, inventory merge, and summary finalization
- [internal/listingkit/phase6a_platform_finalize_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase6a_platform_finalize_boundary_test.go:1)
  - current finalize-layer seam guardrail

### Planned new files

- `internal/listingkit/workflow_platform_asset_dispatch_apply.go`
  - owns dispatch-result mutation application to inventory/result/task state
- `internal/listingkit/workflow_platform_asset_dispatch_persist.go`
  - owns generation decoration and persistence timing after mutation is resolved
- `internal/listingkit/phase7a_asset_dispatch_boundary_test.go`
  - locks the new mutation/persist ownership split

### Files expected to shrink

- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)
  - should become orchestration for pre-attach, dispatch execution, mutation application, and persistence/decorate handoff

Each file should have one clear responsibility. The design goal is not “more files,” but “asset dispatch no longer hides multiple state transitions behind one `run(...)` body and one slice return value.”

## Task 1: Extract explicit dispatch-result mutation seam

**Files:**
- Create: `internal/listingkit/workflow_platform_asset_dispatch_apply.go`
- Modify: `internal/listingkit/workflow_platform_asset_dispatch_phase.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing seam-level tests**

Add behavior tests near the existing deferred dispatch coverage in [workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1).

First add a direct mutation test:

```go
func TestPlatformAssetDispatchApplyPhaseMergesDispatchArtifacts(t *testing.T) {
	t.Parallel()

	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: "listingkit-dispatch-apply-1"},
		Records: []asset.AssetRecord{{
			Kind: asset.KindSourceImage,
			URL:  "https://example.com/source.jpg",
		}},
		Summary: &asset.InventorySummary{TotalRecords: 1},
	}
	final := &ListingKitResult{
		AssetBundle: &asset.Bundle{},
		Amazon: &AmazonPackage{
			ImageBundle: &common.PublishImageBundle{
				Platform: "amazon",
				PendingGeneration: []common.BundleGenerationTask{{
					TaskID: "asset-task-1",
					Slot:   "gallery",
				}},
			},
		},
	}
	recipesByPlatform := map[string][]assetrecipe.AssetRecipe{"amazon": nil}
	persistedTasks := []assetgeneration.Task{{
		TaskID:          "asset-task-1",
		Platform:        "amazon",
		ExecutionStatus: "queued",
	}}
	dispatchResult := &assetgeneration.Result{
		Tasks: []assetgeneration.Task{{
			TaskID:          "asset-task-1",
			Platform:        "amazon",
			ExecutionMode:   "deferred_stub",
			ExecutionStatus: "completed",
		}},
		Assets: []asset.AssetRecord{{
			Kind: asset.KindGalleryImage,
			URL:  "https://cdn.example.com/generated-gallery.jpg",
		}},
	}

	mutated := buildPlatformAssetDispatchApplyPhase(&service{
		assetBundleBuilder: newDefaultAssetBundleBuilder(),
	}).run(final, inventory, recipesByPlatform, persistedTasks, dispatchResult)

	if mutated == nil {
		t.Fatal("mutated = nil")
	}
	if mutated.inventory == nil || !hasInventoryURL(mutated.inventory, "https://cdn.example.com/generated-gallery.jpg") {
		t.Fatalf("inventory = %+v, want merged generated asset", mutated.inventory)
	}
	if mutated.final == nil || mutated.final.AssetInventorySummary == nil || mutated.final.AssetInventorySummary.TotalRecords == 0 {
		t.Fatalf("final inventory summary = %+v, want rebuilt summary", mutated.final)
	}
	if len(mutated.generationTasks) != 1 || mutated.generationTasks[0].ExecutionStatus != "completed" {
		t.Fatalf("generation tasks = %+v, want merged dispatch status", mutated.generationTasks)
	}
}
```

Then add a nil-safe test:

```go
func TestPlatformAssetDispatchApplyPhaseHandlesNilDispatchResult(t *testing.T) {
	t.Parallel()

	persistedTasks := []assetgeneration.Task{{
		TaskID:          "asset-task-1",
		ExecutionStatus: "queued",
	}}

	mutated := buildPlatformAssetDispatchApplyPhase(&service{}).run(
		&ListingKitResult{},
		&asset.Inventory{},
		nil,
		persistedTasks,
		nil,
	)

	if mutated == nil {
		t.Fatal("mutated = nil")
	}
	if len(mutated.generationTasks) != 1 || mutated.generationTasks[0].ExecutionStatus != "queued" {
		t.Fatalf("generation tasks = %+v, want unchanged tasks", mutated.generationTasks)
	}
}
```

- [ ] **Step 2: Run focused mutation verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchApplyPhase(MergesDispatchArtifacts|HandlesNilDispatchResult)$" -count=1
```

Expected: FAIL because `buildPlatformAssetDispatchApplyPhase(...)` does not exist yet.

- [ ] **Step 3: Add the explicit mutation seam**

Create `internal/listingkit/workflow_platform_asset_dispatch_apply.go` with a small local result shape:

```go
type platformAssetDispatchMutation struct {
	final           *ListingKitResult
	inventory       *asset.Inventory
	generationTasks []assetgeneration.Task
}

type platformAssetDispatchApplyPhase struct {
	service *service
}

func buildPlatformAssetDispatchApplyPhase(s *service) *platformAssetDispatchApplyPhase {
	return &platformAssetDispatchApplyPhase{service: s}
}

func (p *platformAssetDispatchApplyPhase) run(
	final *ListingKitResult,
	inventory *asset.Inventory,
	recipesByPlatform map[string][]assetrecipe.AssetRecipe,
	persistedGenerationTasks []assetgeneration.Task,
	dispatchResult *assetgeneration.Result,
) *platformAssetDispatchMutation {
	mutated := &platformAssetDispatchMutation{
		final:           final,
		inventory:       inventory,
		generationTasks: persistedGenerationTasks,
	}
	if dispatchResult == nil {
		return mutated
	}
	if len(dispatchResult.Assets) > 0 && mutated.inventory != nil {
		mutated.inventory.Records = append(mutated.inventory.Records, dispatchResult.Assets...)
		mutated.inventory.Summary = rebuildInventorySummary(mutated.inventory)
		mutated.final.AssetBundle = rebuildBundleWithGeneratedAssets(mutated.final.AssetBundle, dispatchResult.Assets)
		mutated.final.AssetInventorySummary = mutated.inventory.Summary
	}
	if len(dispatchResult.Tasks) > 0 {
		attachPlatformImageBundles(
			mutated.final,
			mutated.inventory,
			recipesByPlatform,
			&assetgeneration.Result{Tasks: dispatchResult.Tasks},
			p.service.assetBundleBuilder,
		)
		mutated.generationTasks = mergeGenerationTasks(mutated.generationTasks, dispatchResult.Tasks)
	}
	return mutated
}
```

Keep it feature-local and mutation-focused. Do not move persistence into this file.

- [ ] **Step 4: Rewire the dispatch seam through the mutation result**

Update [workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1) so:

- dispatch execution stays in the phase entry
- dispatch-result mutation application delegates to `buildPlatformAssetDispatchApplyPhase(...).run(...)`
- the parent seam consumes `mutated.final`, `mutated.inventory`, and `mutated.generationTasks`

Do not yet move decorate/persist timing out of the parent seam in this task.

- [ ] **Step 5: Re-run focused mutation verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchApplyPhase(MergesDispatchArtifacts|HandlesNilDispatchResult)$" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/workflow_platform_asset_dispatch_apply.go internal/listingkit/workflow_platform_asset_dispatch_phase.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: extract listingkit asset dispatch mutation seam"
```

## Task 2: Extract decorate/persist timing seam

**Files:**
- Create: `internal/listingkit/workflow_platform_asset_dispatch_persist.go`
- Modify: `internal/listingkit/workflow_platform_asset_dispatch_phase.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing persistence/decoration tests**

Add a direct seam-level test:

```go
func TestPlatformAssetDispatchPersistPhaseDecoratesAndPersistsGenerationTasks(t *testing.T) {
	t.Parallel()

	repo := assetrepo.NewMemRepository()
	final := &ListingKitResult{}
	task := &Task{ID: "listingkit-dispatch-persist-1"}
	generationTasks := []assetgeneration.Task{{
		TaskID:          "asset-task-1",
		Platform:        "amazon",
		ExecutionMode:   "deferred_stub",
		ExecutionStatus: "completed",
	}}

	buildPlatformAssetDispatchPersistPhase(&service{assetRepo: repo}).run(context.Background(), task, final, generationTasks)

	if len(final.ChildTasks) == 0 {
		t.Fatalf("child tasks = %+v, want decorated generation state", final.ChildTasks)
	}
	tasks, err := repo.ListGenerationTasks(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("ListGenerationTasks() error = %v", err)
	}
	if len(tasks) != 1 || tasks[0].ExecutionStatus != "completed" {
		t.Fatalf("persisted tasks = %+v, want saved generation task", tasks)
	}
}
```

And a persistence-failure test:

```go
func TestPlatformAssetDispatchPersistPhaseWarnsWhenTaskPersistenceFails(t *testing.T) {
	t.Parallel()

	repo := &stubWorkflowAssetRepository{
		saveGenerationTasksErr: fmt.Errorf("db unavailable"),
	}
	final := &ListingKitResult{}
	task := &Task{ID: "listingkit-dispatch-persist-fail-1"}
	generationTasks := []assetgeneration.Task{{
		TaskID:          "asset-task-1",
		Platform:        "amazon",
		ExecutionStatus: "completed",
	}}

	buildPlatformAssetDispatchPersistPhase(&service{assetRepo: repo}).run(context.Background(), task, final, generationTasks)

	if !hasWorkflowIssue(final.WorkflowIssues, "asset_generation_platform", WorkflowIssueSeverityWarning, "asset_generation_task_persistence_failed") {
		t.Fatalf("workflow issues = %+v, want persistence warning", final.WorkflowIssues)
	}
}
```

- [ ] **Step 2: Run focused persist verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchPersistPhase(DecoratesAndPersistsGenerationTasks|WarnsWhenTaskPersistenceFails)$" -count=1
```

Expected: FAIL because `buildPlatformAssetDispatchPersistPhase(...)` does not exist yet.

- [ ] **Step 3: Add the explicit persist seam**

Create `internal/listingkit/workflow_platform_asset_dispatch_persist.go`:

```go
type platformAssetDispatchPersistPhase struct {
	service *service
}

func buildPlatformAssetDispatchPersistPhase(s *service) *platformAssetDispatchPersistPhase {
	return &platformAssetDispatchPersistPhase{service: s}
}

func (p *platformAssetDispatchPersistPhase) run(
	ctx context.Context,
	task *Task,
	final *ListingKitResult,
	generationTasks []assetgeneration.Task,
) {
	decorateListingKitResultGeneration(final, generationTasks)
	if p.service.assetRepo == nil || len(generationTasks) == 0 {
		return
	}
	if err := p.service.assetRepo.SaveGenerationTasks(ctx, task.ID, generationTasks); err != nil {
		appendWarning(final, "asset generation task persistence failed: "+err.Error())
		newWorkflowRecorder(final).AddIssue(
			WorkflowIssueSeverityWarning,
			"asset_generation_platform",
			"asset_generation_task_persistence_failed",
			"Asset generation task persistence failed",
			err.Error(),
		)
	}
}
```

Do not move dispatch execution into this file.

- [ ] **Step 4: Rewire the parent dispatch seam**

Update [workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1) so it now reads more like:

1. pre-attach bundles
2. collect pending tasks
3. execute dispatch
4. apply mutation result
5. persist/decorate through the new seam

Keep existing behavior unchanged.

- [ ] **Step 5: Re-run focused persist verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchPersistPhase(DecoratesAndPersistsGenerationTasks|WarnsWhenTaskPersistenceFails)$" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/workflow_platform_asset_dispatch_persist.go internal/listingkit/workflow_platform_asset_dispatch_phase.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: extract listingkit asset dispatch persistence seam"
```

## Task 3: Thin the parent dispatch seam to orchestration

**Files:**
- Modify: `internal/listingkit/workflow_platform_asset_dispatch_phase.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing orchestration-focused test**

Add a behavior-level seam test:

```go
func TestPlatformAssetDispatchPhaseOrchestratesMutationAndPersistence(t *testing.T) {
	t.Parallel()

	repo := assetrepo.NewMemRepository()
	assetGenerator := &stubWorkflowAssetGenerator{
		dispatchResult: &assetgeneration.Result{
			Tasks: []assetgeneration.Task{{
				TaskID:          "asset-task-1",
				Platform:        "amazon",
				ExecutionMode:   "deferred_stub",
				ExecutionStatus: "completed",
			}},
			Assets: []asset.AssetRecord{{
				Kind: asset.KindGalleryImage,
				URL:  "https://cdn.example.com/generated-gallery.jpg",
			}},
		},
	}
	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: "listingkit-dispatch-orchestrate-1"},
		Records: []asset.AssetRecord{{
			Kind: asset.KindSourceImage,
			URL:  "https://example.com/source.jpg",
		}},
		Summary: &asset.InventorySummary{TotalRecords: 1},
	}
	final := &ListingKitResult{
		CatalogProduct: &productenrich.ProductJSON{Title: "Poster"},
		AssetBundle:    &asset.Bundle{},
		Amazon: &AmazonPackage{
			ImageBundle: &common.PublishImageBundle{
				Platform: "amazon",
				PendingGeneration: []common.BundleGenerationTask{{
					TaskID: "asset-task-1",
				}},
			},
		},
	}

	tasks := buildPlatformAssetDispatchPhase(&service{
		assetRepo:           repo,
		assetGenerator:      assetGenerator,
		assetBundleBuilder:  newDefaultAssetBundleBuilder(),
	}).run(
		context.Background(),
		&Task{ID: "listingkit-dispatch-orchestrate-1"},
		final,
		inventory,
		map[string][]assetrecipe.AssetRecipe{"amazon": nil},
		&assetgeneration.Result{},
		nil,
		true,
	)

	if len(tasks) != 1 || tasks[0].ExecutionStatus != "completed" {
		t.Fatalf("tasks = %+v, want merged dispatch tasks", tasks)
	}
	if !hasInventoryURL(inventory, "https://cdn.example.com/generated-gallery.jpg") {
		t.Fatalf("inventory = %+v, want merged generated asset", inventory)
	}
}
```

- [ ] **Step 2: Run focused orchestration verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchPhaseOrchestratesMutationAndPersistence$" -count=1
```

Expected: PASS or FAIL depending on intermediate shape. If already PASS, keep it as an explicit guard before the next cleanup.

- [ ] **Step 3: Simplify `workflow_platform_asset_dispatch_phase.go`**

Refactor the parent seam so its main body mostly reads as orchestration:

```go
func (p *platformAssetDispatchPhase) run(...) []assetgeneration.Task {
	if inventory == nil {
		return persistedGenerationTasks
	}
	if enableAssetGeneration {
		attachPlatformImageBundles(...)
	}
	pendingTasks := collectPlatformGenerationTasks(final)
	dispatchResult, dispatchErr := p.dispatch(...)
	mutated := buildPlatformAssetDispatchApplyPhase(p.service).run(...)
	buildPlatformAssetDispatchPersistPhase(p.service).run(...)
	return mutated.generationTasks
}
```

It is okay if a tiny local helper remains for dispatch-stage lifecycle bookkeeping, but the parent seam should stop being the primary home of mutation/persist details.

- [ ] **Step 4: Re-run orchestration and regression verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchPhaseOrchestratesMutationAndPersistence|TestRunWorkflowRecordsDeferredAssetGenerationDispatchFailure|TestRunWorkflowPersistsDeferredPlatformDispatchOutputs|TestRunWorkflowFinalizesSummaryAfterPlatformDispatch|TestRunWorkflowSkipsDeferredGenerationWhenProcessImagesDisabled" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/workflow_platform_asset_dispatch_phase.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: thin listingkit asset dispatch orchestration"
```

## Task 4: Lock asset-dispatch ownership boundaries

**Files:**
- Create: `internal/listingkit/phase7a_asset_dispatch_boundary_test.go`
- Modify: `internal/listingkit/phase6a_platform_finalize_boundary_test.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Add source-boundary guardrails**

Create `phase7a_asset_dispatch_boundary_test.go` to lock three things:

1. `workflow_platform_asset_dispatch_phase.go` remains orchestration, not the home of direct inventory mutation or generation-task persistence details
2. `workflow_platform_asset_dispatch_apply.go` owns mutation application concerns
3. `workflow_platform_asset_dispatch_persist.go` owns decorate/persist concerns

Suggested checks:

```go
func TestWorkflowPlatformAssetDispatchPhaseDelegatesToMutationAndPersistSeams(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("workflow_platform_asset_dispatch_phase.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_asset_dispatch_phase.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"buildPlatformAssetDispatchApplyPhase(p.service).run(",
		"buildPlatformAssetDispatchPersistPhase(p.service).run(",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_phase.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"inventory.Records = append(",
		"p.service.assetRepo.SaveGenerationTasks(",
		"decorateListingKitResultGeneration(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_phase.go should not contain %q", needle)
		}
	}
}
```

And one seam ownership test:

```go
func TestWorkflowPlatformAssetDispatchPersistPhaseOwnsGenerationTaskPersistence(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("workflow_platform_asset_dispatch_persist.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_asset_dispatch_persist.go) error = %v", err)
	}
	content := string(src)

	if !strings.Contains(content, "SaveGenerationTasks(") {
		t.Fatalf("workflow_platform_asset_dispatch_persist.go should persist generation tasks")
	}
}
```

Only update [phase6a_platform_finalize_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase6a_platform_finalize_boundary_test.go:1) if the finalize seam call shape needs a minimal alignment.

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
git add internal/listingkit/phase7a_asset_dispatch_boundary_test.go internal/listingkit/phase6a_platform_finalize_boundary_test.go internal/listingkit/workflow_assets_test.go
git commit -m "test: lock listingkit asset dispatch boundaries"
```

## Self-Review

### Spec coverage

This plan intentionally covers one bounded hotspot:

- deferred dispatch mutation ownership
- explicit mutation result handoff
- decorate/persist timing ownership
- source and behavior guardrails for the new split

It does not mix in summary/review redesign, platform post-process sub-splitting, or generic workflow abstractions.

### Reuse check

This slice explicitly reuses mature local ListingKit patterns:

- feature-owned bounded seams
- behavior-first workflow regression tests
- source-boundary tests that prevent seam regrowth

It does not invent a repo-wide mutation framework.

### Root-cause check

The problem being addressed is not just that `workflow_platform_asset_dispatch_phase.go` is “too busy.” The real problem is that:

- one seam still mixes in-memory mutation, persistence timing, and post-dispatch decoration across multiple state objects
- the current `run(...)` return value only exposes one mutated slice, while the real side effects are broader

The plan therefore focuses on:

- making mutation ownership explicit
- separating persistence/decorate timing
- preserving existing behavior
- locking the new ownership split with narrow tests

### Scope discipline

This is a bounded slice:

- no summary/review redesign
- no generic workflow context
- no post-process refactor unless dispatch work forces it
- no business-policy changes

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-30-task-processor-framework-phase7a-asset-dispatch-mutation-contract.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
