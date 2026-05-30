# Task Processor Framework Phase 7B Asset-Dispatch Inventory Persistence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make ListingKit deferred asset-dispatch inventory persistence more explicit so `workflow_platform_asset_dispatch_phase.go` stops being the primary home of the “returned assets should now be durably saved” decision.

**Architecture:** Keep the work fully feature-owned inside `internal/listingkit`. Add a small local seam for post-dispatch inventory persistence, route the parent dispatch phase through that seam, and lock the new ownership line with source-boundary plus behavior tests. Preserve current best-effort persistence behavior and do not redesign mutation or summary/review semantics.

**Tech Stack:** Go, ListingKit workflow layer, `assetgeneration`, `assetrepo`, existing `workflow_assets_test.go` regression harness, source-boundary tests

---

## Out of Scope For This Slice

- redesigning dispatch-result mutation semantics again
- changing deferred asset-generation business policy
- introducing a generic workflow durability abstraction
- revisiting summary/review/finalization semantics
- moving workflow concerns into HTTP/runtime/bootstrap layers

## File Structure

### Existing hotspots

- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)
  - current parent seam that still decides when returned assets get persisted
- [internal/listingkit/workflow_platform_asset_dispatch_apply.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_apply.go:1)
  - already owns dispatch-result mutation and should not regrow persistence timing
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)
  - current behavior harness for deferred dispatch success/failure and persisted inventory outcomes
- [internal/listingkit/phase7a_asset_dispatch_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase7a_asset_dispatch_boundary_test.go:1)
  - current mutation/persist source-boundary guardrail

### Planned new files

- `internal/listingkit/workflow_platform_asset_dispatch_inventory_persist.go`
  - owns the post-dispatch durability decision for inventory persistence
- `internal/listingkit/phase7b_asset_dispatch_persist_boundary_test.go`
  - locks the new inventory-persist ownership split

### Files expected to shrink

- [internal/listingkit/workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1)
  - should become orchestration for pre-attach, dispatch/mutate, inventory-persist handoff, and generation-task persist handoff

Each file should have one clear responsibility. The design goal is not “one more helper file,” but “the parent dispatch seam no longer directly decides durability timing for returned inventory assets.”

## Task 1: Extract explicit inventory-persist seam

**Files:**
- Create: `internal/listingkit/workflow_platform_asset_dispatch_inventory_persist.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing seam-level tests**

Add direct seam-level tests near the existing dispatch behavior coverage in [workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1).

First add a success-path test:

```go
func TestPlatformAssetDispatchInventoryPersistPhaseRunPersistsReturnedAssets(t *testing.T) {
	t.Parallel()

	assetRepository := newStubWorkflowAssetRepository()
	phase := buildPlatformAssetDispatchInventoryPersistPhase(&service{assetRepo: assetRepository})
	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: "task-inventory-persist"},
		Records: []asset.AssetRecord{{
			ID:     "generated-main",
			Kind:   asset.KindSceneImage,
			Origin: asset.OriginGenerated,
			URL:    "https://cdn.example.com/generated-main.jpg",
		}},
		Summary: &asset.InventorySummary{TotalRecords: 1, GeneratedRecords: 1},
	}
	dispatchResult := &assetgeneration.Result{
		Assets: []asset.AssetRecord{{
			ID:     "generated-main",
			Kind:   asset.KindSceneImage,
			Origin: asset.OriginGenerated,
			URL:    "https://cdn.example.com/generated-main.jpg",
		}},
	}

	phase.run(context.Background(), inventory, dispatchResult)

	if assetRepository.saveInventoryCalls != 1 {
		t.Fatalf("save inventory calls = %d, want 1", assetRepository.saveInventoryCalls)
	}
	savedInventory, err := assetRepository.GetInventory(context.Background(), asset.InventoryRef{TaskID: "task-inventory-persist"})
	if err != nil {
		t.Fatalf("GetInventory() error = %v", err)
	}
	if !hasInventoryURL(savedInventory, "https://cdn.example.com/generated-main.jpg") {
		t.Fatalf("saved inventory = %+v, want generated asset persisted", savedInventory)
	}
}
```

Then add a no-op test for empty returned assets:

```go
func TestPlatformAssetDispatchInventoryPersistPhaseRunSkipsWhenNoReturnedAssets(t *testing.T) {
	t.Parallel()

	assetRepository := newStubWorkflowAssetRepository()
	phase := buildPlatformAssetDispatchInventoryPersistPhase(&service{assetRepo: assetRepository})
	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: "task-inventory-persist-skip"},
	}

	phase.run(context.Background(), inventory, &assetgeneration.Result{})

	if assetRepository.saveInventoryCalls != 0 {
		t.Fatalf("save inventory calls = %d, want 0", assetRepository.saveInventoryCalls)
	}
}
```

Finally add a best-effort failure-path test to preserve current behavior:

```go
func TestPlatformAssetDispatchInventoryPersistPhaseRunKeepsBestEffortPersistence(t *testing.T) {
	t.Parallel()

	assetRepository := newStubWorkflowAssetRepository()
	assetRepository.saveInventoryErr = fmt.Errorf("write failed")
	phase := buildPlatformAssetDispatchInventoryPersistPhase(&service{assetRepo: assetRepository})
	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: "task-inventory-persist-error"},
		Records: []asset.AssetRecord{{
			ID:     "generated-main",
			Kind:   asset.KindSceneImage,
			Origin: asset.OriginGenerated,
			URL:    "https://cdn.example.com/generated-main.jpg",
		}},
	}
	dispatchResult := &assetgeneration.Result{
		Assets: []asset.AssetRecord{{
			ID:     "generated-main",
			Kind:   asset.KindSceneImage,
			Origin: asset.OriginGenerated,
			URL:    "https://cdn.example.com/generated-main.jpg",
		}},
	}

	phase.run(context.Background(), inventory, dispatchResult)

	if assetRepository.saveInventoryCalls != 1 {
		t.Fatalf("save inventory calls = %d, want 1", assetRepository.saveInventoryCalls)
	}
}
```

- [ ] **Step 2: Run focused seam verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchInventoryPersistPhaseRun(PersistsReturnedAssets|SkipsWhenNoReturnedAssets|KeepsBestEffortPersistence)$" -count=1
```

