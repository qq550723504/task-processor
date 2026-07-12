# SDS POD Canonical Revision Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reapply completed SDS POD facts during SHEIN revision refresh while preserving the no-result style-only fallback.

**Architecture:** Root ListingKit remains the adapter from `SDSSyncSummary` and `SDSSyncOptions` to neutral `sdspod.CanonicalMetadata`. `refreshSheinDerivedState` selects the completed-result path or the existing style-only fallback; `sdspod` remains unchanged and never imports ListingKit types.

**Tech Stack:** Go 1.26, existing `internal/product/sourcing/sdspod`, Go testing, `gofmt`, `go vet`.

## Global Constraints

- Do not modify `sdspod.CanonicalMetadata`, SDS result DTOs, or persistence schemas.
- Preserve SHEIN revision ordering: canonical refresh remains before the publish-request build and all resolver calls.
- Use full SDS metadata only when `task.Result.SDSDesignResult != nil`.
- When no SDS result exists, retain the current style-only update and do not add title, identity, or image mutations.
- Keep `go.work.sum` unchanged.

---

## File Map

- Modify: `internal/listingkit/service_revision_recompute.go` — select completed-result metadata application or style-only fallback.
- Modify: `internal/listingkit/service_revision_test.go` — characterize completed-result canonical refresh and no-result compatibility.
- Modify: `docs/refactoring/listingkit-boundary-checkpoint.md` only if the ownership record becomes inaccurate; it is not expected to change for this slice.

### Task 1: Characterize SDS Revision Outcomes

**Files:**

- Modify: `internal/listingkit/service_revision_test.go`

**Interfaces:**

- Consumes: `(*service).refreshSheinDerivedState(task *Task, req *ApplyRevisionRequest)`.
- Produces: regression coverage for completed SDS result alignment and the style-only fallback.

- [ ] **Step 1: Add a failing completed-result regression test**

Add this test next to `TestRefreshSheinDerivedStateAppliesSDSStyleToEveryCanonicalVariant`:

```go
func TestRefreshSheinDerivedStateReappliesCompletedSDSCanonicalMetadata(t *testing.T) {
	task := &Task{
		Request: &GenerateRequest{Options: &GenerateOptions{SDS: &SDSSyncOptions{
			StyleName: "Studio A1",
		}}},
		Result: &ListingKitResult{
			CanonicalProduct: &canonical.Product{
				Title: "Stale title",
				Attributes: map[string]canonical.Attribute{"sku": {Value: "STALE"}},
				Images: []canonical.Image{{URL: "https://cdn.example.com/stale.jpg", Role: "primary"}},
				Variants: []canonical.Variant{{
					SKU: "SKU-RED",
					Attributes: map[string]canonical.Attribute{"source_sds_sku": {Value: "SDS-RED"}},
				}},
			},
			SDSDesignResult: &SDSSyncSummary{
				ProductName: "Rendered clock", ProductSKU: "PARENT-1",
				VariantSKU: "SDS-RED", VariantColor: "Red",
				MockupImageURLs: []string{"https://cdn.example.com/rendered-main.jpg"},
				VariantResults: []SDSSyncSummary{{
					VariantSKU: "SDS-RED", VariantColor: "Red", Status: "completed",
					MockupImageURLs: []string{"https://cdn.example.com/rendered-red.jpg"},
				}},
			},
			Shein: &SheinPackage{RequestDraft: &SheinRequestDraft{}},
		},
	}

	(&service{}).refreshSheinDerivedState(task, &ApplyRevisionRequest{
		Platform: "shein", Shein: &SheinRevisionInput{RegenerateAttributes: true},
	})

	product := task.Result.CanonicalProduct
	if product.Title != "Rendered clock" || product.Attributes["product_sku"].Value != "PARENT-1" {
		t.Fatalf("canonical identity = %+v", product)
	}
	if len(product.Images) != 1 || product.Images[0].URL != "https://cdn.example.com/rendered-red.jpg" {
		t.Fatalf("product images = %+v", product.Images)
	}
	if len(product.Variants[0].Images) != 1 || product.Variants[0].Images[0].URL != "https://cdn.example.com/rendered-red.jpg" {
		t.Fatalf("variant images = %+v", product.Variants[0].Images)
	}
	if product.Variants[0].Attributes["ai_style"].Value != "Studio A1" {
		t.Fatalf("ai_style = %+v", product.Variants[0].Attributes["ai_style"])
	}
}
```

- [ ] **Step 2: Verify RED**

Run:

