# SHEIN Product List Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade the SHEIN enrollment `products` tab into a dense operations-style list that surfaces product identity, variant, pricing, stock, status, and time metadata in one scan-friendly row.

**Architecture:** Keep the existing `products` tab and table-based desktop layout, but refactor the row rendering into grouped multi-line cells backed by small pure formatting helpers. Reuse existing API fields only, parse inventory snapshots on the frontend with graceful fallback, and lock the behavior with focused component tests before implementation.

**Tech Stack:** Next.js, React, TypeScript, Vitest, Testing Library, Tailwind CSS

---

## File Structure

- Modify: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-synced-products-table.tsx`
  - Refactor the sparse table into a dense multi-field operational list row.
- Create: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-synced-products-table.test.tsx`
  - Add focused rendering tests for the redesigned products table.
- Create: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-product-table-formatters.ts`
  - Centralize inventory parsing, cost source labeling, badge styling, and time row formatting.
- Create: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-product-table-formatters.test.ts`
  - Verify formatter behavior for price, inventory, and fallback cases.
- Modify: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-price-snapshot.ts`
  - Reuse the existing price snapshot formatter inside the redesigned table helpers if needed.
- Modify: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx`
  - Update the integration-style workbench test to assert the richer products-tab content.

## Task 1: Add Formatter Tests First

**Files:**
- Create: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-product-table-formatters.test.ts`
- Reference: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-price-snapshot.ts`

- [ ] **Step 1: Write the failing formatter tests**

```ts
import { describe, expect, it } from "vitest";

import {
  formatInventorySummary,
  formatProductTimes,
  getCostSourceLabel,
} from "@/components/listingkit/shein-enrollment/shein-product-table-formatters";

describe("shein product table formatters", () => {
  it("parses inventory snapshot objects into total and available values", () => {
    expect(
      formatInventorySummary('{"total_inventory":999,"saleable_inventory":321}'),
    ).toEqual({
      total: "999",
      available: "321",
      raw: null,
    });
  });

  it("falls back to raw inventory snapshot text when structure is unknown", () => {
    expect(formatInventorySummary("warehouse A: 50")).toEqual({
      total: "-",
      available: "-",
      raw: "warehouse A: 50",
    });
  });

  it("maps cost price source into a human label", () => {
    expect(getCostSourceLabel("manual")).toBe("人工");
    expect(getCostSourceLabel("auto")).toBe("自动");
    expect(getCostSourceLabel("none")).toBe("缺失");
  });

  it("returns labeled product times in stable display order", () => {
    expect(
      formatProductTimes({
        created_at: "2026-06-01T01:38:43Z",
        publish_time: "2026-06-02T02:58:40Z",
        first_shelf_time: "2026-06-02T21:04:59Z",
        last_sync_at: "2026-06-06T00:19:00Z",
      }),
    ).toEqual([
      ["创建", "2026-06-01T01:38:43Z"],
      ["发布", "2026-06-02T02:58:40Z"],
      ["首次上架", "2026-06-02T21:04:59Z"],
      ["最近同步", "2026-06-06T00:19:00Z"],
    ]);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- --run src/components/listingkit/shein-enrollment/shein-product-table-formatters.test.ts`

Expected: FAIL because `shein-product-table-formatters.ts` does not exist yet.

- [ ] **Step 3: Write minimal formatter implementation**

```ts
type ProductTimesInput = {
  created_at?: string;
  publish_time?: string;
  first_shelf_time?: string;
  last_sync_at?: string;
};

export function formatInventorySummary(value?: string | null) {
  const text = value?.trim();
  if (!text) {
    return { total: "-", available: "-", raw: null as string | null };
  }

  try {
    const parsed = JSON.parse(text) as Record<string, unknown>;
    const total = parsed.total_inventory ?? parsed.total ?? parsed.inventory_total;
    const available =
      parsed.saleable_inventory ?? parsed.available_inventory ?? parsed.available;
    if (total !== undefined || available !== undefined) {
      return {
        total: total === undefined ? "-" : String(total),
        available: available === undefined ? "-" : String(available),
        raw: null,
      };
    }
  } catch {
  }

  return { total: "-", available: "-", raw: text };
}

export function getCostSourceLabel(source?: string | null) {
  switch (source) {
    case "manual":
      return "人工";
    case "auto":
      return "自动";
    case "none":
    default:
      return "缺失";
  }
}

export function formatProductTimes(input: ProductTimesInput) {
  return [
    ["创建", input.created_at || "-"],
    ["发布", input.publish_time || "-"],
    ["首次上架", input.first_shelf_time || "-"],
    ["最近同步", input.last_sync_at || "-"],
  ] as const;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- --run src/components/listingkit/shein-enrollment/shein-product-table-formatters.test.ts`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-product-table-formatters.ts web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-product-table-formatters.test.ts
