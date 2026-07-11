# SHEIN Promotion Multi-SKU Minimum Discount Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve SKU-specific prices and make SHEIN promotion enrollment submit the smallest discount supported by every SKU.

**Architecture:** Keep SKU prices and SKU costs paired by SKU code from candidate refresh through `marketing.SkcInfo`. Add one promotion pricing helper that calculates each SKU's activity price and returns the minimum supported integer `drop_rate`; use it in candidate-backed and attributes-backed promotion paths while leaving time-limited SKU submission unchanged.

**Tech Stack:** Go, standard library, Testify, existing SHEIN activity and ListingKit enrollment test stubs.

## Global Constraints

- `TIME_LIMITED` keeps SKU-specific original prices and SKU-specific activity prices.
- `PROMOTION` submits the smallest discount supported by all valid SKUs.
- Do not average SKU prices.
- A partial SKU price/cost set excludes the entire SKC.
- For `BOTH`, regular uses `minimum - 1` and limited uses `minimum`.
- Keep scalar fallback only when no SKU-level data exists.
- Do not change SHEIN API endpoints or payload schemas.

---

### Task 1: Preserve SKU Prices During Candidate Refresh

**Files:**
- Modify: `internal/listingkit/sheinsync/enrollment_service_candidates.go:168-195`
- Test: `internal/listingkit/sheinsync/enrollment_service_test.go:760-840`

**Interfaces:**
- Consumes: `refreshSheinEnrollmentPriceSnapshot(existing string, product SheinSyncedProductRecord) string`
- Produces: a refreshed JSON snapshot whose top-level `sale_price` may use `product.SupplyPrice` while existing `sku_prices[*].sale_price` values remain unchanged.

- [ ] **Step 1: Extend the existing supply-price refresh test**

Change the candidate and synced product fixtures in `TestExecuteSheinActivityEnrollmentUsesSupplyPriceAsOriginalPriceWithManualSDSCost` to include distinct SKU prices:

```go
PriceSnapshot: `{"sale_price":40,"currency":"USD","sub_site":"shein-us","sku_prices":[{"sku_code":"sku-small","sale_price":29.9,"currency":"USD"},{"sku_code":"sku-large","sale_price":34.9,"currency":"USD"}]}`,
```

After execution, unmarshal the adapter candidate snapshot and assert:

```go
var refreshed promotionCandidatePriceSnapshot
require.NoError(t, json.Unmarshal([]byte(adapter.calls[0].Candidates[0].PriceSnapshot), &refreshed))
require.Equal(t, 53.95, refreshed.SalePrice)
require.Equal(t, []promotionCandidateSKUPriceSnapshot{
	{SKUCode: "sku-small", SalePrice: 29.9, Currency: "USD"},
	{SKUCode: "sku-large", SalePrice: 34.9, Currency: "USD"},
}, refreshed.SKUPrices)
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
go test ./internal/listingkit/sheinsync -run TestExecuteSheinActivityEnrollmentUsesSupplyPriceAsOriginalPriceWithManualSDSCost -count=1
```

Expected: FAIL because both SKU prices are overwritten with `53.95`.

- [ ] **Step 3: Preserve SKU entries**

In `refreshSheinEnrollmentPriceSnapshot`, keep the top-level assignment and remove the loop that rewrites each SKU entry:

```go
payload["sale_price"] = *product.SupplyPrice
```

Do not mutate `payload["sku_prices"]`.

- [ ] **Step 4: Format, verify GREEN, and commit**

```powershell
gofmt -w internal/listingkit/sheinsync/enrollment_service_candidates.go internal/listingkit/sheinsync/enrollment_service_test.go
go test ./internal/listingkit/sheinsync -run TestExecuteSheinActivityEnrollmentUsesSupplyPriceAsOriginalPriceWithManualSDSCost -count=1
git add -- internal/listingkit/sheinsync/enrollment_service_candidates.go internal/listingkit/sheinsync/enrollment_service_test.go
git commit -m "fix: preserve shein enrollment sku prices"
```

Expected: focused test passes and the commit contains only the refresh behavior and its regression test.

---

