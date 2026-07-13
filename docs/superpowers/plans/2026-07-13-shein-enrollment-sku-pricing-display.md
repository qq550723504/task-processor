# SHEIN 报名候选池 SKU 价格展示 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 SHEIN 活动报名候选池中按 SKU 展示与实际报名完全一致的供货原价和 SDS 成本。

**Architecture:** 抽出候选商品在报名执行前所使用的价格/成本刷新逻辑，供报名执行与候选池查询共同调用。候选池 API 返回已刷新过的 `price_snapshot` 和 `sku_cost_price_info_list`；客户端合并这两份 SKU 明细，以表格行展示原价（供货价）与成本（SDS）。

**Tech Stack:** Go、GORM、Gin、Next.js 16、React 19、TypeScript、Vitest、Testing Library。

## Global Constraints

- 原价必须是同步的 SHEIN 供货价，不是 SHEIN 销售价。
- 成本必须是当前生效的 SDS 手工成本；不改动活动价公式或 SHEIN 请求载荷。
- 候选池和报名执行必须调用同一价格/成本刷新函数，禁止复制计算规则。
- 不新增候选记录持久化字段或数据库迁移；候选查询动态解析当前数据。
- 缺少 SKU 原价或成本时显示 `-`，不得使用商品汇总值伪造 SKU 值。

---

## 文件结构

- `internal/listingkit/sheinsync/enrollment_candidate_pricing.go`：候选记录在展示和报名执行前共同使用的供货价、SDS 成本刷新函数。
- `internal/listingkit/sheinsync/enrollment_service_candidates.go`：删除局部重复实现，改为调用共享函数。
- `internal/listingkit/sheinsync/candidate_service.go`：候选池查询在返回前调用共享函数。
- `internal/listingkit/sheinsync/candidate_service_test.go`：验证候选池返回的 SKU 原价和 SDS 成本与报名解析结果一致。
- `web/listingkit-ui/src/lib/types/listingkit/shein-enrollment.ts`：声明候选记录返回的 SKU 成本类型。
- `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-price-snapshot.ts`：复用金额格式化并提供 SKU 原价解析结果。
- `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-candidates-table.tsx`：按 SKU 对齐渲染原价和成本。
- `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx`：候选池组件展示断言。
- `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-price-snapshot.test.ts`：金额/快照解析回归测试。

### Task 1: 复用报名价格解析并让候选池返回当前 SKU 成本

**Files:**
- Create: `internal/listingkit/sheinsync/enrollment_candidate_pricing.go`
- Modify: `internal/listingkit/sheinsync/enrollment_service_candidates.go`
- Modify: `internal/listingkit/sheinsync/candidate_service.go:153-158`
- Test: `internal/listingkit/sheinsync/candidate_service_test.go`

**Interfaces:**
- Consumes: `SheinCandidateRepository.ListCandidates`、`SheinCandidateRepository.ListSyncedProducts`、`applySheinSDSCostGroupOverrides`、`refreshSheinEnrollmentPriceSnapshot`。
- Produces: `refreshSheinEnrollmentCandidatePricing(ctx, repo, tenantID, storeID, candidates) ([]SheinActivityCandidateRecord, error)`。

- [ ] **Step 1: 写出候选池返回当前报名输入的失败测试**

在 `candidate_service_test.go` 加入测试，保存一个旧候选记录，然后让同步商品提供不同的 SKU 供货价和不同 SKU 的 SDS 成本：

```go
func TestSheinCandidateServiceListCandidatesUsesCurrentEnrollmentSKUPricesAndCosts(t *testing.T) {
    // Candidate: SKU-A=99, SKU-B=88; supply: SKU-A=27.38, SKU-B=31.62.
    rows, total, err := service.ListCandidates(context.Background(), query)
    require.NoError(t, err)
    require.Equal(t, int64(1), total)
    require.Contains(t, rows[0].PriceSnapshot, `"sale_price":27.38`)
    require.Contains(t, rows[0].PriceSnapshot, `"sale_price":31.62`)
    require.Equal(t, []SheinSKUCostPrice{
        {SKUCode: "SKU-A", CostPrice: 19.99},
        {SKUCode: "SKU-B", CostPrice: 22.50},
    }, rows[0].SKUCostPriceInfoList)
}
```

