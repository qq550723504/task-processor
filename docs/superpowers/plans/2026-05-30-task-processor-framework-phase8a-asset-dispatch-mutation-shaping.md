# Task Processor Framework Phase 8A Asset-Dispatch Mutation Shaping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make ListingKit deferred asset-dispatch mutation shaping more explicit so `workflow_platform_asset_dispatch_apply.go` stops being the primary shared home of inventory mutation, bundle rebuild, and returned-task merge at the same time.

**Architecture:** Keep the work fully feature-owned inside `internal/listingkit`. Split the current apply seam into narrower local phases for inventory mutation and bundle/task shaping, then route the existing apply seam through those phases without redesigning durability or generation-task persistence. Preserve current deferred dispatch behavior and existing orchestration order.

**Tech Stack:** Go, ListingKit workflow layer, `assetgeneration`, `assetbundle`, existing `workflow_assets_test.go` regression harness, source-boundary tests

---

## Out of Scope For This Slice

- redesigning inventory durability policy
- changing deferred asset-generation business behavior
- revisiting summary/review/finalization seams
- introducing a generic mutation framework
- moving workflow concerns into HTTP/runtime/bootstrap layers

## File Structure

### Existing hotspots

- [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)
  - current seam that still mixes inventory mutation, bundle rebuild, image-bundle reattach, and generation-task merge
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)
  - current behavior harness for asset-dispatch mutation semantics
- [internal/listingkit/phase7a_asset_dispatch_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase7a_asset_dispatch_boundary_test.go:1)
  - current dispatch-seam boundary guardrail
- [internal/listingkit/phase7b_asset_dispatch_persist_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase7b_asset_dispatch_persist_boundary_test.go:1)
  - current inventory-durability boundary guardrail

### Planned new files

- `internal/listingkit/workflow_platform_asset_dispatch_inventory_apply.go`
  - owns returned-asset inventory mutation and inventory-summary refresh
- `internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go`
  - owns asset-bundle rebuild, platform image-bundle reattach, and returned-task merge
- `internal/listingkit/phase8a_asset_dispatch_apply_boundary_test.go`
  - locks the new mutation-side ownership split

### Files expected to shrink

- [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)
  - should become orchestration for inventory apply and bundle/task apply

Each file should have one clear responsibility. The design goal is not “more helper files,” but “the apply seam no longer hides multiple mutation categories inside one undifferentiated helper body.”

## Task 1: Extract explicit inventory-apply seam

**Files:**
- Create: `internal/listingkit/workflow_platform_asset_dispatch_inventory_apply.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing seam-level tests**

Add direct seam-level tests near the existing mutation coverage in [workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1).

First add a positive inventory-mutation test:

```go
func TestPlatformAssetDispatchInventoryApplyPhaseRunMergesReturnedAssets(t *testing.T) {
	t.Parallel()

	final := &ListingKitResult{
		AssetBundle:           &asset.Bundle{},
		AssetInventorySummary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindSourceImage, Origin: asset.OriginSource, URL: "https://example.com/source-1.jpg"},
		},
		Summary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	dispatchAssets := []asset.AssetRecord{
		{
			ID:       "generated-1",
			Kind:     asset.KindSceneImage,
			Origin:   asset.OriginGenerated,
			URL:      "https://cdn.example.com/generated-1.jpg",
			RecipeID: "scene",
		},
	}

	mutated := buildPlatformAssetDispatchInventoryApplyPhase().run(final, inventory, dispatchAssets)

	if mutated == nil || mutated.inventory == nil {
		t.Fatalf("mutated = %+v, want non-nil mutation", mutated)
	}
	if got := len(mutated.inventory.Records); got != 2 {
		t.Fatalf("inventory records = %d, want 2", got)
	}
	if mutated.final.AssetInventorySummary == nil || mutated.final.AssetInventorySummary.GeneratedRecords != 1 {
		t.Fatalf("asset inventory summary = %+v, want generated record count updated", mutated.final.AssetInventorySummary)
	}
	if mutated.final.AssetBundle == nil || len(mutated.final.AssetBundle.Assets) != 1 {
		t.Fatalf("asset bundle = %+v, want generated asset merged", mutated.final.AssetBundle)
	}
}
```

Then add a no-op test:

```go
func TestPlatformAssetDispatchInventoryApplyPhaseRunSkipsWhenNoReturnedAssets(t *testing.T) {
	t.Parallel()

	final := &ListingKitResult{
		AssetBundle:           &asset.Bundle{},
		AssetInventorySummary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindSourceImage, Origin: asset.OriginSource, URL: "https://example.com/source-1.jpg"},
		},
		Summary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}

	mutated := buildPlatformAssetDispatchInventoryApplyPhase().run(final, inventory, nil)

	if got := len(mutated.inventory.Records); got != 1 {
		t.Fatalf("inventory records = %d, want unchanged count 1", got)
	}
	if mutated.final.AssetInventorySummary == nil || mutated.final.AssetInventorySummary.GeneratedRecords != 0 {
		t.Fatalf("asset inventory summary = %+v, want unchanged summary", mutated.final.AssetInventorySummary)
	}
}
```

- [ ] **Step 2: Run focused seam verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchInventoryApplyPhaseRun(MergesReturnedAssets|SkipsWhenNoReturnedAssets)$" -count=1
```