Expected: FAIL because `buildPlatformAssetDispatchInventoryPersistPhase(...)` does not exist yet.

- [ ] **Step 3: Add the explicit inventory-persist seam**

Create `internal/listingkit/workflow_platform_asset_dispatch_inventory_persist.go`:

```go
package listingkit

import (
	"context"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
)

type platformAssetDispatchInventoryPersistPhase struct {
	service *service
}

func buildPlatformAssetDispatchInventoryPersistPhase(s *service) *platformAssetDispatchInventoryPersistPhase {
	return &platformAssetDispatchInventoryPersistPhase{service: s}
}

func (p *platformAssetDispatchInventoryPersistPhase) run(
	ctx context.Context,
	inventory *asset.Inventory,
	dispatchResult *assetgeneration.Result,
) {
	if p == nil || p.service == nil || p.service.assetRepo == nil || inventory == nil || dispatchResult == nil || len(dispatchResult.Assets) == 0 {
		return
	}
	_ = p.service.assetRepo.SaveInventory(ctx, inventory)
}
```

Keep this seam narrow and durability-focused. Do not move generation-task persistence or warnings into this file.

- [ ] **Step 4: Re-run focused seam verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchInventoryPersistPhaseRun(PersistsReturnedAssets|SkipsWhenNoReturnedAssets|KeepsBestEffortPersistence)$" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/workflow_platform_asset_dispatch_inventory_persist.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: extract listingkit asset dispatch inventory persistence seam"
```

## Task 2: Route parent dispatch seam through the inventory-persist handoff

**Files:**
- Modify: `internal/listingkit/workflow_platform_asset_dispatch_phase.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing orchestration-focused tests**

Extend the existing parent behavior coverage in [workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1).

Add a focused parent-seam assertion that inventory persistence still happens through the overall flow:

