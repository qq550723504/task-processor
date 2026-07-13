# 限时折扣保本价按 SKU 提交 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让旧限时折扣入口在 `BREAKEVEN` 模式下为每个 SKU 计算并提交独立保本活动价，同时允许同一 SKC 的有效 SKU 单独报名。

**Architecture:** 扩展 `ProductDataHelper`，从同步的 Attributes 建立 SKC 内的 SKU Amazon 成本索引。旧入口的价格请求构建器只在 `BREAKEVEN` 时使用该索引，按 SKU 写入保本价；创建请求已有的 requested-SKU 过滤继续保证只提交有效 SKU。

**Tech Stack:** Go、`productsync.EnrichedSkcInfo`、Go 标准测试框架。

## Global Constraints

- 只修改 `CreateTimeLimitedDiscountActivity` 使用的旧定价请求构建路径。
- `DISCOUNT` 与 `PROFIT` 的行为保持不变。
- `BREAKEVEN` 成本来源为同 SKU 的 `AmazonMonitorData.Price`。
- 无效 SKU 单独跳过；只有 SKC 没有有效 SKU 才跳过 SKC。
- 生产代码之前先写并运行失败测试。

---

## File structure

- `internal/shein/activity/product_data_helper.go`：SKU 归一化成本索引。
- `internal/shein/activity/price_calculator.go`：旧限时 `BREAKEVEN` 逐 SKU 计算。
- `internal/shein/activity/price_calculator_test.go`：成本索引与请求价格测试。
- `internal/shein/activity/registration_direct_products_test.go`：部分 SKU 提交回归测试。

### Task 1: 添加 SKU Amazon 成本索引

**Files:**

- Modify: `internal/shein/activity/product_data_helper.go:117-137`
- Test: `internal/shein/activity/price_calculator_test.go`

**Interfaces:**

- Consumes: `productsync.EnrichedSkcInfo.SkuInfo`、`EnrichedSkuInfo.SkuCode`、`AmazonMonitorData.Price`。
- Produces: `func (h *ProductDataHelper) AmazonCostBySKU(skcData *productsync.EnrichedSkcInfo) map[string]float64`。

- [ ] **Step 1: Write the failing test**

Add `TestAmazonCostBySKUUsesNormalizedSKUAndSkipsMissingCosts`. Its fixture contains costs `sku-small=12.5`, `SKU-LARGE=20.5`, and one SKU without Amazon data; assert the result equals `map[string]float64{"SKU-SMALL": 12.5, "SKU-LARGE": 20.5}`.

- [ ] **Step 2: Run the test to verify it fails**

Run: `$env:GOWORK='off'; go test ./internal/shein/activity -run TestAmazonCostBySKUUsesNormalizedSKUAndSkipsMissingCosts -count=1`

Expected: FAIL because `AmazonCostBySKU` does not exist.

- [ ] **Step 3: Write the minimal implementation**

```go
func (h *ProductDataHelper) AmazonCostBySKU(skcData *productsync.EnrichedSkcInfo) map[string]float64 {
	costs := make(map[string]float64)
	if skcData == nil {
		return costs
	}
	for _, sku := range skcData.SkuInfo {
		code := normalizePromotionSKUCode(sku.SkuCode)
		if code == "" || sku.AmazonMonitorData == nil || sku.AmazonMonitorData.Price <= 0 {
			continue
		}
		costs[code] = sku.AmazonMonitorData.Price
	}
	return costs
}
```

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `$env:GOWORK='off'; go test ./internal/shein/activity -run TestAmazonCostBySKUUsesNormalizedSKUAndSkipsMissingCosts -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

Run: `git add internal/shein/activity/product_data_helper.go internal/shein/activity/price_calculator_test.go; git commit -m "feat(shein): expose sku-level amazon costs"`

### Task 2: 为旧限时保本模式构造独立 SKU 价格

**Files:**

- Modify: `internal/shein/activity/price_calculator.go:113-188`
- Test: `internal/shein/activity/price_calculator_test.go`

**Interfaces:**

- Consumes: `BuildSkcDataMap`、`AmazonCostBySKU`、`promotionSKUUSSupplyPrice`、`calculatePriceByBreakeven`。
- Produces: `buildCalculateRequestWithPriceMode` 的 `BREAKEVEN` 分支为每个有效 SKU 填充 `SkuPriceInfo{ProductPrice, DiscountValue}`。

- [ ] **Step 1: Write the failing test**

Add `TestBuildCalculateRequestWithPriceModeBreakevenUsesPerSKUCost`. Use a fake `listingadmin.ProductDataRepository` whose `ListProductData` returns serialized Attributes for `skc-1`. Submit three SKU: `(30, 12.5)`, `(45, 20.5)`, `(18, 20)`, with fixed adjustment `1`. Assert the request includes only the first two and their `DiscountValue` values are `13.5` and `21.5`.

- [ ] **Step 2: Run the test to verify it fails**

Run: `$env:GOWORK='off'; go test ./internal/shein/activity -run TestBuildCalculateRequestWithPriceModeBreakevenUsesPerSKUCost -count=1`

Expected: FAIL because the current function sends the discount-mode value and retains the invalid SKU.

- [ ] **Step 3: Write the minimal implementation**

Load the existing SKC data map for both `PROFIT` and `BREAKEVEN`. Preserve the `PROFIT` block. In the SKU loop, introduce an explicit `BREAKEVEN` branch:

```go
case "BREAKEVEN":
	costs := helper.AmazonCostBySKU(skcDataMap[g.Skc])
	skuDiscountValue = calculatePriceByBreakeven(
		productPrice,
		costs[normalizedPromotionSKUCode(sku.Sku)],
		config.FixedPriceAdjustment,
	)