Expected: FAIL because `buildPlatformAssetDispatchInventoryApplyPhase(...)` does not exist yet.

- [ ] **Step 3: Add the explicit inventory-apply seam**

Create `internal/listingkit/workflow_platform_asset_dispatch_inventory_apply.go`:

```go
package listingkit

import "task-processor/internal/asset"

type platformAssetDispatchInventoryApplyPhase struct{}

func buildPlatformAssetDispatchInventoryApplyPhase() *platformAssetDispatchInventoryApplyPhase {
	return &platformAssetDispatchInventoryApplyPhase{}
}

func (p *platformAssetDispatchInventoryApplyPhase) run(
	final *ListingKitResult,
	inventory *asset.Inventory,
	dispatchAssets []asset.AssetRecord,
) platformAssetDispatchMutation {
	mutation := platformAssetDispatchMutation{
		final:     final,
		inventory: inventory,
	}
	if len(dispatchAssets) == 0 {
		return mutation
	}
	inventory.Records = append(inventory.Records, dispatchAssets...)
	inventory.Summary = rebuildInventorySummary(inventory)
	final.AssetBundle = rebuildBundleWithGeneratedAssets(final.AssetBundle, dispatchAssets)
	final.AssetInventorySummary = inventory.Summary
	return mutation
}
```

Do not move bundle/task shaping into this file. In this slice, “inventory apply” owns returned-asset inventory/bundle mutation only.

- [ ] **Step 4: Re-run focused seam verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchInventoryApplyPhaseRun(MergesReturnedAssets|SkipsWhenNoReturnedAssets)$" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/workflow_platform_asset_dispatch_inventory_apply.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: extract listingkit asset dispatch inventory apply seam"
```

## Task 2: Extract explicit bundle/task-apply seam

**Files:**
- Create: `internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing seam-level tests**

Add direct seam-level tests for returned-task shaping:

```go
func TestPlatformAssetDispatchBundleApplyPhaseRunReattachesBundlesAndMergesTasks(t *testing.T) {
	t.Parallel()

	final := &ListingKitResult{
		Amazon: &AmazonPackage{
			ImageBundle: &common.PublishImageBundle{
				Platform: "amazon",
			},
		},
	}
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindSourceImage, Origin: asset.OriginSource, URL: "https://example.com/source-1.jpg"},
		},
		Summary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	generationTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "planned"},
	}
	dispatchTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "completed"},
		{ID: "amazon:gallery", Platform: "amazon", RecipeID: "gallery", ExecutionStatus: "planned"},
	}

	mutated := buildPlatformAssetDispatchBundleApplyPhase(newDefaultAssetBundleBuilder()).run(
		final,
		inventory,
		map[string][]assetrecipe.AssetRecipe{"amazon": nil},
		generationTasks,
		dispatchTasks,
	)

	if len(mutated.generationTasks) != 2 {
		t.Fatalf("generation tasks = %+v, want merged tasks", mutated.generationTasks)
	}
	if mutated.generationTasks[0].ExecutionStatus != "completed" {
		t.Fatalf("generation tasks = %+v, want merged execution status", mutated.generationTasks)
	}
}
```

And a no-op test:

