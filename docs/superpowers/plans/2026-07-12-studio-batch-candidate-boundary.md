# Studio Batch Candidate Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Move deterministic ListingKit Studio batch candidate ownership, grouping, rejection, and stable-key policy into internal/listingkit/studiobatch without changing public behavior.

**Architecture:** The new child package consumes neutral candidate values and imports only the Go standard library plus internal/listing/studio. Root ListingKit loads and hydrates existing records, adapts them to neutral values, delegates evaluation, then maps results back to the current internal candidate and rejection types before persistence or task creation.

**Tech Stack:** Go 1.26+, Go standard library, existing internal/listing/studio policy helpers, current ListingKit characterization tests.

## Global Constraints

- Do not duplicate generic draft, batch naming, status, or completion policy already in internal/listing/studio.
- Do not import internal/listingkit, HTTP, GORM, Temporal, SDS clients, remote Studio clients, or external SDKs from internal/listingkit/studiobatch.
- Preserve ListingKit public JSON, GORM records, repository interfaces, task-link keys, task creation, persistence ordering, remote execution, and error messages.
- Retain SDS hydration, durable task-link lookup, gate evaluation, and persistence in root ListingKit.
- Preserve candidate/rejection order, explicit item selection ownership, group-mode behavior, fallback selection order, and stable candidate key inputs.
- Do not change go.work.sum.

---

## File Map

- Create: internal/listingkit/studiobatch/model.go — neutral batch, item, design, selection, candidate, rejection, and result values.
- Create: internal/listingkit/studiobatch/evaluate.go — pure selection resolution, design-type normalization, grouping, compatibility validation, fingerprinting, and key construction.
- Create: internal/listingkit/studiobatch/evaluate_test.go — table-driven candidate and rejection behavior tests.
- Create: internal/listingkit/studiobatch/boundary_guard_test.go — production import guard.
- Modify: internal/listingkit/task_studio_batch_candidate_support.go — retain record hydration and side effects; adapt root records to/from studiobatch.
- Modify: internal/listingkit/studio_batch_compatibility_test.go — preserve stable candidate-key characterization through the root adapter.
- Create: internal/listingkit/phase105_studio_batch_candidate_boundary_test.go — AST guard for the root delegation and retired pure helper family.

### Task 1: Establish the Neutral Candidate Contract

**Files:**
- Create: internal/listingkit/studiobatch/model.go
- Create: internal/listingkit/studiobatch/evaluate.go
- Create: internal/listingkit/studiobatch/evaluate_test.go
- Create: internal/listingkit/studiobatch/boundary_guard_test.go

**Interfaces:**
- Produces: Evaluate(input EvaluationInput) EvaluationResult.
- EvaluationInput contains TenantID, BatchID, BatchGroupMode, BatchStoreID, BatchSelection, Item, Design, ResolvedSelections, and ExplicitSelectionOwnership.
- EvaluationResult contains Candidates and Rejections, preserving input order.

- [ ] **Step 1: Write failing package tests**

Create evaluate_test.go with tests that require the new API:

~~~go
func TestEvaluateUsesExplicitItemSelectionOwnership(t *testing.T) {
	result := Evaluate(EvaluationInput{
		BatchID: "batch-1",
		Item: Item{ID: "item-1", GroupMode: "per_product"},
		Design: Design{ID: "design-1"},
		ResolvedSelections: []GroupedSelection{{
			SelectionID: "selected-1",
			Selection: Selection{VariantID: 10, DesignType: "material"},
		}},
		ExplicitSelectionOwnership: true,
	})
	if len(result.Candidates) != 1 || result.Candidates[0].SelectionID != "selected-1" {
		t.Fatalf("candidates = %+v", result.Candidates)
	}
}

func TestEvaluateRejectsPerProductMultipleSelections(t *testing.T) {
	result := Evaluate(EvaluationInput{
		BatchID: "batch-1",
		Item: Item{ID: "item-1", GroupMode: "per_product"},
		Design: Design{ID: "design-1"},
		ResolvedSelections: []GroupedSelection{
			{SelectionID: "first", Selection: Selection{VariantID: 10}},
			{SelectionID: "second", Selection: Selection{VariantID: 20}},
		},
		ExplicitSelectionOwnership: true,
	})
	if len(result.Candidates) != 0 || len(result.Rejections) != 1 ||
		result.Rejections[0].ReasonCode != "selection_cardinality_mismatch" {
		t.Fatalf("result = %+v", result)
	}
}
~~~

- [ ] **Step 2: Verify RED**

Run:

