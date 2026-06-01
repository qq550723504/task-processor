# Task Processor Framework Phase 8B Bundle/Task Shaping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make ListingKit bundle/task-side shaping semantics more explicit so `workflow_platform_asset_dispatch_bundle_apply.go` no longer relies on one implicit trigger model for both platform bundle reshaping and returned-task merge.

**Architecture:** Keep the work fully feature-owned inside `internal/listingkit`. Split the current bundle-apply seam into a bundle-reshape seam and a task-merge seam, then route the existing bundle-apply seam through those helpers without touching inventory-side shaping or durability policy. Preserve current `assets-only`, `tasks-only`, and `assets+tasks` dispatch behavior.

**Tech Stack:** Go, ListingKit workflow layer, `assetgeneration`, `assetbundle`, existing `workflow_assets_test.go` regression harness, source-boundary tests

---

## Out of Scope For This Slice

- redesigning inventory-side shaping
- redesigning durability policy
- revisiting summary/review/finalization seams
- introducing a generic bundle or mutation framework
- moving workflow concerns into HTTP/runtime/bootstrap layers

## File Structure

### Existing hotspots

- [internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go:1)
  - current seam that still couples bundle reshaping and returned-task merge
- [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)
  - current orchestration seam that delegates to inventory-apply and bundle-apply
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)
  - behavior harness already covering `assets-only`, `tasks-only`, and `assets+tasks`
- [internal/listingkit/phase8a_asset_dispatch_apply_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase8a_asset_dispatch_apply_boundary_test.go:1)
  - current mutation-side boundary guardrail

### Planned new files

- `internal/listingkit/workflow_platform_asset_dispatch_bundle_reshape.go`
  - owns platform bundle reshaping trigger and `attachPlatformImageBundles(...)`
- `internal/listingkit/workflow_platform_asset_dispatch_task_merge.go`
  - owns returned-task merge semantics
- `internal/listingkit/phase8b_asset_dispatch_bundle_boundary_test.go`
  - locks the new bundle/task-side ownership split

### Files expected to shrink

- [internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go:1)
  - should become orchestration for bundle reshape and task merge handoff

Each file should have one clear responsibility. The design goal is not “more helper files,” but “bundle reshaping and task merge no longer depend on one implicit shared trigger model.”

## Task 1: Extract explicit bundle-reshape seam

**Files:**
- Create: `internal/listingkit/workflow_platform_asset_dispatch_bundle_reshape.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing seam-level tests**

Add direct seam-level tests near the current bundle-apply coverage in [workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1).

First add a reshape-on-assets-only test:

```go
func TestPlatformAssetDispatchBundleReshapePhaseRunReshapesBundlesWithoutReturnedTasks(t *testing.T) {
	t.Parallel()

	final := &ListingKitResult{
		Amazon: &AmazonPackage{},
		Shein:  &SheinPackage{},
	}
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindSourceImage, Origin: asset.OriginSource, URL: "https://example.com/source-1.jpg"},
		},
		Summary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	recipesByPlatform := resolveRecipesForPlatforms(newDefaultAssetRecipeResolver(), []string{"amazon", "shein"}, nil)

	buildPlatformAssetDispatchBundleReshapePhase(newDefaultAssetBundleBuilder()).run(
		final,
		inventory,
		recipesByPlatform,
		nil,
	)

	if final.Shein == nil || final.Shein.ImageBundle == nil {
		t.Fatalf("shein image bundle = %+v, want reshaped bundle even without returned tasks", final.Shein)
	}
}
```

Then add a tasks-only test:

```go
func TestPlatformAssetDispatchBundleReshapePhaseRunReshapesBundlesWithReturnedTasks(t *testing.T) {
	t.Parallel()

	final := &ListingKitResult{
		Amazon: &AmazonPackage{},
	}
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindSourceImage, Origin: asset.OriginSource, URL: "https://example.com/source-1.jpg"},
		},
		Summary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	recipesByPlatform := resolveRecipesForPlatforms(newDefaultAssetRecipeResolver(), []string{"amazon"}, nil)
	dispatchTasks := []assetgeneration.Task{{
		ID:              "amazon:hero",
		Platform:        "amazon",
		RecipeID:        "hero",
		ExecutionStatus: "completed",
	}}

	buildPlatformAssetDispatchBundleReshapePhase(newDefaultAssetBundleBuilder()).run(
		final,
		inventory,
		recipesByPlatform,
		dispatchTasks,
	)

	if final.Amazon == nil || final.Amazon.ImageBundle == nil {
		t.Fatalf("amazon image bundle = %+v, want reshaped bundle", final.Amazon)
	}
}
```

- [ ] **Step 2: Run focused seam verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchBundleReshapePhaseRun(ReshapesBundlesWithoutReturnedTasks|ReshapesBundlesWithReturnedTasks)$" -count=1
```