```go
func TestPlatformAssetDispatchBundleApplyPhaseRunSkipsWhenNoReturnedTasks(t *testing.T) {
	t.Parallel()

	generationTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "planned"},
	}

	mutated := buildPlatformAssetDispatchBundleApplyPhase(newDefaultAssetBundleBuilder()).run(
		&ListingKitResult{},
		&asset.Inventory{},
		nil,
		generationTasks,
		nil,
	)

	if !reflect.DeepEqual(mutated.generationTasks, generationTasks) {
		t.Fatalf("generation tasks = %+v, want unchanged %+v", mutated.generationTasks, generationTasks)
	}
}
```

- [ ] **Step 2: Run focused seam verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchBundleApplyPhaseRun(ReattachesBundlesAndMergesTasks|SkipsWhenNoReturnedTasks)$" -count=1
```

Expected: FAIL because `buildPlatformAssetDispatchBundleApplyPhase(...)` does not exist yet.

- [ ] **Step 3: Add the explicit bundle/task-apply seam**

Create `internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go`:

```go
package listingkit

import (
	"task-processor/internal/asset"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
)

type platformAssetDispatchBundleApplyPhase struct {
	bundleBuilder assetbundle.Builder
}

func buildPlatformAssetDispatchBundleApplyPhase(bundleBuilder assetbundle.Builder) *platformAssetDispatchBundleApplyPhase {
	return &platformAssetDispatchBundleApplyPhase{bundleBuilder: bundleBuilder}
}

func (p *platformAssetDispatchBundleApplyPhase) run(
	final *ListingKitResult,
	inventory *asset.Inventory,
	recipesByPlatform map[string][]assetrecipe.AssetRecipe,
	generationTasks []assetgeneration.Task,
	dispatchTasks []assetgeneration.Task,
) platformAssetDispatchMutation {
	mutation := platformAssetDispatchMutation{
		final:           final,
		inventory:       inventory,
		generationTasks: generationTasks,
	}
	if len(dispatchTasks) == 0 {
		return mutation
	}
	attachPlatformImageBundles(final, inventory, recipesByPlatform, &assetgeneration.Result{Tasks: dispatchTasks}, p.bundleBuilder)
	mutation.generationTasks = mergeGenerationTasks(generationTasks, dispatchTasks)
	return mutation
}
```

Do not move inventory mutation into this file.

- [ ] **Step 4: Re-run focused seam verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchBundleApplyPhaseRun(ReattachesBundlesAndMergesTasks|SkipsWhenNoReturnedTasks)$" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/workflow_platform_asset_dispatch_bundle_apply.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: extract listingkit asset dispatch bundle apply seam"
```

## Task 3: Route the apply seam through the new local phases

**Files:**
- Modify: `internal/listingkit/workflow_platform_asset_dispatch_apply.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing orchestration-focused tests**

Add a source-shape test:

```go
func TestWorkflowPlatformAssetDispatchApplyFileDelegatesToMutationSubSeams(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("workflow_platform_asset_dispatch_apply.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_asset_dispatch_apply.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"buildPlatformAssetDispatchInventoryApplyPhase().run(",
		"buildPlatformAssetDispatchBundleApplyPhase(bundleBuilder).run(",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_apply.go should contain %q", needle)
		}
	}
}
```

Keep the existing behavior tests like `TestApplyPlatformAssetDispatchMutationMergesDispatchArtifacts` and `TestApplyPlatformAssetDispatchMutationKeepsGenerationTasksWhenDispatchResultNil` as the main regression proof.

- [ ] **Step 2: Run focused orchestration verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(ApplyPlatformAssetDispatchMutation.*|WorkflowPlatformAssetDispatchApplyFileDelegatesToMutationSubSeams)" -count=1
```

Expected: FAIL because the apply file still contains one undifferentiated helper body.

- [ ] **Step 3: Rewire the apply seam**

Update [workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1):

