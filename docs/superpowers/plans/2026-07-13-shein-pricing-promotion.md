# SHEIN Pricing and Promotion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Require complete, same-currency source prices for SHEIN promotion calculations and exclude every incomplete candidate without fallback.

**Architecture:** Keep price selection in `internal/shein/activity`. Represent the selected amount with its source kind so cost-required modes can reject retail-derived values, and preserve per-SKU price/cost selection.

**Tech Stack:** Go, SHEIN marketing DTOs, Testify, existing activity registration tests.

## Global Constraints

- No retail price may become cost in `PROFIT` or `BREAKEVEN`.
- `DISCOUNT` requires an available target-currency retail price; it has no fallback.
- Never silently mix currencies.

---

### Task 1: Make supply-price provenance testable

**Files:**
- Modify: `internal/shein/activity/price_calculator.go`
- Test: `internal/shein/activity/price_calculator_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestPromotionSKUCostDoesNotUseRetailFallback(t *testing.T) {
    got := promotionSKUCost(marketing.PromotionSkuInfo{}, 0)
    require.False(t, got.Available)
}
```

Add cases asserting `USSupplyPrice`, `SupplyPrice`, `SupplyPriceInfo.SupplyPrice`, then SKC supply price are selected in that order.

- [ ] **Step 2: Verify red**

Run: `go test ./internal/shein/activity -run TestPromotionSKUCostDoesNotUseRetailFallback`

Expected: FAIL because no provenance-aware cost selector exists.

- [ ] **Step 3: Implement minimal selector**

```go
type promotionPrice struct { Value float64; Available bool }
func promotionSKUCost(sku marketing.PromotionSkuInfo, skcSupply float64) promotionPrice {
    // Select only supply fields, then skcSupply; otherwise unavailable.
}
```

- [ ] **Step 4: Verify green and commit**

Run: `go test ./internal/shein/activity -run 'TestPromotionSKUCost|TestCalculatePrice'`

Commit: `git commit -m "fix: track shein promotion supply price source"`

### Task 2: Enforce mode-specific fallback policy

**Files:**
- Modify: `internal/shein/activity/promotion_create_products.go`
- Test: `internal/shein/activity/registration_direct_products_test.go`

- [ ] **Step 1: Write failing integration tests**

Add `PROFIT`, `BREAKEVEN`, and `DISCOUNT` registration cases with missing required values; assert no candidate reaches `calculateSupplyPrice` and the result contains a stable missing-price reason.

- [ ] **Step 2: Verify red**

Run: `go test ./internal/shein/activity -run 'TestRegisterPromotionProducts.*(Profit|Breakeven|Discount)'`

Expected: FAIL because current enrichment substitutes product, other-SKU, or other-currency prices.

- [ ] **Step 3: Implement minimal mode guard**

Pass `PriceMode` and requested currency into snapshot enrichment. Retain only required direct SKU values; add a filter reason for every missing or incompatible value. Do not substitute any product, other-SKU, or other-currency value.

- [ ] **Step 4: Verify green and commit**

Run: `go test ./internal/shein/activity -run 'TestRegisterPromotionProducts.*(Profit|Breakeven|Discount)'`

Commit: `git commit -m "fix: enforce shein promotion cost fallback policy"`

### Task 3: Verify multi-SKU semantics and closeout

**Files:**
- Test: `internal/shein/activity/registration_direct_products_test.go`
- Modify: `docs/refactoring/next-phase-plan.md`

- [ ] **Step 1: Write failing multi-SKU test**

Create two SKUs with different customer prices and costs; assert `DISCOUNT` returns two independently discounted values, while `PROFIT` and `BREAKEVEN` use each SKU cost and exclude only the SKU missing true cost.

- [ ] **Step 2: Verify red**

Run: `go test ./internal/shein/activity -run TestRegisterPromotionProductsUsesIndependentMultiSKUCostAndPrice`

- [ ] **Step 3: Implement only the per-SKU correction required by the test**

Do not add a shared pricing service. Keep selection inside the activity request builder.

- [ ] **Step 4: Run closure suite and commit**

Run:

```powershell
go test ./internal/shein/activity
go test ./internal/listingkit ./internal/publishing/shein
git diff --check
```

Commit: `git commit -m "test: validate shein promotion pricing semantics"`