Expected: FAIL because `buildPlatformAssetDispatchBundleReshapePhase(...)` does not exist yet.

- [ ] **Step 3: Add the explicit bundle-reshape seam**

Create `internal/listingkit/workflow_platform_asset_dispatch_bundle_reshape.go`:

```go
package listingkit

import (
	"task-processor/internal/asset"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
)

type platformAssetDispatchBundleReshapePhase struct {
	bundleBuilder assetbundle.Builder
}

func buildPlatformAssetDispatchBundleReshapePhase(builder assetbundle.Builder) *platformAssetDispatchBundleReshapePhase {
	return &platformAssetDispatchBundleReshapePhase{bundleBuilder: builder}
}

func (p *platformAssetDispatchBundleReshapePhase) run(
	final *ListingKitResult,
	inventory *asset.Inventory,
	recipesByPlatform map[string][]assetrecipe.AssetRecipe,
	dispatchTasks []assetgeneration.Task,
) {
	if p == nil {
		return
	}
	attachPlatformImageBundles(final, inventory, recipesByPlatform, &assetgeneration.Result{Tasks: dispatchTasks}, p.bundleBuilder)
}
```

Keep this seam focused only on bundle reshaping trigger semantics. Do not move task merge into this file.

- [ ] **Step 4: Re-run focused seam verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchBundleReshapePhaseRun(ReshapesBundlesWithoutReturnedTasks|ReshapesBundlesWithReturnedTasks)$" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/workflow_platform_asset_dispatch_bundle_reshape.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: extract listingkit asset dispatch bundle reshape seam"
```

## Task 2: Extract explicit task-merge seam

**Files:**
- Create: `internal/listingkit/workflow_platform_asset_dispatch_task_merge.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing seam-level tests**

Add direct task-merge tests:

```go
func TestPlatformAssetDispatchTaskMergePhaseRunMergesReturnedTasks(t *testing.T) {
	t.Parallel()

	generationTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "planned"},
	}
	dispatchTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "completed"},
		{ID: "shein:gallery", Platform: "shein", RecipeID: "gallery", ExecutionStatus: "planned"},
	}

	got := buildPlatformAssetDispatchTaskMergePhase().run(generationTasks, dispatchTasks)

	want := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "completed"},
		{ID: "shein:gallery", Platform: "shein", RecipeID: "gallery", ExecutionStatus: "planned"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("generation tasks = %+v, want %+v", got, want)
	}
}
```

And a no-op test:

```go
func TestPlatformAssetDispatchTaskMergePhaseRunSkipsWhenNoReturnedTasks(t *testing.T) {
	t.Parallel()

	generationTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "planned", Metadata: map[string]string{"k": "v"}},
	}

	got := buildPlatformAssetDispatchTaskMergePhase().run(generationTasks, nil)

	if !reflect.DeepEqual(got, generationTasks) {
		t.Fatalf("generation tasks = %+v, want unchanged %+v", got, generationTasks)
	}
}
```