git commit -m "test: add shein product table formatters"
```

## Task 2: Lock the Dense Product Row with a Failing Component Test

**Files:**
- Create: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-synced-products-table.test.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx`
- Reference: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-synced-products-table.tsx`

- [ ] **Step 1: Write the failing products-table component test**

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SheinSyncedProductsTable } from "@/components/listingkit/shein-enrollment/shein-synced-products-table";

describe("SheinSyncedProductsTable", () => {
  it("renders dense product operation details", () => {
    render(
      <SheinSyncedProductsTable
        isLoading={false}
        items={[
          {
            id: 1,
            main_image_url: "https://example.com/item.png",
            product_name_multi: "Round Drawer Knobs",
            spu_code: "spu-123",
            supplier_code: "J0529021001",
            sale_name: "White",
            skc_name: "skc-001",
            price_snapshot: "USD 34.17",
            effective_cost_price: 12.5,
            cost_price_source: "manual",
            inventory_snapshot: '{"total_inventory":999,"saleable_inventory":999}',
            shelf_status: "ON_SHELF",
            created_at: "2026-06-01 01:38:43",
            publish_time: "2026-06-02 02:58:40",
            first_shelf_time: "2026-06-02 21:04:59",
            last_sync_at: "2026-06-06 00:19:00",
          },
        ]}
      />,
    );

    expect(screen.getByText("Round Drawer Knobs")).toBeInTheDocument();
    expect(screen.getByText("SPU: spu-123")).toBeInTheDocument();
    expect(screen.getByText("货号: J0529021001")).toBeInTheDocument();
    expect(screen.getByText("White")).toBeInTheDocument();
    expect(screen.getByText("SKC: skc-001")).toBeInTheDocument();
    expect(screen.getByText("$34.17")).toBeInTheDocument();
    expect(screen.getByText("人工")).toBeInTheDocument();
    expect(screen.getByText("总库存 999")).toBeInTheDocument();
    expect(screen.getByText("可用库存 999")).toBeInTheDocument();
    expect(screen.getByText("创建 2026-06-01 01:38:43")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Update the workbench test to fail on richer products-tab expectations**

```tsx
expect(screen.getByText("SPU: spu-123")).toBeInTheDocument();
expect(screen.getByText("货号: J0529021001")).toBeInTheDocument();
expect(screen.getByText("$29.99")).toBeInTheDocument();
expect(screen.getByText("总库存 999")).toBeInTheDocument();
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `npm test -- --run src/components/listingkit/shein-enrollment/shein-synced-products-table.test.tsx src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx`

Expected: FAIL because the current table only renders simple cells.

- [ ] **Step 4: Commit the red tests if your workflow allows red commits; otherwise keep local and proceed immediately**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-synced-products-table.test.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx
git commit -m "test: describe shein dense product list"
```

## Task 3: Implement the Dense Product Table

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-synced-products-table.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-price-snapshot.ts`
- Modify: `web/listingkit-ui/src/lib/types/listingkit/shein-enrollment.ts`
- Reference: `web/listingkit-ui/src/components/ui/table.tsx`

- [ ] **Step 1: Add any missing frontend fields needed by the view**

```ts
export type SheinSyncedProductRecord = {
  id?: number;
  main_image_url?: string;
  product_name_multi?: string;
  spu_name?: string;
  spu_code?: string;
  supplier_code?: string;
  sale_name?: string;
  skc_name?: string;
  skc_code?: string;
  price_snapshot?: string;
  effective_cost_price?: number | null;
  cost_price_source?: "none" | "auto" | "manual" | string;
  inventory_snapshot?: string;
  shelf_status?: string;
  created_at?: string;
  publish_time?: string;
  first_shelf_time?: string;
  last_sync_at?: string;
};
```

- [ ] **Step 2: Refactor the table headings and row structure**

```tsx
<TableHeader>
  <TableRow>
    <TableHead className="w-[320px]">商品信息</TableHead>
    <TableHead className="w-[220px]">主规格</TableHead>
    <TableHead className="w-[120px]">近 7 天销量</TableHead>
    <TableHead className="w-[180px]">价格</TableHead>
    <TableHead className="w-[180px]">库存</TableHead>
    <TableHead className="w-[120px]">上架状态</TableHead>
    <TableHead className="w-[220px]">时间</TableHead>
  </TableRow>
</TableHeader>
```

- [ ] **Step 3: Implement the dense product info cell**