```go
func applyPlatformAssetDispatchMutation(
	final *ListingKitResult,
	inventory *asset.Inventory,
	recipesByPlatform map[string][]assetrecipe.AssetRecipe,
	generationTasks []assetgeneration.Task,
	dispatchResult *assetgeneration.Result,
	bundleBuilder assetbundle.Builder,
) platformAssetDispatchMutation {
	mutation := platformAssetDispatchMutation{
		final:           final,
		inventory:       inventory,
		generationTasks: generationTasks,
	}
	if dispatchResult == nil {
		return mutation
	}
	mutation = buildPlatformAssetDispatchInventoryApplyPhase().run(
		mutation.final,
		mutation.inventory,
		dispatchResult.Assets,
	)
	bundleMutation := buildPlatformAssetDispatchBundleApplyPhase(bundleBuilder).run(
		mutation.final,
		mutation.inventory,
		recipesByPlatform,
		generationTasks,
		dispatchResult.Tasks,
	)
	mutation.generationTasks = bundleMutation.generationTasks
	return mutation
}
```

Keep the public helper name stable for this slice. The aim is not to change callers, only to stop one file from owning multiple mutation categories inline.

- [ ] **Step 4: Re-run focused orchestration verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(ApplyPlatformAssetDispatchMutation.*|WorkflowPlatformAssetDispatchApplyFileDelegatesToMutationSubSeams)" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/workflow_platform_asset_dispatch_apply.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: route listingkit asset dispatch apply through mutation sub-seams"
```

## Task 4: Lock mutation-side ownership boundaries

**Files:**
- Create: `internal/listingkit/phase8a_asset_dispatch_apply_boundary_test.go`
- Modify: `internal/listingkit/phase7a_asset_dispatch_boundary_test.go`

- [ ] **Step 1: Write the boundary tests**

Create [phase8a_asset_dispatch_apply_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase8a_asset_dispatch_apply_boundary_test.go:1):

```go
package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestWorkflowPlatformAssetDispatchInventoryApplyFileOwnsInventoryMutation(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("workflow_platform_asset_dispatch_inventory_apply.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_asset_dispatch_inventory_apply.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"inventory.Records = append(",
		"rebuildInventorySummary(",
		"rebuildBundleWithGeneratedAssets(",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_inventory_apply.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"mergeGenerationTasks(",
		"attachPlatformImageBundles(",
		"SaveInventory(",
		"SaveGenerationTasks(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_inventory_apply.go should not contain %q", needle)
		}
	}
}

func TestWorkflowPlatformAssetDispatchBundleApplyFileOwnsBundleTaskShaping(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("workflow_platform_asset_dispatch_bundle_apply.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_asset_dispatch_bundle_apply.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"attachPlatformImageBundles(",
		"mergeGenerationTasks(",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_bundle_apply.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"inventory.Records = append(",
		"rebuildInventorySummary(",
		"SaveInventory(",
		"SaveGenerationTasks(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_bundle_apply.go should not contain %q", needle)
		}
	}
}
```

Then extend [phase7a_asset_dispatch_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase7a_asset_dispatch_boundary_test.go:1) so `workflow_platform_asset_dispatch_apply.go` must contain:

```go
"buildPlatformAssetDispatchInventoryApplyPhase().run("
"buildPlatformAssetDispatchBundleApplyPhase(bundleBuilder).run("
```

and must not contain:

```go
"inventory.Records = append("
"rebuildInventorySummary("
"mergeGenerationTasks("
"attachPlatformImageBundles("
```

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestWorkflowPlatformAssetDispatch(InventoryApplyFileOwnsInventoryMutation|BundleApplyFileOwnsBundleTaskShaping|PhaseFileDelegatesToOrchestrationHelpers)|TestWorkflowPlatformAssetDispatchApplyFileDelegatesToMutationSubSeams" -count=1
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
git add internal/listingkit/phase7a_asset_dispatch_boundary_test.go internal/listingkit/phase8a_asset_dispatch_apply_boundary_test.go
git commit -m "test: lock listingkit asset dispatch mutation shaping boundaries"
```

## Self-Review Checklist

Before executing, verify the plan still satisfies the scope:

- it only reshapes mutation-side ownership, not durability policy
- it preserves current deferred dispatch business behavior
- it keeps inventory durability and generation-task persistence seams untouched
- it keeps all new seams feature-local inside `internal/listingkit`
- it adds both behavior and source-boundary protection

## Expected Outcome

When `Phase 8A` is complete:

- [workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1) will no longer directly own all mutation categories inline
- inventory mutation and bundle/task shaping will have separate local homes
- parent dispatch orchestration and durability boundaries from `Phase 7A/7B` will remain intact
- deferred dispatch mutation behavior will stay covered and stable