```

Skip a SKU when its product price or activity price is non-positive. Append an SKC only if it has at least one valid SKU. Keep the `DISCOUNT` and `PROFIT` expressions unchanged.

- [ ] **Step 4: Run focused and package tests to verify they pass**

Run:

```powershell
$env:GOWORK='off'; go test ./internal/shein/activity -run TestBuildCalculateRequestWithPriceModeBreakevenUsesPerSKUCost -count=1
$env:GOWORK='off'; go test ./internal/shein/activity -count=1
```

Expected: both commands PASS.

- [ ] **Step 5: Commit**

Run: `git add internal/shein/activity/price_calculator.go internal/shein/activity/price_calculator_test.go; git commit -m "fix(shein): calculate limited breakeven per sku"`

### Task 3: Prove creation retains only valid SKU

**Files:**

- Test: `internal/shein/activity/registration_direct_products_test.go`

**Interfaces:**

- Consumes: `buildCreateActivityRequest(config, goods, calcReq, calcResp)`。
- Produces: Proof that an invalid SKU omitted from `calcReq` is absent from `AddSkuList`, while its valid sibling remains.

- [ ] **Step 1: Write the failing regression test**

Add `TestBuildCreateActivityRequestKeepsValidBreakevenSKUWhenSiblingIsSkipped`. Create one SKC with `sku-valid` and `sku-invalid`; include only `sku-valid` (activity price `13.5`) in `calcReq` and in the calculation response. Assert exactly one submitted SKU with `Sku == "sku-valid"` and `ProductActPrice == 13.5`.

- [ ] **Step 2: Run the test to verify it fails on the unmodified code**

Run: `$env:GOWORK='off'; go test ./internal/shein/activity -run TestBuildCreateActivityRequestKeepsValidBreakevenSKUWhenSiblingIsSkipped -count=1`

Expected: FAIL before Task 2 because the invalid SKU is included in the calculation request.

- [ ] **Step 3: Keep the requested-SKU filtering behavior**

No additional production code is expected after Task 2. The production invariant is:

```go
if _, requested := requestedSKUProductPriceBySKC[g.Skc][skuCode]; calcReq != nil && !requested {
	continue
}
```

- [ ] **Step 4: Run focused and package tests to verify they pass**

Run:

```powershell
$env:GOWORK='off'; go test ./internal/shein/activity -run TestBuildCreateActivityRequestKeepsValidBreakevenSKUWhenSiblingIsSkipped -count=1
$env:GOWORK='off'; go test ./internal/shein/activity -count=1
```

Expected: both commands PASS.

- [ ] **Step 5: Commit the regression test**

Run: `git add internal/shein/activity/registration_direct_products_test.go; git commit -m "test(shein): cover partial limited breakeven enrollment"`

### Task 4: Final verification

- [ ] **Step 1: Format changed Go files**

Run: `gofmt -w internal/shein/activity/product_data_helper.go internal/shein/activity/price_calculator.go internal/shein/activity/price_calculator_test.go internal/shein/activity/registration_direct_products_test.go`

- [ ] **Step 2: Run complete relevant verification**

Run:

```powershell
$env:GOWORK='off'; go test ./internal/shein/activity -count=1
$env:GOWORK='off'; go vet ./internal/shein/activity
git diff --check
git status --short
```

Expected: tests and vet PASS; `git diff --check` is clean; status contains only intended files.