- [ ] **Step 2: 运行测试，确认它因候选池直接返回持久化快照而失败**

Run: `$env:GOWORK='off'; go test ./internal/listingkit/sheinsync -run TestSheinCandidateServiceListCandidatesUsesCurrentEnrollmentSKUPricesAndCosts -count=1`

Expected: FAIL，`PriceSnapshot` 仍为旧值或 `SKUCostPriceInfoList` 为空。

- [ ] **Step 3: 新建共享刷新函数并替换报名执行中的局部实现**

在 `enrollment_candidate_pricing.go` 定义最小共享仓储接口和函数：

```go
type sheinEnrollmentCandidatePricingRepository interface {
    ListSyncedProducts(context.Context, *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error)
}

func refreshSheinEnrollmentCandidatePricing(
    ctx context.Context,
    repo sheinEnrollmentCandidatePricingRepository,
    tenantID, storeID int64,
    candidates []SheinActivityCandidateRecord,
) ([]SheinActivityCandidateRecord, error) {
    // Query current active products, apply SDS overrides, refresh price/cost/profit.
}
```

将现有 `sheinEnrollmentService.refreshCandidateCostOverrides` 替换为该函数调用，保留活动提交行为。

- [ ] **Step 4: 候选池查询调用共享函数**

将 `sheinCandidateService.ListCandidates` 从直接返回改为：

```go
rows, total, err := s.repo.ListCandidates(ctx, query)
if err != nil {
    return nil, 0, err
}
rows, err = refreshSheinEnrollmentCandidatePricing(ctx, s.repo, query.TenantID, query.StoreID, rows)
if err != nil {
    return nil, 0, err
}
return rows, total, nil
```

保留空结果、仓储错误和分页总数的现有语义。

- [ ] **Step 5: 运行定向测试，确认通过**

Run: `$env:GOWORK='off'; go test ./internal/listingkit/sheinsync -run TestSheinCandidateServiceListCandidatesUsesCurrentEnrollmentSKUPricesAndCosts -count=1`

Expected: PASS。

- [ ] **Step 6: 提交后端共享逻辑**

```powershell
git add internal/listingkit/sheinsync/enrollment_candidate_pricing.go internal/listingkit/sheinsync/enrollment_service_candidates.go internal/listingkit/sheinsync/candidate_service.go internal/listingkit/sheinsync/candidate_service_test.go
git commit -m "fix(shein): expose enrollment SKU pricing in candidates"
```

### Task 2: 声明 SKU 成本并在候选池按 SKU 展示

**Files:**
- Modify: `web/listingkit-ui/src/lib/types/listingkit/shein-enrollment.ts:130-154`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-price-snapshot.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-candidates-table.tsx:4-170`
- Test: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-price-snapshot.test.ts`
- Test: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx:255-283`

**Interfaces:**
- Consumes: `SheinActivityCandidateRecord.price_snapshot` 和新的 `sku_cost_price_info_list`。
- Produces: `getSheinCandidateSKUPriceRows(item)`，每行具有 `skuCode`、`originalPrice`、`costPrice`。

- [ ] **Step 1: 写出失败的前端展示测试**

在 workbench 测试候选记录中提供：

```ts
price_snapshot: JSON.stringify({
  currency: "USD",
  sku_prices: [
    { sku_code: "SKU-A", sale_price: 27.38, currency: "USD" },
    { sku_code: "SKU-B", sale_price: 31.62, currency: "USD" },
  ],
}),
sku_cost_price_info_list: [
  { sku_code: "SKU-A", cost_price: 19.99, currency: "USD" },
  { sku_code: "SKU-B", cost_price: 22.5, currency: "USD" },
],
```

断言页面出现 `SKU`、`原价（供货价）`、`成本（SDS）`、`SKU-A`、`$27.38`、`$19.99`、`SKU-B`、`$31.62`、`$22.50`。

- [ ] **Step 2: 运行测试，确认因当前仅显示商品级“售价/成本”而失败**

Run: `npm test -- src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx`

Expected: FAIL，找不到 `原价（供货价）` 或 SKU 成本值。

- [ ] **Step 3: 添加类型和纯展示映射函数**

在 TypeScript 类型中加入：

```ts
export type SheinSKUCostPrice = {
  sku_code?: string;
  cost_price?: number;
  currency?: string;
};

