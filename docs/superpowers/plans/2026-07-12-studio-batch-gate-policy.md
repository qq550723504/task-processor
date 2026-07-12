# Studio Batch Gate Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Delegate deterministic Studio batch candidate admission decisions to internal/listingkit/studiobatch while retaining store/baseline integration and side effects in root ListingKit.

**Architecture:** Extend the existing neutral candidate values with the minimum design, item, and variant-surface facts needed by a pure EvaluateGate function. Root ListingKit maps its records to that input, delegates the first three admission phases, then retains store and baseline calls, their caches, tenant resolution, errors, rejected-link persistence, and task creation.

**Tech Stack:** Go 1.26+, Go standard library, existing internal/listing/studio, current ListingKit gate tests.

## Global Constraints

- Do not import root ListingKit, HTTP, GORM, Temporal, SDS clients, store repositories, remote Studio clients, or external SDKs from studiobatch.
- Do not move store validation, SDS baseline checks, tenant resolution, cache maps, task-link persistence, or task creation.
- Preserve rejection codes, messages, ordering, public contracts, candidate keys, and task behavior.
- Preserve the root call order: pure admission, store validation, baseline validation.
- Keep go.work.sum unchanged.

---

## File Map

- Modify: internal/listingkit/studiobatch/model.go — add neutral variant surface, item ownership, design admission fields, GateInput, and GateResult.
- Create: internal/listingkit/studiobatch/gate.go — pure admission evaluator.
- Create: internal/listingkit/studiobatch/gate_test.go — table-driven admission tests.
- Modify: internal/listingkit/studiobatch/boundary_guard_test.go — retain package import restriction over the new file.
- Modify: internal/listingkit/studio_batch_task_gate.go — root adapter plus external store/baseline phases only.
- Modify: internal/listingkit/studio_batch_task_gate_test.go — root external-order characterization.
- Create: internal/listingkit/phase106_studio_batch_gate_boundary_test.go — AST guard for the root delegation.
- Modify: docs/refactoring/listingkit-boundary-checkpoint.md — record pure gate ownership.

### Task 1: Define and Test the Pure Gate Contract

**Files:**
- Modify: internal/listingkit/studiobatch/model.go
- Create: internal/listingkit/studiobatch/gate.go
- Create: internal/listingkit/studiobatch/gate_test.go

**Interfaces:**
- Produces: EvaluateGate(input GateInput) GateResult.
- GateInput contains BatchID, BatchGroupMode, Candidate, Designs, SelectionByID, and ItemSelections.
- GateResult contains Eligible, ReasonCode, and Message.

- [ ] **Step 1: Write failing pure-gate tests**

Create gate_test.go with one eligible test and table cases for design_not_found, design_target_mismatch, design_not_approved, design_image_missing, selection_not_in_batch, selection_not_in_item, selection_identity_incomplete, selection_variant_incompatible, compatibility_incomplete, compatibility_mismatch, and target-group design_target_mismatch.

Use this shared fixture shape:

~~~go
func eligibleGateInput() GateInput {
	selection := GroupedSelection{
		SelectionID: "selection-1",
		Selection: Selection{
			ParentProductID:  2002,
			PrototypeGroupID: 4004,
			VariantID:        6006,
			LayerID:          "layer-front",
			DesignType:       "material",
			PrintableWidth:   1200,
			PrintableHeight:  1200,
			TemplateImageURL: "https://cdn.example.com/template.png",
			MaskImageURL:     "https://cdn.example.com/mask.png",
			Variants: []VariantSurface{{
				VariantID: 6006, PrototypeGroupID: 4004,
				LayerID: "layer-front",
				TemplateImageURL: "https://cdn.example.com/template.png",
				MaskImageURL: "https://cdn.example.com/mask.png",
			}},
		},
	}
	design := Design{
		ID: "design-1", BatchID: "batch-1", ItemID: "item-1",
		Approved: true, ImageURL: "https://cdn.example.com/design.png",
	}
	return GateInput{
		BatchID: "batch-1", BatchGroupMode: "shared_by_size",
		Candidate: Candidate{
			Design: design, Item: Item{
				ID: "item-1", SelectionIDs: []string{"selection-1"},
				GroupMode: "shared_by_size",
			},
			Selection: selection, SelectionSnapshot: selection.Selection,
			SelectionID: "selection-1",
		},
		Designs: []Design{design},
		SelectionByID: map[string]GroupedSelection{"selection-1": selection},
		ItemSelections: []GroupedSelection{selection},
	}
}
~~~

