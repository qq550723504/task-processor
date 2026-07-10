# SHEIN Time-Limited Multi-SKU Discount Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject a SHEIN time-limited activity candidate when any SKU activity price is equal to or greater than 95% of that SKU's original price.

**Architecture:** Extend the existing request builder's SKU loop so it validates the already-resolved SKU original and activity prices before appending the SKU. Share one strict discount-rate constant between SKU-level and existing SKC-level checks, and preserve the current candidate filter-reason flow.

**Tech Stack:** Go, standard library testing, existing `internal/shein/activity` test stubs and request builder.

## Global Constraints

- Every SKU must satisfy `activity_price < original_price * 0.95`.
- Equality at 95% is invalid.
- One invalid SKU excludes the entire SKC; never submit a partial SKU set.
- Keep the existing SKC-level check as defense in depth.
- Do not clamp or otherwise change configured prices.
- Do not change the create-activity endpoint or request schema.

---

### Task 1: Validate Every Resolved SKU Price

**Files:**
- Modify: `internal/shein/activity/time_limited.go:277-367`
- Test: `internal/shein/activity/registration_direct_products_test.go`

**Interfaces:**
- Consumes: `(*activityRegistrationServiceImpl).buildCreateActivityRequest(config TimeLimitedDiscountConfig, goods []marketing.PromotionGoodsData, calcReq *marketing.CalculateSupplyPriceRequest, calcResp *marketing.CalculateSupplyPriceResponse) (*marketing.CreateActivityRequest, []string, map[string]string)`
- Produces: unchanged request-builder signature; candidates with any invalid SKU are omitted and receive an entry in `filterReasonBySKC`.

- [ ] **Step 1: Add the failing boundary tests**

Add a table-driven test to `registration_direct_products_test.go`:

```go
func TestBuildCreateActivityRequestValidatesEverySKUDiscount(t *testing.T) {
	tests := []struct {
		name            string
		secondSKUPrice  float64
		wantIncluded    bool
		wantReasonPrice string
	}{
		{name: "equal to 95 percent", secondSKUPrice: 190, wantIncluded: false, wantReasonPrice: "190.00"},
		{name: "above 95 percent", secondSKUPrice: 191, wantIncluded: false, wantReasonPrice: "191.00"},
		{name: "strictly below 95 percent", secondSKUPrice: 189.99, wantIncluded: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &activityRegistrationServiceImpl{logger: logrus.NewEntry(logrus.New())}
			goods := []marketing.PromotionGoodsData{{
				Skc:              "sg-multi-sku-discount",
				InventoryNum:     100,
				USSupplyPrice:    100,
				MaxUSSupplyPrice: 100,
				SkuInfoList: []marketing.PromotionSkuInfo{
					{Sku: "sku-small", USSupplyPrice: promotionTestFloat64Ptr(100)},
					{Sku: "sku-large", USSupplyPrice: promotionTestFloat64Ptr(200)},
				},
			}}
			calcResp := &marketing.CalculateSupplyPriceResponse{Info: []marketing.SkcCalculationResult{{
				SkcName: "sg-multi-sku-discount",
				SkuInfoList: []marketing.SkuCalculationInfo{
					{SkuCode: "sku-small", PriceInfo: marketing.PriceInfo{ProductAmount: 100, PromotionAmount: 20}},
					{SkuCode: "sku-large", PriceInfo: marketing.PriceInfo{ProductAmount: 200, PromotionAmount: 200 - tt.secondSKUPrice}},
				},
			}}}

			req, _, reasons := service.buildCreateActivityRequest(
				TimeLimitedDiscountConfig{EffectiveCenterList: []int{2}},
				goods,
				nil,
				calcResp,
			)

			if got := len(req.AddCostAndStockInfoList); (got == 1) != tt.wantIncluded {
				t.Fatalf("created goods count = %d, want included %t", got, tt.wantIncluded)
			}
			if tt.wantIncluded {
				if reason := reasons["sg-multi-sku-discount"]; reason != "" {
					t.Fatalf("filter reason = %q, want empty", reason)
				}
				return
			}
			reason := reasons["sg-multi-sku-discount"]
			for _, want := range []string{"sku-large", tt.wantReasonPrice, "200.00", "95%"} {
				if !strings.Contains(reason, want) {
					t.Fatalf("filter reason = %q, want %q", reason, want)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
go test ./internal/shein/activity -run TestBuildCreateActivityRequestValidatesEverySKUDiscount -count=1
```

Expected: the `equal to 95 percent` and `above 95 percent` cases fail because the current builder includes the SKC and has no SKU-specific filter reason.

- [ ] **Step 3: Add the minimal per-SKU validation**

In `time_limited.go`, define a shared package constant:

```go
const maxTimeLimitedDiscountRate = 0.95
```

Inside the SKU loop, after resolving `skuCostPrice` and `skuActPrice` but before appending to `addSkuList`, add:

```go
if skuCostPrice > 0 && skuActPrice >= skuCostPrice*maxTimeLimitedDiscountRate {
	invalidSKUReason = fmt.Sprintf(
		"商品 %s 的 SKU %s 折扣不足(活动价 %.2f, 原价 %.2f, 要求低于原价95%%)",
		g.Skc,
		sku.Sku,
		skuActPrice,
		skuCostPrice,
	)
	break
}
```

Declare `invalidSKUReason := ""` immediately before the SKU loop. Immediately after the loop, exclude the whole SKC when it is set:

```go
if invalidSKUReason != "" {
	s.logger.Warn(invalidSKUReason)
	filterReasons = appendPromotionFilterReasonForSKC(filterReasons, filterReasonBySKC, g.Skc, invalidSKUReason)
	skippedByDiscount++
	continue
}
```

Replace the existing SKC-level local `maxDiscountRate := 0.95` and its uses with `maxTimeLimitedDiscountRate` so both checks enforce the same strict boundary.

- [ ] **Step 4: Format and verify GREEN**

Run:

```powershell
gofmt -w internal/shein/activity/time_limited.go internal/shein/activity/registration_direct_products_test.go
go test ./internal/shein/activity -run TestBuildCreateActivityRequestValidatesEverySKUDiscount -count=1
```

Expected: all three table cases pass.

- [ ] **Step 5: Run package diagnostics and regression tests**

Run:

```powershell
gopls check internal/shein/activity/time_limited.go internal/shein/activity/registration_direct_products_test.go
go test ./internal/shein/activity -count=1
go vet ./internal/shein/activity
git diff --check
```

Expected: all commands exit 0. On Windows, `git diff --check` may report existing LF-to-CRLF warnings but no whitespace errors.

- [ ] **Step 6: Commit the implementation**

```powershell
git add -- internal/shein/activity/time_limited.go internal/shein/activity/registration_direct_products_test.go
git commit -m "fix: validate shein time-limited sku discounts"
```