### Task 2: Allow Distinct-Price SKUs Through Promotion Enrollment

**Files:**
- Modify: `internal/listingkit/sheinsync/activity_adapter.go:100-315`
- Modify: `internal/shein/activity/registration.go:185-258`
- Test: `internal/listingkit/sheinsync/enrollment_service_test.go:1250-1310`
- Test: `internal/shein/activity/registration_direct_products_test.go:150-225`

**Interfaces:**
- Consumes: `buildPromotionCandidateProduct(candidate SheinActivityEnrollmentCandidate) (marketing.SkcInfo, string, bool)`
- Produces: promotion bridge input containing distinct `SkuPriceInfoList` and matching `SkuCostPriceInfoList` without rejecting the product solely because prices differ.

- [ ] **Step 1: Replace adapter rejection test with a pass-through test**

Rename `TestSheinActivityAdapterPromotionRejectsMultiSKUDifferentPrices` to `TestSheinActivityAdapterPromotionPassesMultiSKUPricesAndCosts` and provide costs:

```go
SKUCostPriceInfoList: []SheinSKUCostPrice{
	{SKUCode: "sku-small", CostPrice: 12.5, Currency: "USD"},
	{SKUCode: "sku-large", CostPrice: 20.5, Currency: "USD"},
},
```

Make the bridge return a successful SaveConfig result, then assert one call and the exact SKU lists:

```go
require.Len(t, bridge.calls, 1)
require.Len(t, bridge.calls[0].Products, 1)
product := bridge.calls[0].Products[0]
require.Len(t, product.SkuPriceInfoList, 2)
require.Len(t, product.SkuCostPriceInfoList, 2)
require.Equal(t, "sku-small", product.SkuPriceInfoList[0].SkuCode)
require.Equal(t, "sku-small", product.SkuCostPriceInfoList[0].SkuCode)
```

- [ ] **Step 2: Replace activity-service rejection test**

Rename `TestRegisterPromotionProductsRejectsPromotionMultiSkuDifferentPrices` to `TestRegisterPromotionProductsAcceptsPromotionMultiSkuDifferentPrices`. Add matching `SkuCostPriceInfoList`, use `BREAKEVEN`, and assert that `SaveConfig` is called instead of expecting the old error.

- [ ] **Step 3: Run both tests and verify RED**

```powershell
go test ./internal/listingkit/sheinsync -run TestSheinActivityAdapterPromotionPassesMultiSKUPricesAndCosts -count=1
go test ./internal/shein/activity -run TestRegisterPromotionProductsAcceptsPromotionMultiSkuDifferentPrices -count=1
```

Expected: both fail on the existing multi-SKU rejection guards.

- [ ] **Step 4: Remove rejection-only plumbing**

Simplify the adapter helpers by removing `rejectMultiSKUDifferentPrices` parameters and the conditional rejection:

```go
func buildPromotionCandidateProduct(candidate SheinActivityEnrollmentCandidate) (marketing.SkcInfo, string, bool)
```

Update promotion and time-limited callers to use the same product builder. Remove `promotionSnapshotHasDifferentSKUPrices` when it has no remaining callers.

In `RegisterPromotionProducts`, remove the call to `rejectPromotionProductsWithDifferentSKUPrices` and delete that helper plus `promotionProductHasDifferentSKUPrices` when unused.

- [ ] **Step 5: Format, verify GREEN, and commit**

```powershell
gofmt -w internal/listingkit/sheinsync/activity_adapter.go internal/listingkit/sheinsync/enrollment_service_test.go internal/shein/activity/registration.go internal/shein/activity/registration_direct_products_test.go
go test ./internal/listingkit/sheinsync -run TestSheinActivityAdapterPromotionPassesMultiSKUPricesAndCosts -count=1
go test ./internal/shein/activity -run TestRegisterPromotionProductsAcceptsPromotionMultiSkuDifferentPrices -count=1
git add -- internal/listingkit/sheinsync/activity_adapter.go internal/listingkit/sheinsync/enrollment_service_test.go internal/shein/activity/registration.go internal/shein/activity/registration_direct_products_test.go
git commit -m "fix: allow shein promotion multi-sku prices"
```