// SheinActivityCandidateRecord
sku_cost_price_info_list?: SheinSKUCostPrice[];
```

在 `shein-price-snapshot.ts` 导出 `formatSheinCurrencyAmount(currency, amount)`，以同一规则格式化原价和成本。

- [ ] **Step 4: 渲染按 SKU 对齐的三列表格**

在 `shein-candidates-table.tsx` 从 `getSheinSKUPriceSnapshots` 和 `sku_cost_price_info_list` 建立 SKU 并集，按 SKU 字典序排列；对每行渲染：

```tsx
<div className="mt-3 overflow-x-auto rounded-lg border border-zinc-100">
  <div className="grid min-w-[420px] grid-cols-[minmax(150px,1fr)_130px_130px] text-xs">
    <span>SKU</span>
    <span>原价（供货价）</span>
    <span>成本（SDS）</span>
    {rows.map((row) => (
      <Fragment key={row.skuCode}>
        <span>{row.skuCode}</span>
        <span>{row.originalPrice ?? "-"}</span>
        <span>{row.costPrice ?? "-"}</span>
      </Fragment>
    ))}
  </div>
</div>
```

移除会误导用户的 `售价 {formatSheinPriceSnapshot(...)}` 文案；保留状态、资格、利润率和报名操作。

- [ ] **Step 5: 增加缺失数据回归测试并运行前端定向测试**

在 `shein-price-snapshot.test.ts` 覆盖非法金额/货币，组件测试覆盖只有成本或只有原价时另一列显示 `-`。

Run: `npm test -- src/components/listingkit/shein-enrollment/shein-price-snapshot.test.ts src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx`

Expected: PASS。

- [ ] **Step 6: 提交前端展示**

```powershell
git add web/listingkit-ui/src/lib/types/listingkit/shein-enrollment.ts web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-price-snapshot.ts web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-candidates-table.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-price-snapshot.test.ts web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx
git commit -m "feat(listingkit): show enrollment SKU source prices"
```

### Task 3: 全量验证和交付检查

**Files:**
- Modify: 仅修复 Task 1-2 验证发现的相关文件。

**Interfaces:**
- Consumes: Task 1 的共享解析函数和 Task 2 的展示类型。
- Produces: 可通过 Go 与 Next.js 验证的候选池价格展示。

- [ ] **Step 1: 运行后端完整相关测试与静态检查**

Run:

```powershell
$env:GOWORK='off'; go test ./internal/listingkit/sheinsync
$env:GOWORK='off'; go vet ./internal/listingkit/sheinsync
```

Expected: 两条命令退出码均为 0。

- [ ] **Step 2: 运行前端测试和类型检查**

Run:

```powershell
npm test -- src/components/listingkit/shein-enrollment/shein-price-snapshot.test.ts src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx
npm run typecheck
```

Expected: Vitest 全部通过，TypeScript 退出码为 0。

- [ ] **Step 3: 审查变更范围**

Run:

```powershell
git diff --check
git status --short
```

Expected: 没有空白错误，工作树只包含本计划明确需要的变更。

- [ ] **Step 4: 提交验证期间的必要修复**

```powershell
git add internal/listingkit/sheinsync web/listingkit-ui/src
git commit -m "test(listingkit): verify enrollment SKU price display"
```