~~~powershell
$env:GOWORK='off'
go test ./internal/listingkit/studiobatch -count=1
~~~

Expected: compilation failure because Evaluate and the neutral values do not exist.

- [ ] **Step 3: Add the exact neutral values and minimal evaluator**

Create model.go with concrete value types. Keep IDs and display fields as strings, numeric product identifiers as int64, and ordering as slices:

~~~go
type Selection struct {
	VariantID        int64
	ParentProductID  int64
	PrototypeGroupID int64
	LayerID          string
	DesignType       string
	PrintableWidth   int
	PrintableHeight  int
	TemplateImageURL string
	MaskImageURL     string
	ProductSize      string
	PackagingSpec    string
	VariantLabel     string
	ProductName      string
}

type GroupedSelection struct {
	SelectionID string
	StoreID     int64
	Selection   Selection
}

type Item struct {
	ID               string
	TargetGroupKey   string
	TargetGroupLabel string
	GroupMode        string
}

type Design struct {
	ID               string
	TargetGroupKey   string
	TargetGroupLabel string
}

type EvaluationInput struct {
	TenantID                   string
	BatchID                    string
	BatchGroupMode             string
	BatchStoreID               int64
	BatchSelection             Selection
	Item                       Item
	Design                     Design
	ResolvedSelections         []GroupedSelection
	ExplicitSelectionOwnership bool
	FallbackSelection          GroupedSelection
}

type Candidate struct {
	Design                   Design
	Item                     Item
	Selection                GroupedSelection
	SelectionSnapshot        Selection
	SelectionID              string
	CompatibilityFingerprint string
	CandidateKey             string
	StoreID                  int64
	StyleID                  string
	Title                    string
}

type Rejection struct {
	DesignID    string
	ItemID      string
	SelectionID string
	ReasonCode  string
	Message     string
}

type EvaluationResult struct {
	Candidates []Candidate
	Rejections []Rejection
}
~~~

Create Evaluate in evaluate.go. It must use studio.NormalizeBatchDesignType for blank design types, prefer Item.GroupMode over BatchGroupMode, apply the existing selection-ID fallback order, return selection_not_in_batch when no selection resolves, reject per_product inputs with any cardinality other than one, and reject multi-selection non-per_product inputs with compatibility_incomplete or compatibility_mismatch using the current field set.

- [ ] **Step 4: Verify GREEN and add key characterization**

Add a table case where only ProductSize changes and assert CandidateKey changes. Run:

~~~powershell
gofmt -w internal/listingkit/studiobatch
$env:GOWORK='off'
go test ./internal/listingkit/studiobatch -count=1
~~~

Expected: PASS.

- [ ] **Step 5: Add the package import guard and commit**

The guard scans all non-test Go files in the directory and permits only standard-library imports and task-processor/internal/listing/studio. It rejects every other task-processor path and all dotted third-party imports.

Run:

~~~powershell
$env:GOWORK='off'
go test ./internal/listingkit/studiobatch -count=1
git add internal/listingkit/studiobatch
git commit -m "refactor: define studio batch candidate policy"
~~~

Expected: PASS and a new platform-neutral candidate package commit.

### Task 2: Add the Root ListingKit Candidate Adapter

**Files:**
- Modify: internal/listingkit/task_studio_batch_candidate_support.go
- Modify: internal/listingkit/studio_batch_compatibility_test.go

**Interfaces:**
- Consumes: studiobatch.EvaluationInput and EvaluationResult.
- Produces: the existing studioBatchTaskCandidate and SheinStudioRejectedTask values without changing callers.

- [ ] **Step 1: Add a root characterization test**

In studio_batch_compatibility_test.go, create a root candidate fixture with a tenant, batch, item, design, and one hydrated selection. Call the existing root candidate builder and assert its CandidateKey is exactly the key returned when the same fields are adapted through studiobatch. The test must also assert a ProductSize-only change produces a different key.

- [ ] **Step 2: Run the root test before the adapter change**

Run:

~~~powershell
$env:GOWORK='off'
go test ./internal/listingkit -run TestStudioBatchTaskCandidateKey -count=1
~~~

Expected: PASS; this is the legacy characterization oracle.

- [ ] **Step 3: Build a narrow root adapter**

In task_studio_batch_candidate_support.go:

1. Keep buildStudioBatchTaskCandidates responsible for selection lookup, missing-selection rejections, fallback selection selection, and SDS hydration.
2. Add a private function that maps one hydrated root batch/item/design/selection set into studiobatch.EvaluationInput.
3. Call studiobatch.Evaluate.
4. Map Candidate back to studioBatchTaskCandidate and Rejection back to SheinStudioRejectedTask.
5. Keep gate evaluation, task-link persistence, durable-task lookup, and task creation unchanged.