```tsx
<TableCell>
  <div className="flex items-start gap-3">
    <input aria-label={`选择商品 ${item.id ?? "-"}`} type="checkbox" />
    <div className="size-14 overflow-hidden rounded-xl border border-zinc-200 bg-zinc-100">
      {item.main_image_url ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          alt={item.product_name_multi || item.spu_name || "SHEIN 商品"}
          className="h-full w-full object-cover"
          src={item.main_image_url}
        />
      ) : (
        <div className="flex h-full items-center justify-center text-xs text-zinc-400">
          无图
        </div>
      )}
    </div>
    <div className="min-w-0 space-y-1">
      <div className="line-clamp-2 font-medium text-zinc-950">
        {item.product_name_multi || item.spu_name || "-"}
      </div>
      <div className="text-xs text-zinc-500">SPU: {item.spu_code || "-"}</div>
      <div className="text-xs text-zinc-500">货号: {item.supplier_code || "-"}</div>
    </div>
  </div>
</TableCell>
```

- [ ] **Step 4: Implement the variant, sales, price, inventory, status, and time cells**

```tsx
<TableCell>
  <div className="space-y-1">
    <div className="font-medium text-zinc-950">{item.sale_name || "-"}</div>
    <div className="text-xs text-zinc-500">SKC: {item.skc_name || item.skc_code || "-"}</div>
  </div>
</TableCell>
<TableCell>-</TableCell>
<TableCell>
  <div className="space-y-1">
    <div className="font-medium text-zinc-950">
      {formatSheinPriceSnapshot(item.price_snapshot)}
    </div>
    <div className="text-xs text-zinc-500">
      生效成本 {item.effective_cost_price ?? "-"}
    </div>
    <div className="text-xs text-zinc-500">
      来源 {getCostSourceLabel(item.cost_price_source)}
    </div>
  </div>
</TableCell>
<TableCell>
  {(() => {
    const inventory = formatInventorySummary(item.inventory_snapshot);
    return (
      <div className="space-y-1 text-xs text-zinc-500">
        <div>总库存 {inventory.total}</div>
        <div>可用库存 {inventory.available}</div>
        {inventory.raw ? <div className="line-clamp-2">库存摘要 {inventory.raw}</div> : null}
      </div>
    );
  })()}
</TableCell>
<TableCell>
  <span className="inline-flex rounded-full bg-emerald-100 px-2 py-1 text-xs text-emerald-700">
    {item.shelf_status || "-"}
  </span>
</TableCell>
<TableCell>
  <div className="space-y-1 text-xs text-zinc-500">
    {formatProductTimes(item).map(([label, value]) => (
      <div key={label}>
        {label} {value}
      </div>
    ))}
  </div>
</TableCell>
```

- [ ] **Step 5: Run tests to verify the implementation passes**

Run: `npm test -- --run src/components/listingkit/shein-enrollment/shein-synced-products-table.test.tsx src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx src/components/listingkit/shein-enrollment/shein-price-snapshot.test.ts src/components/listingkit/shein-enrollment/shein-product-table-formatters.test.ts`

Expected: PASS

- [ ] **Step 6: Run typecheck**

Run: `npm run typecheck`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-synced-products-table.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-price-snapshot.ts web/listingkit-ui/src/lib/types/listingkit/shein-enrollment.ts web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-synced-products-table.test.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-product-table-formatters.ts web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-product-table-formatters.test.ts
git commit -m "feat: redesign shein product operations list"
```

## Task 4: Browser Verification

**Files:**
- No code changes expected
- Verify: `http://localhost:3000/listing-kits/shein-enrollment/<storeId>?tab=products`

- [ ] **Step 1: Open the product tab in the in-app browser**

Run: open the current local workbench product tab.

Expected: the redesigned table is visible with grouped cells.

- [ ] **Step 2: Verify representative row content**

Check:

- Product thumbnail or placeholder appears
- Product title, SPU, and supplier code render together
- Variant and SKC render together
- Sale price shows symbol formatting
- Effective cost and cost source are both visible
- Inventory block shows parsed values or fallback text
- Multiple timestamps render in the time block

- [ ] **Step 3: Commit only if browser verification required a small polish fix**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-synced-products-table.tsx
git commit -m "fix: polish shein product list layout"
```

## Self-Review

### Spec Coverage

- Dense operations-style row: covered in Task 3
- Existing data-only implementation: covered in Tasks 1 and 3
- Missing-data strategy: covered in Task 1 helpers and Task 3 rendering
- Test coverage: covered in Tasks 1, 2, and 3

No spec gaps found.

### Placeholder Scan

- No `TODO`, `TBD`, or deferred implementation markers remain.
- All tasks contain concrete files, commands, and code snippets.

### Type Consistency

- Formatter helpers operate on existing `SheinSyncedProductRecord` fields.
- Display logic consistently uses `price_snapshot`, `effective_cost_price`, `cost_price_source`, and `inventory_snapshot`.

No naming mismatches found.

Plan complete and saved to `docs/superpowers/plans/2026-06-06-shein-product-list-redesign.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?

