# SHEIN Final Resolution Cache Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a successful SHEIN publish remember complete per-value sale-attribute assignments and the complete final size chart so a same-baseline republish can reuse both safely.

**Architecture:** Reconcile reusable sale-attribute state from the final publish package before saving it, and reject any cached resolution that does not cover every current source value. Keep the existing dedicated size-attribute cache, but add stable product-identity fallback lookup for legacy entries and validate cached rows against the reconciled current sale-value IDs instead of requiring the partially regenerated size-chart shape to match first.

**Tech Stack:** Go 1.26, GORM-backed resolution cache, standard `testing`, `gofmt`, `gopls`, `go test`, `go vet`.

## Global Constraints

- Do not cache the whole SHEIN submission package.
- Do not infer or invent SHEIN `attribute_value_id` values.
- Do not reuse cache entries across stores, categories, product identities, or source dimensions.
- Preserve existing fallback behavior when a cache entry cannot be proven applicable.
- Preserve the user's existing `go.work.sum` change.

---

### Task 1: Reconcile and validate final sale-attribute assignments

**Files:**
- Create: `internal/publishing/shein/sale_attribute_cache_reconcile.go`
- Create: `internal/publishing/shein/sale_attribute_cache_reconcile_test.go`
- Modify: `internal/publishing/shein/resolver_cache.go`
- Test: `internal/publishing/shein/resolver_cache_test.go`

**Interfaces:**
- Consumes: `*Package`, `*SaleAttributeResolution`, `SourceVariantDimension`, final SKC/SKU sale attributes.
- Produces: `ReconcilePublishedSaleAttributeResolution(pkg *Package, resolution *SaleAttributeResolution) *SaleAttributeResolution` and `SaleAttributeResolutionApplicable(pkg *Package, resolution *SaleAttributeResolution) (bool, string)`.

- [ ] **Step 1: Write a failing reconciliation test**