---

### Task 3: Select the Minimum Supported SKU Discount

**Files:**
- Modify: `internal/shein/activity/registration_config.go:141-406`
- Test: `internal/shein/activity/registration_direct_products_test.go`

**Interfaces:**
- Produces: `minimumPromotionSKUDiscountRate(product marketing.SkcInfo, minProfitRate float64, fixedPriceAdjustment float64, priceMode string) (int, bool, bool)` where the final boolean reports whether SKU-level data was present.
- Consumes: existing `calculatePriceByProfit`, `calculatePriceByBreakeven`, `firstAvailablePromotionSalePrice`, and `ValidateDropRate` helpers.

- [ ] **Step 1: Add the store-177 regression test**

Add a direct registration test using three representative SKUs from `sh260625180761728097751`:

```go
func TestRegisterPromotionProductsUsesMinimumMultiSKUDiscountForBoth(t *testing.T) {
	product := marketing.SkcInfo{
		Skc:   "sh260625180761728097751",
		Stock: 10989,
		SkuPriceInfoList: []marketing.SkuSitePriceInfo{
			{SkuCode: "sku-min", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 39.20, Currency: "USD", IsAvailable: true}}},
			{SkuCode: "sku-mid", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 43.50, Currency: "USD", IsAvailable: true}}},
			{SkuCode: "sku-deep", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 23.00, Currency: "USD", IsAvailable: true}}},
		},
		SkuCostPriceInfoList: []marketing.SkuCostPriceInfo{
			{SkuCode: "sku-min", CostPrice: 23.88, Currency: "USD"},
			{SkuCode: "sku-mid", CostPrice: 20.88, Currency: "USD"},
			{SkuCode: "sku-deep", CostPrice: 8.88, Currency: "USD"},
		},
	}
	api := &promotionProductsMarketingAPIStub{}
	service := &activityRegistrationServiceImpl{marketingAPI: api, logger: logrus.NewEntry(logrus.New())}
	result, err := service.RegisterPromotionProducts(t.Context(), &listingruntime.OperationStrategy{
		StoreID: 177, ActivityPriceMode: "BREAKEVEN", ActivityPartakeType: "BOTH", ActivityStockRatio: 0.5,
	}, "", []marketing.SkcInfo{product})
	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if result == nil || len(result.Requests) != 2 {
		t.Fatalf("requests = %+v, want regular and limited", result)
	}
	if got := result.Requests[0].ConfigList[0].DropRate; got != 38 {
		t.Fatalf("regular drop rate = %d, want 38", got)
	}
	if got := result.Requests[1].ConfigList[0].DropRate; got != 39 {
		t.Fatalf("limited drop rate = %d, want 39", got)
	}
}
```

- [ ] **Step 2: Add a partial-data rejection test**

Add this partial-data case:

```go
func TestRegisterPromotionProductsRejectsPartialMultiSKUPrices(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{}
	service := &activityRegistrationServiceImpl{marketingAPI: api, logger: logrus.NewEntry(logrus.New())}
	result, err := service.RegisterPromotionProducts(t.Context(), &listingruntime.OperationStrategy{
		StoreID: 177, ActivityPriceMode: "BREAKEVEN", ActivityPartakeType: "REGULAR", ActivityStockRatio: 0.5,
	}, "", []marketing.SkcInfo{{
		Skc: "skc-partial", Stock: 10,
		SkuPriceInfoList: []marketing.SkuSitePriceInfo{
			{SkuCode: "sku-one", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 30, IsAvailable: true}}},
			{SkuCode: "sku-two", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 40, IsAvailable: true}}},
		},
		SkuCostPriceInfoList: []marketing.SkuCostPriceInfo{{SkuCode: "sku-one", CostPrice: 18}},
	}})
	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if result == nil || result.Request != nil || len(result.Requests) != 0 {
		t.Fatalf("result = %+v, want no promotion request", result)
	}
	if api.saved != nil {
		t.Fatalf("saved request = %+v, want SaveConfig not called", api.saved)
	}
}
```

