# SHEIN Pricing and Promotion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Require complete, same-currency source prices for SHEIN promotion calculations and exclude every incomplete candidate without fallback.

**Architecture:** Keep price selection in `internal/shein/activity`. Build an internal per-SKU pricing input (`SKU`, `RetailPrice`, `CostPrice`, `Currency`) from synchronized snapshots, then let each mode consume only its required field.

**Tech Stack:** Go, SHEIN marketing DTOs, Testify, existing activity registration tests.

## Global Constraints

- No retail price may become cost in `PROFIT` or `BREAKEVEN`.
- `DISCOUNT` requires an available target-currency retail price; it has no fallback.
- Never silently mix currencies.

---

### Task 1: Build direct SKU price inputs

**Files:**
- Modify: `internal/shein/activity/price_calculator.go`
- Test: `internal/shein/activity/price_calculator_test.go`

- [x] **Step 1: Write failing tests**

Cover a retail-only discount SKU and a multi-mode request where one SKU lacks
direct cost. `DISCOUNT` keeps both retail-priced SKUs; `PROFIT` and
`BREAKEVEN` keep only the SKU with both direct prices.

- [x] **Step 2: Verify red**

Run: `go test ./internal/shein/activity -run 'Test(BuildCalculateRequestForPromotionProductsRequiresDirectSKUPrices|PromotionGoodsFromProductSnapshotsKeepsRetailOnlySKUForDiscountPricing)'`

Expected: FAIL because retail-only SKUs are discarded at product level and no
direct SKU price input exists.

- [x] **Step 3: Implement minimal selector**

```go
type promotionSKUPriceInput struct {
    SKU, Currency string
    RetailPrice, CostPrice float64
}
func promotionSKUPriceInputs(products []marketing.SkcInfo, currency string) map[string]map[string]promotionSKUPriceInput
```

Read retail price only from enabled, matching-currency SKU site prices and cost
only from matching-currency `SkuCostPriceInfoList` entries. Do not write either
value into `marketing.PromotionSkuInfo` supply fields.

- [x] **Step 4: Verify green and commit**

Run: `go test ./internal/shein/activity -run 'Test(BuildCalculateRequestForPromotionProductsRequiresDirectSKUPrices|PromotionGoodsFromProductSnapshotsKeepsRetailOnlySKUForDiscountPricing)'`

Commit together after Task 3.

### Task 2: Enforce mode-specific direct-input policy

**Files:**
- Modify: `internal/shein/activity/promotion_create_products.go`
- Test: `internal/shein/activity/registration_direct_products_test.go`

- [x] **Step 1: Write failing integration tests**

Add `PROFIT`, `BREAKEVEN`, and `DISCOUNT` registration cases with a SKU that
lacks a required direct input; assert that the SKU is absent from
`calculateSupplyPrice`.

- [x] **Step 2: Verify red**

Run: `go test ./internal/shein/activity -run 'TestRegisterPromotionProducts.*(Profit|Breakeven|Discount)'`

Expected: FAIL because current enrichment substitutes product, other-SKU, or other-currency prices.

- [x] **Step 3: Implement minimal mode guard**

Build `promotionSKUPriceInput` from matching `SkuPriceInfoList` and
`SkuCostPriceInfoList` entries in the requested currency. `DISCOUNT` requires
retail price; `PROFIT` and `BREAKEVEN` require both retail and cost. Filter each
SKU before creating `marketing.SkuPriceInfo`; do not mutate
`marketing.PromotionSkuInfo` supply fields.

- [x] **Step 4: Verify green and commit**

Run: `go test ./internal/shein/activity -run 'TestRegisterPromotionProducts.*(Profit|Breakeven|Discount)'`

Commit: `git commit -m "fix: enforce shein promotion cost fallback policy"`

### Task 3: Verify multi-SKU semantics and closeout

**Files:**
- Test: `internal/shein/activity/registration_direct_products_test.go`
- Modify: `internal/shein/activity/time_limited.go`

- [x] **Step 1: Write failing multi-SKU test**

Create multiple SKUs with different customer prices and costs; assert
`DISCOUNT` keeps direct retail-priced SKUs, while `PROFIT` and `BREAKEVEN`
exclude only the SKU missing same-currency cost.

- [x] **Step 2: Verify red**

Run: `go test ./internal/shein/activity -run TestBuildCalculateRequestForPromotionProductsRequiresDirectSKUPrices`

- [x] **Step 3: Implement only the per-SKU correction required by the test**

Do not add a shared pricing service. Keep selection inside the activity request
builder and make the creation request iterate only SKUs that were included in
the calculation request.

- [x] **Step 4: Run closure suite and commit**

Run:

```powershell
go test ./internal/shein/activity
go test ./internal/listingkit ./internal/publishing/shein
git diff --check
```

Commit: `git commit -m "test: validate shein promotion pricing semantics"`