- [ ] **Step 2: Run focused seam verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchTaskMergePhaseRun(MergesReturnedTasks|SkipsWhenNoReturnedTasks)$" -count=1
```

Expected: FAIL because `buildPlatformAssetDispatchTaskMergePhase(...)` does not exist yet.

- [ ] **Step 3: Add the explicit task-merge seam**

Create `internal/listingkit/workflow_platform_asset_dispatch_task_merge.go`:

```go
package listingkit

import assetgeneration "task-processor/internal/asset/generation"

type platformAssetDispatchTaskMergePhase struct{}

func buildPlatformAssetDispatchTaskMergePhase() *platformAssetDispatchTaskMergePhase {
	return &platformAssetDispatchTaskMergePhase{}
}

func (p *platformAssetDispatchTaskMergePhase) run(
	generationTasks []assetgeneration.Task,
	dispatchTasks []assetgeneration.Task,
) []assetgeneration.Task {
	if p == nil || len(dispatchTasks) == 0 {
		return generationTasks
	}
	return mergeGenerationTasks(generationTasks, dispatchTasks)
}
```

Do not move bundle reshaping into this file.

- [ ] **Step 4: Re-run focused seam verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchTaskMergePhaseRun(MergesReturnedTasks|SkipsWhenNoReturnedTasks)$" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/workflow_platform_asset_dispatch_task_merge.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: extract listingkit asset dispatch task merge seam"
```

## Task 3: Route bundle-apply seam through the new trigger helpers

**Files:**
- Modify: `internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing orchestration-focused tests**

Add a source-shape test:

```go
func TestWorkflowPlatformAssetDispatchBundleApplyFileDelegatesToTriggerSubSeams(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("workflow_platform_asset_dispatch_bundle_apply.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_asset_dispatch_bundle_apply.go) error = %v", err)
	}
	source := string(content)

	for _, needle := range []string{
		"buildPlatformAssetDispatchBundleReshapePhase(p.bundleBuilder).run(",
		"buildPlatformAssetDispatchTaskMergePhase().run(",
	} {
		if !strings.Contains(source, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_bundle_apply.go should contain %q", needle)
		}
	}
}
```

Keep the current behavior tests for `assets-only`, `tasks-only`, and `assets+tasks` as the main regression harness.

- [ ] **Step 2: Run focused orchestration verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(WorkflowPlatformAssetDispatchBundleApplyFileDelegatesToTriggerSubSeams|PlatformAssetDispatchBundleApplyPhaseRun.*|ApplyPlatformAssetDispatchMutationShapesBundlesWhenDispatchReturns(TasksOnly|AssetsOnly))" -count=1
```

Expected: FAIL because `workflow_platform_asset_dispatch_bundle_apply.go` still contains one combined helper body.

- [ ] **Step 3: Rewire the bundle-apply seam**

Update [workflow_platform_asset_dispatch_bundle_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go:1):

```go
func (p *platformAssetDispatchBundleApplyPhase) run(
	final *ListingKitResult,
	inventory *asset.Inventory,
	recipesByPlatform map[string][]assetrecipe.AssetRecipe,
	generationTasks []assetgeneration.Task,
	dispatchTasks []assetgeneration.Task,
) []assetgeneration.Task {
	if p == nil {
		return generationTasks
	}

	buildPlatformAssetDispatchBundleReshapePhase(p.bundleBuilder).run(
		final,
		inventory,
		recipesByPlatform,
		dispatchTasks,
	)
	return buildPlatformAssetDispatchTaskMergePhase().run(generationTasks, dispatchTasks)
}
```

Keep the public helper name stable for this slice. The aim is to separate trigger semantics, not to change callers.