- [ ] **Step 3: Run tests and verify RED**

```powershell
go test ./internal/shein/activity -run 'TestRegisterPromotionProductsUsesMinimumMultiSKUDiscountForBoth|TestRegisterPromotionProductsRejectsPartialMultiSKUPrices' -count=1
```

Expected: the first test produces the current scalar/average discount instead of `38/39`; the second incorrectly falls back to scalar pricing or accepts a partial set.

- [ ] **Step 4: Implement the minimum-discount helper**

Add a helper that indexes costs by SKU code, requires a complete match, calculates each activity price, and keeps the smallest discount:

```go
func minimumPromotionSKUDiscountRate(
	product marketing.SkcInfo,
	minProfitRate float64,
	fixedPriceAdjustment float64,
	priceMode string,
) (int, bool, bool) {
	if len(product.SkuPriceInfoList) == 0 && len(product.SkuCostPriceInfoList) == 0 {
		return 0, false, false
	}
	if len(product.SkuPriceInfoList) == 0 || len(product.SkuCostPriceInfoList) == 0 {
		return 0, false, true
	}

	costBySKU := make(map[string]float64, len(product.SkuCostPriceInfoList))
	for _, item := range product.SkuCostPriceInfoList {
		if item.SkuCode == "" || item.CostPrice <= 0 {
			return 0, false, true
		}
		costBySKU[item.SkuCode] = item.CostPrice
	}

	minimumRate := 1.0
	for _, item := range product.SkuPriceInfoList {
		originalPrice := firstAvailablePromotionSalePrice(item.SitePriceInfoList)
		costPrice, ok := costBySKU[item.SkuCode]
		if item.SkuCode == "" || originalPrice <= 0 || !ok || costPrice <= 0 {
			return 0, false, true
		}
		activityPrice := calculatePriceByBreakeven(originalPrice, costPrice, fixedPriceAdjustment)
		if strings.EqualFold(priceMode, "PROFIT") {
			activityPrice = calculatePriceByProfit(originalPrice, costPrice, minProfitRate, fixedPriceAdjustment)
		}
		if activityPrice <= 0 {
			return 0, false, true
		}
		rate := (originalPrice - activityPrice) / originalPrice
		if rate < minimumRate {
			minimumRate = rate
		}
	}
	return ValidateDropRate(int(minimumRate*100), minimumRate, nil), true, true
}
```

Use this helper at the start of `dropRateFromProvidedProduct` for `PROFIT` and `BREAKEVEN`. If SKU data is present but invalid, reject the product; if absent, retain scalar fallback.

In `buildActivityConfigsByProfit`, replace average accumulation with the minimum per-SKU discount and reject missing SKU prices/costs instead of silently continuing.

- [ ] **Step 5: Verify GREEN and existing BOTH behavior**

```powershell
gofmt -w internal/shein/activity/registration_config.go internal/shein/activity/registration_direct_products_test.go
go test ./internal/shein/activity -run 'TestRegisterPromotionProductsUsesMinimumMultiSKUDiscountForBoth|TestRegisterPromotionProductsRejectsPartialMultiSKUPrices|TestRegisterPromotionProductsKeepsLimitedProfitDropRateGreaterAfterRounding' -count=1
```

Expected: store-177 fixture produces regular `38`, limited `39`; partial data is rejected; existing limited-greater-than-regular behavior passes.

- [ ] **Step 6: Run cross-package verification**

```powershell
gopls check internal/listingkit/sheinsync/enrollment_service_candidates.go internal/listingkit/sheinsync/activity_adapter.go internal/shein/activity/registration.go internal/shein/activity/registration_config.go
go test ./internal/listingkit/sheinsync ./internal/shein/activity -count=1
go vet ./internal/listingkit/sheinsync ./internal/shein/activity
git diff --check
```

Expected: all commands exit 0; Windows may print LF-to-CRLF warnings without whitespace errors.

- [ ] **Step 7: Commit the pricing behavior**

```powershell
git add -- internal/shein/activity/registration_config.go internal/shein/activity/registration_direct_products_test.go
git commit -m "fix: use minimum shein promotion sku discount"
```