- [ ] **Step 2: Verify RED**

Run:

~~~powershell
$env:GOWORK='off'
go test ./internal/listingkit/studiobatch -run TestEvaluateGate -count=1
~~~

Expected: compilation failure because GateInput, GateResult, VariantSurface, and EvaluateGate do not exist.

- [ ] **Step 3: Add neutral admission values**

Add these fields without changing existing JSON or root DTOs:

~~~go
type VariantSurface struct {
	VariantID        int64
	PrototypeGroupID int64
	LayerID          string
	TemplateImageURL string
	MaskImageURL     string
}

type GateInput struct {
	BatchID        string
	BatchGroupMode string
	Candidate      Candidate
	Designs        []Design
	SelectionByID  map[string]GroupedSelection
	ItemSelections []GroupedSelection
}

type GateResult struct {
	Eligible   bool
	ReasonCode string
	Message    string
}
~~~

Extend Selection with Variants []VariantSurface, Item with SelectionIDs []string, and Design with BatchID, ItemID, Approved, and ImageURL.

- [ ] **Step 4: Implement the pure evaluator**

Create gate.go. EvaluateGate must call private helpers in this exact order:

1. evaluateGateDesign
2. evaluateGateSelection
3. evaluateGateCompatibility

Each helper returns GateResult. Rejection messages must be byte-for-byte equal to the current root messages in studio_batch_task_gate.go. Use the existing package compatibilityFingerprint, compatibilityComplete, selectionIDForSnapshot, and firstNonEmpty helpers; do not duplicate hashing or design-type policy.

- [ ] **Step 5: Verify GREEN and commit**

Run:

~~~powershell
gofmt -w internal/listingkit/studiobatch
$env:GOWORK='off'
go test ./internal/listingkit/studiobatch -count=1
git add internal/listingkit/studiobatch
git commit -m "refactor: define studio batch gate policy"
~~~

Expected: PASS and a self-contained pure-gate commit.

### Task 2: Delegate Pure Admission from the Root Gate

**Files:**
- Modify: internal/listingkit/studio_batch_task_gate.go
- Modify: internal/listingkit/studio_batch_task_gate_test.go

**Interfaces:**
- Consumes: studiobatch.EvaluateGate.
- Produces: unchanged studioBatchTaskGateResult and preserves root store/baseline phases.

- [ ] **Step 1: Add root order characterization tests**

Append tests that use the existing stub validators:

1. mutate the eligible evaluation so its design is unapproved; assert result is design_not_approved and both store.calls and baseline.calls remain zero.
2. leave pure admission eligible but configure the store validator as unavailable; assert result is store_not_available and baseline.calls remains zero.
3. leave store eligible but configure baseline entry blocked; assert result is baseline_not_ready.

Run these tests before production edits. Expected: PASS as a characterization of current behavior.

- [ ] **Step 2: Add root-to-neutral adapters**

In studio_batch_task_gate.go add private mapping functions:

~~~go
func studioBatchGateInput(eval *studioBatchTaskGateEvaluation) studiobatch.GateInput
func studioBatchGateSelection(selection SheinStudioSelection) studiobatch.Selection
func studioBatchGateGroupedSelection(grouped SheinStudioGroupedSelection) studiobatch.GroupedSelection
func studioBatchGateDesign(design StudioMaterializedDesignRecord) studiobatch.Design
func studioBatchGateItem(item StudioBatchItemRecord) studiobatch.Item
func studioBatchGateResult(result studiobatch.GateResult) studioBatchTaskGateResult
~~~

Map root review status Approved to a boolean, root selection IDs to a copied string slice, and root selection variants to VariantSurface values. No root record pointer enters studiobatch.