- [ ] **Step 4: Re-run focused orchestration verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(WorkflowPlatformAssetDispatchBundleApplyFileDelegatesToTriggerSubSeams|PlatformAssetDispatchBundleApplyPhaseRun.*|ApplyPlatformAssetDispatchMutationShapesBundlesWhenDispatchReturns(TasksOnly|AssetsOnly))" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: route listingkit asset dispatch bundle apply through trigger sub-seams"
```

## Task 4: Lock bundle/task-side ownership boundaries

**Files:**
- Create: `internal/listingkit/phase8b_asset_dispatch_bundle_boundary_test.go`
- Modify: `internal/listingkit/phase8a_asset_dispatch_apply_boundary_test.go`

- [ ] **Step 1: Write the boundary tests**

Create [phase8b_asset_dispatch_bundle_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase8b_asset_dispatch_bundle_boundary_test.go:1):

```go
package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestWorkflowPlatformAssetDispatchBundleReshapeFileOwnsBundleReshaping(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("workflow_platform_asset_dispatch_bundle_reshape.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_asset_dispatch_bundle_reshape.go) error = %v", err)
	}
	source := string(content)

	if !strings.Contains(source, "attachPlatformImageBundles(") {
		t.Fatalf("workflow_platform_asset_dispatch_bundle_reshape.go should contain %q", "attachPlatformImageBundles(")
	}
	for _, needle := range []string{
		"mergeGenerationTasks(",
		"inventory.Records = append(",
		"rebuildInventorySummary(",
		"SaveInventory(",
		"SaveGenerationTasks(",
	} {
		if strings.Contains(source, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_bundle_reshape.go should not contain %q", needle)
		}
	}
}

func TestWorkflowPlatformAssetDispatchTaskMergeFileOwnsReturnedTaskMerge(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("workflow_platform_asset_dispatch_task_merge.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_asset_dispatch_task_merge.go) error = %v", err)
	}
	source := string(content)

	if !strings.Contains(source, "mergeGenerationTasks(") {
		t.Fatalf("workflow_platform_asset_dispatch_task_merge.go should contain %q", "mergeGenerationTasks(")
	}
	for _, needle := range []string{
		"attachPlatformImageBundles(",
		"inventory.Records = append(",
		"rebuildInventorySummary(",
		"SaveInventory(",
		"SaveGenerationTasks(",
	} {
		if strings.Contains(source, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_task_merge.go should not contain %q", needle)
		}
	}
}
```

Then update [phase8a_asset_dispatch_apply_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase8a_asset_dispatch_apply_boundary_test.go:1) so:

- `workflow_platform_asset_dispatch_bundle_apply.go` must delegate to:
  - `buildPlatformAssetDispatchBundleReshapePhase(p.bundleBuilder).run(`
  - `buildPlatformAssetDispatchTaskMergePhase().run(`
- `workflow_platform_asset_dispatch_bundle_apply.go` must no longer inline:
  - `attachPlatformImageBundles(`
  - `mergeGenerationTasks(`

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestWorkflowPlatformAssetDispatch(BundleReshapeFileOwnsBundleReshaping|TaskMergeFileOwnsReturnedTaskMerge|ApplyFileDelegatesToMutationSubSeams|MutationSubSeamFilesOwnShapingResponsibilities|BundleApplyFileDelegatesToTriggerSubSeams)" -count=1
```

Expected: PASS

- [ ] **Step 3: Run full verification**

Run:

```powershell
go test ./internal/listingkit -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/listingkit/phase8a_asset_dispatch_apply_boundary_test.go internal/listingkit/phase8b_asset_dispatch_bundle_boundary_test.go
git commit -m "test: lock listingkit asset dispatch bundle trigger boundaries"
```

## Self-Review Checklist

Before executing, verify the plan still satisfies the scope:

- it only clarifies bundle/task-side trigger ownership, not inventory-side shaping
- it preserves current `assets-only`, `tasks-only`, and `assets+tasks` behavior
- it keeps durability and inventory-side seams untouched
- it keeps all new seams feature-local inside `internal/listingkit`
- it adds both behavior and source-boundary protection

## Expected Outcome

When `Phase 8B` is complete:

- [workflow_platform_asset_dispatch_bundle_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go:1) will no longer use one implicit trigger model for both bundle reshaping and task merge
- bundle reshaping and task merge will have separate local homes
- the `assets-only`, `tasks-only`, and `assets+tasks` dispatch-result paths will stay explicitly protected
- mutation-side shaping from `Phase 8A` will remain intact and clearer to evolve