```go
func TestPlatformAssetDispatchPhaseRunPersistsInventoryThroughDurabilityHandoff(t *testing.T) {
	t.Parallel()

	assetRepository := newStubWorkflowAssetRepository()
	assetGenerator := &stubWorkflowAssetGenerator{
		dispatchResult: &assetgeneration.Result{
			Assets: []asset.AssetRecord{{
				ID:     "generated-main",
				Kind:   asset.KindSceneImage,
				Origin: asset.OriginGenerated,
				URL:    "https://cdn.example.com/generated-main.jpg",
			}},
			Tasks: []assetgeneration.Task{{
				ID:              "amazon-main",
				TaskID:          "task-dispatch-inventory-handoff",
				Platform:        "amazon",
				RecipeID:        "amazon-lifestyle",
				ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
				ExecutionStatus: "completed",
			}},
		},
	}
	phase := buildPlatformAssetDispatchPhase(&service{
		assetGenerator:     assetGenerator,
		assetRepo:          assetRepository,
		assetBundleBuilder: newDefaultAssetBundleBuilder(),
	})
	final := &ListingKitResult{
		Summary: &GenerationSummary{},
		Amazon:  &AmazonPackage{},
		AssetBundle: &asset.Bundle{
			Assets: []asset.Asset{{
				ID:   "source-1",
				Kind: asset.KindSourceImage,
				URL:  "https://example.com/source-1.jpg",
			}},
		},
		AssetInventorySummary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: "task-dispatch-inventory-handoff"},
		Records: []asset.AssetRecord{{
			ID:     "source-1",
			Kind:   asset.KindSourceImage,
			Origin: asset.OriginSource,
			URL:    "https://example.com/source-1.jpg",
		}},
		Summary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	recipesByPlatform := resolveRecipesForPlatforms(newDefaultAssetRecipeResolver(), []string{"amazon"}, nil)
	generationPlan := &assetgeneration.Result{
		Tasks: []assetgeneration.Task{{
			ID:              "amazon-main",
			TaskID:          "task-dispatch-inventory-handoff",
			Platform:        "amazon",
			RecipeID:        "amazon-lifestyle",
			ExecutionStatus: "planned",
			CanExecute:      true,
		}},
	}
	persistedGenerationTasks := []assetgeneration.Task{{
		ID:              "amazon-main",
		TaskID:          "task-dispatch-inventory-handoff",
		Platform:        "amazon",
		RecipeID:        "amazon-lifestyle",
		ExecutionStatus: "planned",
		CanExecute:      true,
	}}

	phase.run(context.Background(), &Task{ID: "task-dispatch-inventory-handoff"}, final, inventory, recipesByPlatform, generationPlan, persistedGenerationTasks, true)

	if assetRepository.saveInventoryCalls != 1 {
		t.Fatalf("save inventory calls = %d, want 1", assetRepository.saveInventoryCalls)
	}
}
```

Also add a source-shape test near the existing orchestration checks:

```go
func TestWorkflowPlatformAssetDispatchPhaseFileUsesInventoryPersistHandoff(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("workflow_platform_asset_dispatch_phase.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_asset_dispatch_phase.go) error = %v", err)
	}
	content := string(src)

	if !strings.Contains(content, "p.persistInventory(") {
		t.Fatalf("workflow_platform_asset_dispatch_phase.go should contain %q", "p.persistInventory(")
	}
	if strings.Contains(content, "SaveInventory(ctx, inventory)") {
		t.Fatalf("workflow_platform_asset_dispatch_phase.go should not contain direct SaveInventory call")
	}
}
```

