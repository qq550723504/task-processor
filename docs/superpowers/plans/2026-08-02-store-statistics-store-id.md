# ListingKit 店铺统计页展示店铺 ID Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在 ListingKit 管理员店铺统计页新增独立的数字“店铺 ID”列，同时保留现有业务店铺标识。

**Architecture:** 复用现有统计接口返回的 `ListingStoreStatistics.id`，仅调整共享的 `StoreStatisticsAdminPage` 表格结构。管理员视图和租户视图共用该组件，因此无需新增数据请求或后端字段。

**Tech Stack:** Next.js 16、React 19、TypeScript、TanStack Query、Vitest、Testing Library、Tailwind CSS。

## Global Constraints

- 不修改统计接口、API schema 或路由；接口已有数字 `id` 和业务 `storeId`。
- “店铺 ID”列展示数字主键 `item.id`；“店铺”列继续展示名称和 `storeId`。
- 保留现有管理员租户信息、租户视图隐藏租户 ID、响应式横向滚动行为。
- 保留用户现有未提交的 `scripts/migrate-yudao-users-to-zitadel.ps1` 修改，不要将其加入本次提交。

---

### Task 1: Add regression coverage for the store ID column

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/admin/store-statistics-admin-page.test.tsx`

**Interfaces:**
- Consumes: `StoreStatisticsAdminPage` and the existing mocked `getListingStoreStatistics` response.
- Produces: A regression assertion proving a rendered row exposes the numeric store primary key separately from the business `storeId`.

- [ ] **Step 1: Write the failing assertion**

In the existing `loads and renders ListingKit store statistics` test, after the row has loaded, assert that the new column header and numeric ID are visible while the business ID remains visible:

```tsx
expect(screen.getByRole("columnheader", { name: "店铺 ID" })).toBeInTheDocument();
expect(screen.getByText("1", { selector: "td" })).toBeInTheDocument();
expect(screen.getByText("SHEIN-US")).toBeInTheDocument();
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run from `web/listingkit-ui`:

```powershell
npm.cmd test -- src/components/listingkit/admin/store-statistics-admin-page.test.tsx
```

Expected: the existing page tests start, but the new `columnheader` assertion fails because no “店铺 ID” column exists yet.

### Task 2: Render the numeric store ID column and verify the UI

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/admin/store-statistics-admin-page.tsx:127-151`
- Test: `web/listingkit-ui/src/components/listingkit/admin/store-statistics-admin-page.test.tsx`

**Interfaces:**
- Consumes: `ListingStoreStatistics.id: number` and the existing table row rendering.
- Produces: A seven-column table with a visible “店铺 ID” cell for each statistics row.

- [ ] **Step 1: Add the table header and row cell**

Insert `<TableHead>店铺 ID</TableHead>` immediately after the existing `<TableHead>店铺</TableHead>`. Insert a matching `<TableCell className="font-mono text-zinc-700">{item.id}</TableCell>` immediately after the store-name cell in each row.

- [ ] **Step 2: Update empty/loading row spans**

Change both loading and empty-state `TableCell` values from `colSpan={6}` to `colSpan={7}` so the state rows cover the new table width.

- [ ] **Step 3: Run the focused component test to verify it passes**

Run:

```powershell
npm.cmd test -- src/components/listingkit/admin/store-statistics-admin-page.test.tsx
```

Expected: all tests in the file pass, including the new numeric ID assertion and the existing tenant/mobile-view coverage.

- [ ] **Step 4: Run the API parser regression test**

Run:

```powershell
npm.cmd test -- src/lib/api/admin-store-statistics.test.ts
```

Expected: all API parsing and proxy request tests pass, confirming no API contract changes were introduced.

- [ ] **Step 5: Run TypeScript validation**

Run:

```powershell
npm.cmd run typecheck
```

Expected: TypeScript exits with code 0 and reports no errors.

- [ ] **Step 6: Review the diff and commit only the requested UI/test files**

Run:

```powershell
git diff --check -- web/listingkit-ui/src/components/listingkit/admin/store-statistics-admin-page.tsx web/listingkit-ui/src/components/listingkit/admin/store-statistics-admin-page.test.tsx
git status --short
git diff -- web/listingkit-ui/src/components/listingkit/admin/store-statistics-admin-page.tsx web/listingkit-ui/src/components/listingkit/admin/store-statistics-admin-page.test.tsx
```

Confirm the diff contains only the new header, ID cell, `colSpan` updates, and regression assertion; then commit only these two files:

```powershell
git add -- web/listingkit-ui/src/components/listingkit/admin/store-statistics-admin-page.tsx web/listingkit-ui/src/components/listingkit/admin/store-statistics-admin-page.test.tsx
git commit -m "feat: show store IDs in statistics"
```