Create a package-level test with source sizes `S` and `5XL`, an input resolution containing only `S`, and a final package SKU patch containing `5XL -> attribute_id 87, attribute_value_id 1430561`. Assert that reconciliation returns a clone, preserves `S`, adds normalized key `5xl`, and leaves the original resolution unchanged.

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test ./internal/publishing/shein -run TestReconcilePublishedSaleAttributeResolutionAddsFinalSKUAssignments -count=1`

Expected: FAIL because `ReconcilePublishedSaleAttributeResolution` does not exist.

- [ ] **Step 3: Implement minimal final-package reconciliation**

Implement the exported helper by cloning the resolution, walking final SKC and SKU patch data, and merging only entries with positive attribute and value IDs. Normalize source values with the package's existing sale-value normalization helper; never derive IDs from display text.

- [ ] **Step 4: Verify GREEN**

Run: `gofmt -w internal/publishing/shein/sale_attribute_cache_reconcile.go internal/publishing/shein/sale_attribute_cache_reconcile_test.go`

Run: `go test ./internal/publishing/shein -run TestReconcilePublishedSaleAttributeResolutionAddsFinalSKUAssignments -count=1`

Expected: PASS.

- [ ] **Step 5: Write failing completeness tests**

Add table tests asserting that `SaleAttributeResolutionApplicable`:

```go
tests := []struct {
    name       string
    sizes      []string
    assignments map[string]ResolvedSaleAttribute
    wantOK     bool
}{
    {name: "all current sizes mapped", sizes: []string{"S", "5XL"}, assignments: completeAssignments, wantOK: true},
    {name: "legacy cache misses 5XL", sizes: []string{"S", "5XL"}, assignments: onlySAssignment, wantOK: false},
}
```

Assert the rejected reason names normalized missing value `5xl`.

- [ ] **Step 6: Run completeness tests and verify RED**

Run: `go test ./internal/publishing/shein -run TestSaleAttributeResolutionApplicable -count=1`

Expected: FAIL because the applicability helper does not exist.

- [ ] **Step 7: Implement completeness validation and wire cache read/write**

Validate each value in the selected primary and secondary source dimensions against the corresponding SKC/SKU value-assignment map. Update `cachedSaleAttributeResolver.Resolve` to delete/reject both memory and persistent entries that fail this validation, set `CacheRejectedReason`, and fall back to `inner.Resolve`. Update `RememberSaleAttributeResolution` to reconcile the final package first and save only an applicable clone.

- [ ] **Step 8: Add resolver regression tests**

Extend `resolver_cache_test.go` with:

- a remembered `S..5XL` resolution whose final package supplies the missing manual `5XL` mapping, then assert the next resolve returns eight assignments without calling the inner resolver;
- an incomplete legacy memory cache, then assert the inner resolver is called and the returned review note identifies the rejected missing value;
- the same legacy case through the persistent cache store.

- [ ] **Step 9: Run package verification**

Run: `gofmt -w internal/publishing/shein/resolver_cache.go internal/publishing/shein/resolver_cache_test.go`

Run: `go test ./internal/publishing/shein -run 'Test(ReconcilePublishedSaleAttributeResolution|SaleAttributeResolutionApplicable|CachedSaleAttributeResolver)' -count=1`

Expected: PASS.

### Task 2: Remember reconciled sale attributes at the ListingKit publish boundary

**Files:**
- Modify: `internal/listingkit/shein_resolution_cache.go`
- Modify: `internal/listingkit/service_submit_lifecycle_test.go`

**Interfaces:**
- Consumes: Task result after successful remote publish and final draft reconciliation.
- Produces: a call to `RememberSaleAttributeResolution` with the publish-confirmed reconciled resolution.

- [ ] **Step 1: Write a failing successful-publish regression test**

Extend `TestSubmitTaskRemembersSheinResolutionCacheAfterPublishSuccess` or add a focused sibling test. Build a ready task with eight source sizes and a final `5XL -> 1430561` SKU sale attribute while the pre-publish resolution omits `5xl`. Submit successfully and decode the stored `sale_attribute` entry. Assert `SKUValueAssignments["5xl"].AttributeValueID` equals `1430561`.

- [ ] **Step 2: Run the test and verify RED**

Run: `go test ./internal/listingkit -run TestSubmitTaskRemembersCompleteFinalSaleAttributeResolutionAfterPublishSuccess -count=1`

Expected: FAIL because the stored entry omits `5xl`.

- [ ] **Step 3: Pass a reconciled clone to the cache boundary**

In `rememberSheinSaleAttributeResolution`, call `sheinpub.ReconcilePublishedSaleAttributeResolution(task.Result.Shein, task.Result.Shein.SaleAttributeResolution)` and pass the returned clone to the cache. Do not mutate the live result merely to persist cache metadata.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `gofmt -w internal/listingkit/shein_resolution_cache.go internal/listingkit/service_submit_lifecycle_test.go`

Run: `go test ./internal/listingkit -run 'TestSubmitTaskRemembers(SheinResolutionCache|CompleteFinalSaleAttributeResolution)AfterPublishSuccess' -count=1`

Expected: PASS.

### Task 3: Restore complete final size charts by stable identity

**Files:**
- Modify: `internal/publishing/shein/size_attribute_cache_reconcile.go`
- Modify: `internal/publishing/shein/size_attribute_cache_reconcile_test.go`
- Modify: `internal/listingkit/size_attribute_cache_service.go`
- Create: `internal/listingkit/size_attribute_cache_service_test.go`

**Interfaces:**
- Consumes: existing `SizeAttributeReview`, current package product identity, category, and reconciled sale-value assignments.
- Produces: `SizeAttributeReviewApplicableToSaleResolution(pkg *Package, review *SizeAttributeReview) bool`; exact-key lookup followed by manual product-identity fallback through `ResolutionCacheSourceIdentityGetter`.

- [ ] **Step 1: Write a failing applicability test for a complete cached chart**

Create a current package whose regenerated draft contains only seven sizes and three measurement fields, whose reconciled sale resolution contains all eight size value IDs, and a cached review containing eight sizes and five fields. Assert the new applicability helper accepts the cached review and rejects a row referencing an unknown sale value ID.

- [ ] **Step 2: Run the test and verify RED**

Run: `go test ./internal/publishing/shein -run TestSizeAttributeReviewApplicableToSaleResolution -count=1`

Expected: FAIL because the helper does not exist.

- [ ] **Step 3: Implement sale-resolution-based validation**

Build the allowed `(sale_attribute_id, sale_attribute_value_id)` set from current SKC/SKU value assignments. Require every cached row to have positive IDs and reference that set. Require the cached review to cover every current sale-value ID for the size dimension. Do not compare against the partially regenerated draft row count or alias set.

- [ ] **Step 4: Write a failing ListingKit fallback lookup test**

Use a fake store implementing `ResolutionCacheSourceIdentityGetter`. Configure exact-key lookup to miss and product-identity lookup to return the first publish's eight-size/five-field review. Assert `loadSheinSizeAttributeCache` returns the review and `applyDefaultSheinSizeAttributes` installs all 40 rows.

- [ ] **Step 5: Run the fallback test and verify RED**

Run: `go test ./internal/listingkit -run TestLoadSheinSizeAttributeCacheFallsBackToPublishedProductIdentity -count=1`

Expected: FAIL because size-cache loading only performs exact-key lookup.

- [ ] **Step 6: Implement stable-identity fallback and legacy upgrade**

In `loadSheinSizeAttributeCache`, retain exact-key lookup first. On miss or inapplicable exact entry, use `ResolutionCacheSourceIdentityGetter.GetManualResolutionCacheByProductIdentity` with kind `size_attribute`, current store, category, and `StablePricingPackageIdentity(pkg)`. Decode and validate with `SizeAttributeReviewApplicableToSaleResolution`. When a legacy fallback succeeds, save it under the current computed key so later reads use the direct path.

- [ ] **Step 7: Preserve final rows during reconciliation**

Change `ReconcileSizeAttributeCacheReview` so a validated complete published review remains the source of row shape and measurements; remap only sale-value IDs where the current reconciled mapping provides an unambiguous normalized source-value match. Do not truncate the review to the current incomplete draft list.

- [ ] **Step 8: Run focused size-cache tests**

Run: `gofmt -w internal/publishing/shein/size_attribute_cache_reconcile.go internal/publishing/shein/size_attribute_cache_reconcile_test.go internal/listingkit/size_attribute_cache_service.go internal/listingkit/size_attribute_cache_service_test.go`

Run: `go test ./internal/publishing/shein -run 'Test(SizeAttributeCache|ReconcileSizeAttributeCache|SizeAttributeReviewApplicable)' -count=1`

Run: `go test ./internal/listingkit -run 'Test(LoadSheinSizeAttributeCache|ApplyDefaultSheinSizeAttributes|SubmitTaskRemembers)' -count=1`

Expected: PASS.

### Task 4: End-to-end regression and broad verification

**Files:**
- Modify: `internal/listingkit/service_submit_lifecycle_test.go`
- Modify: `internal/listingkit/size_attribute_cache_service_test.go`

**Interfaces:**
- Consumes: completed Tasks 1-3.
- Produces: regression proof for first publish followed by same-baseline republish.

- [ ] **Step 1: Add a two-task regression test**

Exercise one shared cache store with:

1. first publish: eight sizes, final manual `5XL -> 1430561`, and 40 size-chart rows;
2. second generation: same store/category/product identity, initially regenerated as seven sizes and 21 rows;
3. assertions: sale cache restores eight assignments, size cache restores 40 rows, and no sale-attribute review error remains.

- [ ] **Step 2: Verify the regression test fails without the combined behavior and passes with it**

Run: `go test ./internal/listingkit -run TestSheinRepublishRestoresCompleteSaleAttributesAndSizeChart -count=1`

Expected after implementation: PASS. Confirm the test failed for missing `5xl` or incomplete size rows before the production changes were present in its TDD cycle.

- [ ] **Step 3: Run diagnostics and package tests**

Run: `gopls check internal/publishing/shein/sale_attribute_cache_reconcile.go internal/publishing/shein/resolver_cache.go internal/publishing/shein/size_attribute_cache_reconcile.go internal/listingkit/shein_resolution_cache.go internal/listingkit/size_attribute_cache_service.go`

Run: `go test ./internal/publishing/shein ./internal/listingkit -count=1`

Run: `go vet ./internal/publishing/shein ./internal/listingkit`

Expected: all commands exit 0 with no diagnostics.

- [ ] **Step 4: Run repository verification proportional to the shared-cache change**

Run: `go test ./... -count=1`

Run: `go vet ./...`

Expected: all commands exit 0. If an unrelated pre-existing failure occurs, record the exact package and output rather than masking it.

- [ ] **Step 5: Review the final diff**

Run: `git diff --check`

Run: `git status --short`

Confirm only the intended Go/test/plan files plus the user's pre-existing `go.work.sum` change are present, and do not stage `go.work.sum`.
