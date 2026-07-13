# ListingKit 保本活动价修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 ListingKit 限时促销的保本模式按 SKU 直接提交“成本加固定调整值”的活动价。

**Architecture:** 保留现有 SKU 成本与目标活动价计算、SHEIN 价格计算和风险校验。仅修改最终创建请求组装阶段，使 `BREAKEVEN` 与 `PROFIT` 一样优先采用计算请求中的 SKU 目标活动价。

**Tech Stack:** Go、testing、SHEIN marketing API request models。

## Global Constraints

- 不改变 `PROFIT` 或 `DISCOUNT` 的既有定价行为。
- 不修改外部活动或既有报名记录。
- 所有新行为先以失败测试锁定。

---

### Task 1: 保本最终活动价回归测试与修复

**Files:**

- Modify: `internal/shein/activity/registration_direct_products_test.go`
- Modify: `internal/shein/activity/time_limited.go`

**Interfaces:**

- Consumes: `buildCreateActivityRequest(config, goods, calcReq, calcResp)`
- Produces: `marketing.CreateActivityRequest.AddCostAndStockInfoList[].AddSkuList[].ProductActPrice`

- [x] **Step 1: Write the failing test**

在现有 `TestRegisterPromotionProductsUsesSKUCostsForBreakevenModeSkuActivityPrices` 的 API stub 中，让价格计算响应的 `ProductAmount - PromotionAmount` 与请求的 `DiscountValue` 不同；断言最终 `ProductActPrice` 仍为每个 SKU 的成本加调整值。

- [x] **Step 2: Run test to verify it fails**

Run: `GOWORK=off go test ./internal/shein/activity -run TestRegisterPromotionProductsUsesSKUCostsForBreakevenModeSkuActivityPrices -count=1`

Expected: FAIL，最终活动价采用了价格计算响应而不是保本目标价。

- [x] **Step 3: Write minimal implementation**

在 `buildCreateActivityRequest` 中将条件改为：

```go
useRequestedActivityPrices := strings.EqualFold(config.PriceMode, "PROFIT") ||
	strings.EqualFold(config.PriceMode, "BREAKEVEN")
```

- [x] **Step 4: Run test to verify it passes**

Run: `GOWORK=off go test ./internal/shein/activity -run TestRegisterPromotionProductsUsesSKUCostsForBreakevenModeSkuActivityPrices -count=1`

Expected: PASS。

- [x] **Step 5: Run regression verification and commit**

Run:

```powershell
$env:GOWORK='off'
go test ./internal/shein/activity -count=1
go vet ./internal/shein/activity
git diff --check
```

Commit: `fix(shein): preserve breakeven sku activity prices`