- [ ] **Step 2: Run focused orchestration verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchPhaseRunPersistsInventoryThroughDurabilityHandoff|TestWorkflowPlatformAssetDispatchPhaseFileUsesInventoryPersistHandoff" -count=1
```

Expected: FAIL because the parent seam still contains the direct inventory save.

- [ ] **Step 3: Rewire the parent seam**

Update [workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1):

```go
func (p *platformAssetDispatchPhase) dispatchAndApply(
	ctx context.Context,
	task *Task,
	final *ListingKitResult,
	inventory *asset.Inventory,
	recipesByPlatform map[string][]assetrecipe.AssetRecipe,
	persistedGenerationTasks []assetgeneration.Task,
	pendingTasks []assetgeneration.Task,
	enableAssetGeneration bool,
) (*ListingKitResult, *asset.Inventory, []assetgeneration.Task, *assetgeneration.Result) {
	if !enableAssetGeneration || p.service.assetGenerator == nil || len(pendingTasks) == 0 {
		return final, inventory, persistedGenerationTasks, nil
	}
	deferredStage := newWorkflowRecorder(final).Start("asset_generation_platform", "")
	dispatchResult, dispatchErr := p.service.assetGenerator.Dispatch(ctx, assetgeneration.DispatchRequest{
		TaskID:    task.ID,
		Product:   final.CatalogProduct,
		Inventory: inventory,
		Tasks:     pendingTasks,
	})
	if dispatchErr != nil {
		deferredStage.Degrade("asset_generation_platform_deferred_dispatch_failed", "Deferred platform asset generation dispatch failed", dispatchErr.Error())
	}
	if dispatchResult != nil {
		mutation := applyPlatformAssetDispatchMutation(
			final,
			inventory,
			recipesByPlatform,
			persistedGenerationTasks,
			dispatchResult,
			p.service.assetBundleBuilder,
		)
		final = mutation.final
		inventory = mutation.inventory
		persistedGenerationTasks = mutation.generationTasks
	}
	if dispatchErr == nil {
		deferredStage.Complete()
	}
	return final, inventory, persistedGenerationTasks, dispatchResult
}
```

and in `run(...)`:

```go
final, inventory, persistedGenerationTasks, dispatchResult := p.dispatchAndApply(
	ctx,
	task,
	final,
	inventory,
	recipesByPlatform,
	persistedGenerationTasks,
	pendingTasks,
	enableAssetGeneration,
)
p.persistInventory(ctx, inventory, dispatchResult)
return p.persistHandoff(ctx, task, final, persistedGenerationTasks)
```

Add the new helper:

```go
func (p *platformAssetDispatchPhase) persistInventory(
	ctx context.Context,
	inventory *asset.Inventory,
	dispatchResult *assetgeneration.Result,
) {
	buildPlatformAssetDispatchInventoryPersistPhase(p.service).run(ctx, inventory, dispatchResult)
}
```

Do not move generation-task persistence into this helper. Keep the slice limited to inventory durability ownership.

- [ ] **Step 4: Re-run focused orchestration verification**

Run:

```powershell
go test ./internal/listingkit -run "TestPlatformAssetDispatchPhaseRunPersistsInventoryThroughDurabilityHandoff|TestWorkflowPlatformAssetDispatchPhaseFileUsesInventoryPersistHandoff" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/workflow_platform_asset_dispatch_phase.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: route listingkit asset dispatch through inventory persistence seam"
```

## Task 3: Lock inventory-persist ownership boundaries

**Files:**
- Create: `internal/listingkit/phase7b_asset_dispatch_persist_boundary_test.go`
- Modify: `internal/listingkit/phase7a_asset_dispatch_boundary_test.go`

- [ ] **Step 1: Write the boundary tests**

Create [phase7b_asset_dispatch_persist_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase7b_asset_dispatch_persist_boundary_test.go:1):

```go
package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestWorkflowPlatformAssetDispatchInventoryPersistFileOwnsInventoryDurability(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("workflow_platform_asset_dispatch_inventory_persist.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_asset_dispatch_inventory_persist.go) error = %v", err)
	}
	content := string(src)

	if !strings.Contains(content, "SaveInventory(ctx, inventory)") {
		t.Fatalf("workflow_platform_asset_dispatch_inventory_persist.go should contain %q", "SaveInventory(ctx, inventory)")
	}

	for _, needle := range []string{
		"decorateListingKitResultGeneration(",
		"SaveGenerationTasks(",
		"mergeGenerationTasks(",
		"rebuildInventorySummary(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_asset_dispatch_inventory_persist.go should not contain %q", needle)
		}
	}
}
```

Then expand [phase7a_asset_dispatch_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase7a_asset_dispatch_boundary_test.go:1) so the parent seam now must contain:

```go
"p.persistInventory("
```

and must not contain:

```go
"SaveInventory(ctx, inventory)"
```

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestWorkflowPlatformAssetDispatchInventoryPersistFileOwnsInventoryDurability|TestWorkflowPlatformAssetDispatchPhaseFileDelegatesToOrchestrationHelpers" -count=1
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
git add internal/listingkit/phase7a_asset_dispatch_boundary_test.go internal/listingkit/phase7b_asset_dispatch_persist_boundary_test.go
git commit -m "test: lock listingkit asset dispatch inventory persistence boundaries"
```

## Self-Review Checklist

Before executing, verify the plan still satisfies the scope:

- it only moves inventory durability ownership, not mutation semantics
- it preserves current best-effort `SaveInventory(...)` behavior
- it does not re-open review/summary/finalization seams
- it keeps all new seams feature-local inside `internal/listingkit`
- it adds both behavior and source-boundary protection

## Expected Outcome

When `Phase 7B` is complete:

- [workflow_platform_asset_dispatch_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_asset_dispatch_phase.go:1) will no longer directly call `SaveInventory(...)`
- inventory durability ownership will have one explicit feature-owned seam
- dispatch-result mutation and generation-task persistence seams will remain separate
- deferred dispatch persisted-inventory behavior will stay covered and stable