Remove only the root pure helpers replaced by the package: group-mode resolution, grouped-selection design-type normalization, per-design candidate construction, compatibility completeness checks, pure fingerprint construction, and key construction. Do not remove root helpers still used by hydration or persistence.

- [ ] **Step 4: Verify root compatibility**

Run:

~~~powershell
gofmt -w internal/listingkit/task_studio_batch_candidate_support.go internal/listingkit/studio_batch_compatibility_test.go
$env:GOWORK='off'
go test ./internal/listingkit -run "TestStudioBatchTaskCandidateKey|TestBuildStudioBatchTaskCandidates" -count=1
~~~

Expected: PASS with unchanged root behavior.

- [ ] **Step 5: Commit the adapter**

~~~powershell
git add internal/listingkit/task_studio_batch_candidate_support.go internal/listingkit/studio_batch_compatibility_test.go
git commit -m "refactor: delegate studio batch candidate policy"
~~~

### Task 3: Add the Boundary Guard and Verify the Slice

**Files:**
- Create: internal/listingkit/phase105_studio_batch_candidate_boundary_test.go
- Modify: docs/refactoring/listingkit-boundary-checkpoint.md

**Interfaces:**
- Consumes: root adapter and studiobatch package.
- Produces: architecture regression protection and an ownership checkpoint.

- [ ] **Step 1: Write a failing AST boundary test**

Create phase105_studio_batch_candidate_boundary_test.go. Parse task_studio_batch_candidate_support.go with go/parser. Require:

- an import of task-processor/internal/listingkit/studiobatch;
- a selector call studiobatch.Evaluate;
- no root function declarations named buildStudioBatchTaskCandidatesForDesign, studioBatchTaskCandidateGroupMode, normalizeStudioBatchTaskGroupedSelection, normalizeStudioBatchTaskDesignType, buildStudioBatchTaskCandidateKey, or studioBatchTaskCandidateCompatibilityFingerprint.

Run the named test before adding AST helper support or before retiring the root helpers.

Expected: FAIL if the current root implementation still owns the retired policy.

- [ ] **Step 2: Implement the smallest AST helper or reuse an existing one**

Use existing ListingKit AST test helpers if they already provide parsed imports, function declarations, and selector-call checks. Do not introduce a second string-based guard. The completed test must inspect syntax, not comments or source substrings.

- [ ] **Step 3: Update the active ownership checkpoint**

Add an internal/listingkit/studiobatch entry stating that it owns neutral, deterministic ListingKit Studio batch candidate evaluation and imports only the standard library plus internal/listing/studio. State that root ListingKit retains legacy DTO adaptation, record hydration, gate evaluation, durable task-link behavior, task creation, persistence, and API orchestration.

- [ ] **Step 4: Run focused and package verification**

Run:

~~~powershell
gofmt -w internal/listingkit/studiobatch internal/listingkit/task_studio_batch_candidate_support.go internal/listingkit/phase105_studio_batch_candidate_boundary_test.go
git diff --check
$env:GOWORK='off'
go test ./internal/listingkit/studiobatch -count=1
go test ./internal/listingkit -run "TestStudioBatchTaskCandidateKey|TestBuildStudioBatchTaskCandidates|TestStudioBatchCandidateBoundary" -count=1
go test ./internal/listingkit/... -count=1
go vet ./internal/listingkit/...
~~~

Expected: PASS.

- [ ] **Step 5: Commit documentation and verification guards**

~~~powershell
git add internal/listingkit/phase105_studio_batch_candidate_boundary_test.go docs/refactoring/listingkit-boundary-checkpoint.md
git commit -m "docs: record studio batch candidate ownership"
~~~

## Final Acceptance Checklist

- [ ] studiobatch reuses internal/listing/studio generic policy and does not duplicate it.
- [ ] Root ListingKit retains all persistence, remote, SDS hydration, gate, task-link, and task-creation behavior.
- [ ] Candidate/rejection ordering, reason codes, group-mode fallback, selection ownership, fingerprints, and candidate key inputs remain unchanged.
- [ ] Public JSON, GORM, repository, API, and task contracts remain unchanged.
- [ ] studiobatch depends only on the standard library and internal/listing/studio.
- [ ] The root adapter has an AST-backed delegation guard.
- [ ] Focused and ListingKit package verification pass.
- [ ] go.work.sum remains unchanged.