- [ ] **Step 3: Replace the root pure phases**

In studioBatchTaskGate.Evaluate replace the direct evaluateDesign, evaluateSelection, and evaluateCompatibility calls with:

~~~go
if result := studioBatchGateResult(studiobatch.EvaluateGate(studioBatchGateInput(eval))); !result.Eligible {
	return result, nil
}
~~~

Retain evaluateStore and evaluateBaseline in their current order and leave their cache fields, validator interfaces, tenant helper, and error behavior unchanged.

Delete root-only pure helpers after confirming they have no remaining production callers: evaluateDesign, evaluateSelection, evaluateCompatibility, studioBatchTaskItemOwnsSelection, and studioBatchSelectionVariantsCompatible. Keep root adapter functions and external integration methods.

- [ ] **Step 4: Verify root behavior**

Run:

~~~powershell
gofmt -w internal/listingkit/studio_batch_task_gate.go internal/listingkit/studio_batch_task_gate_test.go
$env:GOWORK='off'
go test ./internal/listingkit -run "TestStudioBatchTaskGate" -count=1
~~~

Expected: PASS, including the external-call order tests.

- [ ] **Step 5: Commit the root delegation**

~~~powershell
git add internal/listingkit/studio_batch_task_gate.go internal/listingkit/studio_batch_task_gate_test.go
git commit -m "refactor: delegate studio batch gate policy"
~~~

### Task 3: Guard the Boundary and Verify the Slice

**Files:**
- Create: internal/listingkit/phase106_studio_batch_gate_boundary_test.go
- Modify: docs/refactoring/listingkit-boundary-checkpoint.md

- [ ] **Step 1: Add a failing AST boundary test**

Parse studio_batch_task_gate.go using go/parser. Require the exact studiobatch import path and a selector call studiobatch.EvaluateGate inside the root Evaluate method. Reject root declarations named evaluateDesign, evaluateSelection, evaluateCompatibility, studioBatchTaskItemOwnsSelection, and studioBatchSelectionVariantsCompatible.

Run the named test before the root delegation deletion. Expected: FAIL because the legacy declarations still exist.

- [ ] **Step 2: Use syntax inspection, not source substrings**

Use go/ast to inspect imports, function declarations, and call expressions. A comment or string containing studiobatch.EvaluateGate must not satisfy the guard.

- [ ] **Step 3: Update the checkpoint**

Add to the existing studiobatch ownership text that it now owns deterministic candidate gate admission: design, selection, variant-surface, compatibility, and target-group checks. State that root retains store/baseline integration, caches, tenant resolution, errors, persistence, and task orchestration.

- [ ] **Step 4: Run full affected verification**

Run:

~~~powershell
gofmt -w internal/listingkit/studiobatch internal/listingkit/studio_batch_task_gate.go internal/listingkit/studio_batch_task_gate_test.go internal/listingkit/phase106_studio_batch_gate_boundary_test.go
git diff --check
$env:GOWORK='off'
go test ./internal/listingkit/studiobatch -count=1
go test ./internal/listingkit -run "TestStudioBatchTaskGate|TestStudioBatchCandidateBoundary" -count=1
go test ./internal/listingkit/... -count=1
go vet ./internal/listingkit/...
~~~

Expected: PASS.

- [ ] **Step 5: Commit the guard and ownership record**

~~~powershell
git add internal/listingkit/phase106_studio_batch_gate_boundary_test.go docs/refactoring/listingkit-boundary-checkpoint.md
git commit -m "docs: record studio batch gate ownership"
~~~

## Final Acceptance Checklist

- [ ] studiobatch owns only deterministic gate admission and reuses its existing compatibility helpers.
- [ ] Root retains store/baseline validators, caches, tenant resolution, persistence, and task orchestration.
- [ ] Root pure admission runs before store and baseline checks.
- [ ] Rejection reason codes, messages, and external-call order are unchanged.
- [ ] New child-package files import only the standard library and internal/listing/studio.
- [ ] AST guard verifies real root delegation and retired pure helper removal.
- [ ] All ListingKit subpackage tests and go vet pass.
- [ ] go.work.sum remains unchanged.
