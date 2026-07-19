# SHEIN Sale-Attribute Cache Repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist a successful SHEIN sale-attribute confirmation under its true source values and reuse it when the same SDS source is rendered with a volatile CE-suffixed variant SKU.

**Architecture:** Published-sale reconciliation will source primary SKC values from each SKC's SKU attributes, matching the existing secondary-SKU path. Sale-attribute cache keys will prefer `source_sds_sku` over a downstream `variant_sku`. The existing applicability gate remains the authority that prevents incomplete mappings from being stored or reused.

**Tech Stack:** Go, package `internal/publishing/shein`, standard `testing` package.

## Global Constraints

- Keep `SaleAttributeResolutionApplicable` unchanged as the safety gate.
- Do not alter SDS baseline-cache storage or database schema.
- Use `gofmt` and package-scoped Go tests.

---

### Task 1: Preserve primary source values during published-sale reconciliation

**Files:**
- Modify: `internal/publishing/shein/sale_attribute_cache_reconcile.go:20-26`
- Test: `internal/publishing/shein/sale_attribute_cache_reconcile_test.go`

**Interfaces:**
- Consumes: `Package.DraftPayload.SKCList[].SKUList[].Attributes` and `SKCRequestDraft.SaleAttribute`.
- Produces: `ReconcilePublishedSaleAttributeResolution(*Package, *SaleAttributeResolution) *SaleAttributeResolution` with `SKCValueAssignments[normalize(Color)]` populated.

- [x] **Step 1: Write the failing test**

```go
func TestReconcilePublishedSaleAttributeResolutionUsesSKUSourceValueForPrimaryAssignment(t *testing.T) {
	colorID := 447
	original := &SaleAttributeResolution{Status: "resolved", PrimaryAttributeID: 27, PrimarySourceDimension: "Color", SourceDimensions: []SourceVariantDimension{{Name: "Color", Values: []string{"white"}}}}
	pkg := &Package{DraftPayload: &RequestDraft{SKCList: []SKCRequestDraft{{SaleName: "product display title", SaleAttribute: &ResolvedSaleAttribute{AttributeID: 27, AttributeValueID: &colorID}, SKUList: []SKUDraft{{Attributes: map[string]string{"Color": "white"}}}}}}}
	got := ReconcilePublishedSaleAttributeResolution(pkg, original)
	if assignment := got.SKCValueAssignments["white"]; assignment.AttributeValueID == nil || *assignment.AttributeValueID != colorID { t.Fatalf("white assignment = %+v, want %d", assignment, colorID) }
	if ok, reason := SaleAttributeResolutionApplicable(got); !ok { t.Fatalf("reconciled resolution is not applicable: %s", reason) }
}
```

- [x] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/publishing/shein -run TestReconcilePublishedSaleAttributeResolutionUsesSKUSourceValueForPrimaryAssignment -count=1`

Expected: FAIL because the current implementation uses `SaleName` as the primary source value.

- [x] **Step 3: Write minimal implementation**

```go
for _, skc := range pkg.DraftPayload.SKCList {
	for _, sku := range skc.SKUList {
		mergePublishedSaleAssignment(result.SKCValueAssignments, sku.Attributes, result.PrimarySourceDimension, skc.SaleAttribute)
		// retain existing secondary-sale-attribute loop
	}
}
```

- [x] **Step 4: Run focused reconciliation tests**

Run: `go test ./internal/publishing/shein -run 'Test(ReconcilePublishedSaleAttributeResolution|SaleAttributeResolutionApplicable)' -count=1`

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/publishing/shein/sale_attribute_cache_reconcile.go internal/publishing/shein/sale_attribute_cache_reconcile_test.go
git commit -m "fix(shein): preserve source value in sale cache"
```

### Task 2: Normalize sale-cache identity when a source SDS SKU is present

**Files:**
- Modify: `internal/publishing/shein/cache_identity.go:47-77`
- Test: `internal/publishing/shein/resolver_cache_test.go:1300-1355`

**Interfaces:**
- Consumes: `canonical.Variant.Attributes["source_sds_sku"]` and optional `canonical.Variant.Attributes["variant_sku"]`.
- Produces: stable input for `saleAttributeResolverCacheKey` across equivalent SDS source variants.

- [x] **Step 1: Write the failing test**

Extend `TestSaleAttributeResolverCacheKeyUsesCanonicalSDSIdentifiers` so both variants retain `source_sds_sku: MG8014062001`, the first has `variant_sku: MG8014062001`, and the second has `variant_sku: MG8014062001-CE954D2D`; keep the existing equal-key assertion.

- [x] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/publishing/shein -run TestSaleAttributeResolverCacheKeyUsesCanonicalSDSIdentifiers -count=1`

Expected: FAIL because the raw CE-suffixed `variant_sku` is currently included in the identifier set.

- [x] **Step 3: Write minimal implementation**

```go
for _, variant := range canonical.Variants {
	source := variant.Attributes["source_sds_sku"].Value
	if source != "" {
		values = append(values, source)
		continue
	}
	values = append(values, variant.Attributes["variant_sku"].Value)
}
```

Preserve product-level identifiers and the fallback behavior when no source SDS SKU exists.

- [x] **Step 4: Run focused cache tests**

Run: `go test ./internal/publishing/shein -run 'Test(CachedSaleAttributeResolver|SaleAttributeResolverCacheKey)' -count=1`

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/publishing/shein/cache_identity.go internal/publishing/shein/resolver_cache_test.go
git commit -m "fix(shein): normalize sale cache SDS identity"
```

### Task 3: Verify integrated behavior

**Files:**
- Verify: `internal/publishing/shein/sale_attribute_cache_reconcile.go`
- Verify: `internal/publishing/shein/cache_identity.go`

**Interfaces:**
- Consumes the published reconciliation and normalized cache identity from Tasks 1-2.
- Produces verified package behavior without schema or baseline-cache changes.

- [x] **Step 1: Format changed Go files**

Run: `gofmt -w internal/publishing/shein/sale_attribute_cache_reconcile.go internal/publishing/shein/sale_attribute_cache_reconcile_test.go internal/publishing/shein/cache_identity.go internal/publishing/shein/resolver_cache_test.go`

- [x] **Step 2: Run package verification**

Run: `go test ./internal/publishing/shein ./internal/listingkit`

Expected: PASS.

- [x] **Step 3: Review the diff**

Run:

```bash
git diff --check
git status --short
git log --oneline -3
```

Expected: only the intended source, test, plan, and design changes are present.