```powershell
$env:GOWORK='off'
go test ./internal/listingkit -run TestRefreshSheinDerivedStateReappliesCompletedSDSCanonicalMetadata -count=1
```

Expected: FAIL because the current revision path only writes `ai_style`; the title, identity attributes, and rendered images remain stale.

- [ ] **Step 3: Strengthen the no-result compatibility test**

Extend `TestRefreshSheinDerivedStateAppliesSDSStyleToEveryCanonicalVariant` so its canonical product starts with a title, a `sku` attribute, and product/variant images. Keep `SDSDesignResult` nil and assert after refresh:

```go
if product.Title != "Existing title" || product.Attributes["sku"].Value != "EXISTING" {
	t.Fatalf("unexpected no-result identity mutation: %+v", product)
}
if product.Images[0].URL != "https://cdn.example.com/existing.jpg" ||
	product.Variants[0].Images[0].URL != "https://cdn.example.com/existing-variant.jpg" {
	t.Fatalf("unexpected no-result image mutation: product=%+v variant=%+v", product.Images, product.Variants[0].Images)
}
```

- [ ] **Step 4: Run the focused regression set**

Run:

```powershell
$env:GOWORK='off'
go test ./internal/listingkit -run 'TestRefreshSheinDerivedState(ReappliesCompletedSDSCanonicalMetadata|AppliesSDSStyleToEveryCanonicalVariant)' -count=1
```

Expected: the existing style-only test passes and the new completed-result test fails before Task 2.

### Task 2: Align Revision Refresh with the Existing SDS Adapter

**Files:**

- Modify: `internal/listingkit/service_revision_recompute.go`
- Modify: `internal/listingkit/service_revision_test.go`

**Interfaces:**

- Consumes: `applySDSSyncMetadataToCanonical(*canonical.Product, *SDSSyncSummary, *SDSSyncOptions) bool`.
- Preserves: `sdspod.ApplyCanonical` style-only fallback when no completed SDS result is present.

- [ ] **Step 1: Replace the style-only block with explicit branch selection**

Replace the existing direct `sdspod.ApplyCanonical` block with:

```go
if task.Request != nil && task.Request.Options != nil {
	sdsOptions := task.Request.Options.SDS
	if task.Result.SDSDesignResult != nil {
		applySDSSyncMetadataToCanonical(
			task.Result.CanonicalProduct,
			task.Result.SDSDesignResult,
			sdsOptions,
		)
	} else {
		sdspod.ApplyCanonical(task.Result.CanonicalProduct, sdspod.CanonicalMetadata{
			StyleName: studioStyleName(sdsOptions),
		})
	}
}
```

Keep this block before `buildSheinPublishRequestForTask`. Do not change imports beyond what this branch requires.

- [ ] **Step 2: Run focused tests to verify GREEN**

Run:

```powershell
gofmt -w internal/listingkit/service_revision_recompute.go internal/listingkit/service_revision_test.go
$env:GOWORK='off'
go test ./internal/listingkit -run 'TestRefreshSheinDerivedState(ReappliesCompletedSDSCanonicalMetadata|AppliesSDSStyleToEveryCanonicalVariant)' -count=1
```

Expected: PASS. The completed-result test sees title, identity, product images, per-variant images, and style refreshed; the no-result test sees only the legacy style update.

- [ ] **Step 3: Run affected-package verification**

Run:

```powershell
$env:GOWORK='off'
go test ./internal/product/sourcing/sdspod -count=1
go test ./internal/listingkit -run 'TestApplyTaskRevision|TestRefreshSheinDerivedState|TestApplySDSSyncMetadataToCanonical' -count=1
go test ./internal/listingkit/... -count=1
go vet ./internal/listingkit/... ./internal/product/sourcing/sdspod
git diff --check
```

Expected: all commands exit 0. `go.work.sum` remains unchanged.

- [ ] **Step 4: Commit the implementation**

```powershell
git add internal/listingkit/service_revision_recompute.go internal/listingkit/service_revision_test.go
git commit -m "refactor: align sds revision canonical metadata"
```

## Final Acceptance Checklist

- [ ] Completed SDS revision refresh uses the existing root metadata adapter.
- [ ] Revision refresh preserves the style-only fallback when `SDSDesignResult` is nil.
- [ ] `sdspod` remains independent of ListingKit DTOs and runtime concerns.
- [ ] Category, attribute, review, persistence, and remote operation ordering are unchanged.
- [ ] Focused tests, ListingKit subpackage tests, and affected `go vet` checks pass.
- [ ] `go.work.sum` is unchanged.
