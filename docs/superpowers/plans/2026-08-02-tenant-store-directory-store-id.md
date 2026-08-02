# ListingKit 租户店铺页展示店铺 ID Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在 `/listing-kits/stores` 的租户店铺主数据表格中新增独立的数字“店铺 ID”列，并保留业务店铺标识。

**Architecture:** 复用 `TenantStoreDirectoryPanel` 已加载的 `ListingStore` 数据，不改后端接口或表单逻辑。表格将数字主键放进独立列，名称列改为显示业务 `storeId`，管理员可直接按数字 ID 定位店铺且不会重复显示同一 ID。

**Tech Stack:** Next.js 16、React 19、TypeScript、TanStack Query、Vitest、Testing Library、Tailwind CSS。

## Global Constraints

- 不修改租户店铺 API、API schema、路由、筛选、登录、增删改或表单行为。
- “店铺 ID”列展示数字主键 `store.id`；名称列下方展示业务 `storeId`（有值时）。
- 移除名称列中原有的 `#store.id`，避免数字主键重复展示。
- 加载和空数据状态的 `colSpan` 必须覆盖新增后的 8 列。
- 保留用户现有未提交的 `scripts/migrate-yudao-users-to-zitadel.ps1` 修改，不要将其加入本次提交。
- 实现完成后更新已存在的 Draft PR #56，不创建新的 PR。

---

### Task 1: Add regression coverage for the tenant store ID column

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/stores/tenant-store-directory-panel.test.tsx`

**Interfaces:**
- Consumes: `TenantStoreDirectoryPanel` and the existing mocked `getTenantListingStores` row containing `id: 1` and `storeId: "SHEIN-US-001"`.
- Produces: A regression assertion proving the table has a “店铺 ID” column, renders numeric ID `1`, and still renders business ID `SHEIN-US-001`.

- [ ] **Step 1: Write the failing assertions**

In `renders tenant store list`, after the existing name and username assertions, add:

```tsx
expect(
  screen.getByRole("columnheader", { name: "店铺 ID" }),
).toBeInTheDocument();
expect(screen.getByText("1", { selector: "td" })).toBeInTheDocument();
expect(screen.getByText("SHEIN-US-001")).toBeInTheDocument();
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run from `web/listingkit-ui`:

```powershell
npm.cmd test -- src/components/listingkit/stores/tenant-store-directory-panel.test.tsx
```

Expected: the test fails at the missing “店铺 ID” column assertion because the current table has no dedicated ID header.

### Task 2: Render the tenant store ID column and update PR #56

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/stores/tenant-store-directory-panel.tsx:208-244`
- Test: `web/listingkit-ui/src/components/listingkit/stores/tenant-store-directory-panel.test.tsx`

**Interfaces:**
- Consumes: `ListingStore.id: number` and optional `ListingStore.storeId: string` from the existing tenant store query.
- Produces: An eight-column tenant store table with a dedicated numeric ID cell and no duplicated inline numeric ID.

- [ ] **Step 1: Add the header and row cell**

Insert `<TableHead className="px-4 py-3">店铺 ID</TableHead>` after the existing “店铺” header. In each row, replace the inline `#{store.id}` element with `{store.storeId || "-"}`, then insert a numeric ID cell after the store-name cell:

```tsx
<TableCell className="px-4 py-3 font-mono text-zinc-700">
  {store.id}
</TableCell>
```

- [ ] **Step 2: Update loading and empty-state spans**

Change both tenant table state cells from `colSpan={7}` to `colSpan={8}`.

- [ ] **Step 3: Run the focused component test to verify it passes**

Run:

```powershell
npm.cmd test -- src/components/listingkit/stores/tenant-store-directory-panel.test.tsx
```

Expected: all tests in the file pass, including the new ID assertions and existing login-status, form, and mobile-layout coverage.

- [ ] **Step 4: Run the relevant API and TypeScript checks**

Run:

```powershell
npm.cmd test -- src/lib/api/admin-stores.test.ts src/lib/api/tenant-stores.test.ts
npm.cmd run typecheck
```

Expected: both API test files pass and TypeScript exits with code 0.

- [ ] **Step 5: Review, commit, push, and verify Draft PR #56**

Run:

```powershell
git diff --check -- web/listingkit-ui/src/components/listingkit/stores/tenant-store-directory-panel.tsx web/listingkit-ui/src/components/listingkit/stores/tenant-store-directory-panel.test.tsx
git add -- web/listingkit-ui/src/components/listingkit/stores/tenant-store-directory-panel.tsx web/listingkit-ui/src/components/listingkit/stores/tenant-store-directory-panel.test.tsx
git commit -m "feat: show store IDs in tenant store list"
git push
gh pr view 56 --repo qq550723504/task-processor --json url,isDraft,headRefOid,baseRefName
```

Confirm only the two requested UI files are staged, the push succeeds, and PR #56 points to the new commit.
