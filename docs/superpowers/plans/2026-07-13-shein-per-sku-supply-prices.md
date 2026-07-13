# SHEIN Per-SKU Supply Prices Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist the SHEIN cost-price response for every SKU and display it in the ListingKit synced-products table.

**Architecture:** Add a JSON `supply_price_snapshot` to the synced-product record. The cost resolver will retain one valid price per SKU while preserving the current maximum `SupplyPrice` only for backwards compatibility. The existing list API serializes the record; the Next.js table parses this new snapshot independently of the sale-price snapshot.

**Tech Stack:** Go, GORM AutoMigrate, React 19, Next.js 16, TypeScript, Vitest.

## Global Constraints

- `price_snapshot` remains a sales-price snapshot and is never a supply-price fallback.
- `supply_price_snapshot` only contains values returned by the SHEIN cost-price endpoint.
- Existing rows obtain SKU supply prices after their next sync.
- The product-level `supply_price` remains a compatibility maximum; the UI renders the SKU snapshot.

---

### Task 1: Preserve SKU supply prices in the sync domain

**Files:**
- Modify: `internal/listingkit/sheinsync/model_sync_products.go`
- Modify: `internal/listingkit/sheinsync/cost_resolver.go`
- Test: `internal/listingkit/sheinsync/cost_resolver_test.go`

**Interfaces:**
- Produces `SheinSKUSupplyPrice { SKUCode string; SupplyPrice float64; Currency string }`.
- Extends `resolvedSheinCost` with `SKUSupplyPrices []SheinSKUSupplyPrice`.

- [ ] **Step 1: Write the failing resolver test**

```go
require.Equal(t, []SheinSKUSupplyPrice{
  {SKUCode: "sku-small", SupplyPrice: 12.50, Currency: "USD"},
  {SKUCode: "sku-large", SupplyPrice: 18.25, Currency: "USD"},
}, resolved["skc-1"].SKUSupplyPrices)
```

- [ ] **Step 2: Run the test to prove the SKU list is discarded**

Run: `GOWORK=off go test ./internal/listingkit/sheinsync -run TestSheinProductCostResolver -count=1`

- [ ] **Step 3: Add the snapshot type and retain endpoint values**

```go
type SheinSKUSupplyPrice struct {
  SKUCode string `json:"sku_code"`
  SupplyPrice float64 `json:"supply_price"`
  Currency string `json:"currency"`
}
```

For every non-empty SKU code with a positive parseable cost, append one entry and sort the result by SKU code. Keep the maximum only in the existing compatibility field.

- [ ] **Step 4: Format and re-run the resolver test**

Run: `gofmt -w internal/listingkit/sheinsync/model_sync_products.go internal/listingkit/sheinsync/cost_resolver.go; GOWORK=off go test ./internal/listingkit/sheinsync -run TestSheinProductCostResolver -count=1`

### Task 2: Persist and expose the SKU supply-price snapshot

**Files:**
- Modify: `internal/listingkit/sheinsync/model_sync_products.go`
- Modify: `internal/listingkit/sheinsync/service_sync.go`
- Modify: `internal/listingkit/store/shein_sync_repo_products.go`
- Test: `internal/listingkit/sheinsync/service_test.go`

**Interfaces:**
- Consumes `resolvedSheinCost.SKUSupplyPrices`.
- Produces `SheinSyncedProductRecord.SupplyPriceSnapshot string` as `{"sku_supply_prices":[...]}`.

- [ ] **Step 1: Write the failing sync test**

```go
require.JSONEq(t,
  `{"sku_supply_prices":[{"sku_code":"sku-large","supply_price":18.25,"currency":"USD"},{"sku_code":"sku-small","supply_price":12.5,"currency":"USD"}]}`,
  rows[0].SupplyPriceSnapshot,
)
```

- [ ] **Step 2: Run the focused sync test to confirm the field is empty**

Run: `GOWORK=off go test ./internal/listingkit/sheinsync -run TestSyncSheinOnShelfProductsPersistsSKU -count=1`

- [ ] **Step 3: Serialize and save the snapshot in both sync flows**

```go
record.SupplyPriceSnapshot = marshalSheinSKUSupplyPrices(resolved.SKUSupplyPrices)
```

Add a text GORM field, add it to `sheinSyncedProductAssignments`, and retain the former snapshot only when cost resolution is unavailable.

- [ ] **Step 4: Re-run sync and store package tests**

Run: `GOWORK=off go test ./internal/listingkit/sheinsync ./internal/listingkit/store -run 'TestSyncSheinOnShelfProductsPersistsSKU|TestAutoMigrateSheinSyncRepository' -count=1`

### Task 3: Render the supply-price snapshot

**Files:**
- Modify: `web/listingkit-ui/src/lib/types/listingkit/shein-enrollment.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-price-snapshot.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-products/shein-synced-products-table.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shein-products/shein-synced-products-table.test.tsx`

**Interfaces:**
- Consumes `SheinSyncedProductRecord.supply_price_snapshot`.
- Produces `getSheinSKUSupplyPriceSnapshots(value)` returning `{ skuCode, price }[]`.

- [ ] **Step 1: Write the failing table test**

```tsx
expect(screen.getByText("SKU 供货价（SHEIN 供货价接口）")).toBeInTheDocument();
expect(screen.getByText("SKU: sku-small")).toBeInTheDocument();
expect(screen.getByText("$12.50")).toBeInTheDocument();
```

- [ ] **Step 2: Run the table test to confirm the label is absent**

Run: `npm test -- shein-synced-products-table.test.tsx`

- [ ] **Step 3: Add a strict JSON parser and table section**

Accept only a non-empty `sku_code`, positive numeric `supply_price`, and optional `currency`. Render the results under sales prices. When absent, show `SKU 供货价：下次同步后可见`; never derive it from the sales-price snapshot or SDS costs.

- [ ] **Step 4: Run the front-end checks**

Run: `npm test -- shein-synced-products-table.test.tsx; npm run typecheck; npm run build`

### Task 4: Verify and commit

**Files:**
- Verify all files above

- [ ] **Step 1: Run formatting, Go package checks, and final diff check**

Run: `powershell -ExecutionPolicy Bypass -File "$env:USERPROFILE\.codex\skills\go-dev-workflow\scripts\run-go-checks.ps1" -Path . -PackagePattern ./internal/listingkit/... -Format -Vet -Test; git diff --check`

- [ ] **Step 2: Commit the implementation**

```powershell
git add internal/listingkit/sheinsync internal/listingkit/store web/listingkit-ui/src/lib/types/listingkit/shein-enrollment.ts web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-price-snapshot.ts web/listingkit-ui/src/components/listingkit/shein-products/shein-synced-products-table.tsx
git commit -m "feat(listingkit): persist SKU supply price snapshots"
```
