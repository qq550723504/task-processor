# ListingKit Product Design V1 Phase 1 — Information Architecture & Product Language Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reframe the existing ListingKit authenticated UI from task-oriented language to product-oriented language and reorganize the current navigation without adding dead routes or changing backend APIs.

**Architecture:** Keep all current Next.js routes and query/API contracts intact. Change only the application-shell information hierarchy and the user-facing copy of the existing canonical-product list/detail surfaces; preserve ZITADEL role filtering, current task/workspace links, and task-result-backed read models underneath.

**Tech Stack:** Next.js 16.3.0, React 19.2.4, TypeScript 6.0.3, lucide-react, existing Sidebar/Card/Button/Badge primitives, Vitest 4.1.4, Testing Library.

**Spec:** `docs/superpowers/specs/2026-08-27-listingkit-product-design-v1.md`

## Global Constraints

- Do not add `/listing-center`, `/exceptions`, `/products/:productId`, or any other future route in this phase.
- Preserve the existing routes `/listing-kits/home`, `/listing-kits/new`, `/listing-kits/sds`, `/listing-kits/style-gallery`, `/listing-kits/canonical-products`, and `/listing-kits`.
- Preserve existing canonical-product APIs/read models and the current `taskId`-backed product detail route.
- Preserve ZITADEL role checks and role visibility behavior.
- Use product/business language in primary UI; task/workflow language may remain only in execution-oriented secondary destinations.
- Do not change Temporal actions, task execution, submission behavior, queue semantics, database schemas, or backend endpoints.
- Do not create placeholder links for Product Center features that are not implemented yet.
- Keep changes incremental and independently testable.

---

## File Structure

### Files to modify

- `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx`
  - Reorganize the existing navigation into product-oriented groups while preserving all current routes and permission rules.
  - Replace the shell tagline `源信息 -> 标准商品 -> 平台资料` with product-oriented copy.
- `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx`
  - Lock down the new labels, grouping, route preservation, active state, and role filtering.
- `web/listingkit-ui/src/components/listingkit/canonical/canonical-product-list-page.tsx`
  - Relabel the current standard-product list as `商品中心` and remove task-result/canonical-product engineering copy from the primary UI.
- `web/listingkit-ui/src/components/listingkit/canonical/canonical-product-list-page.test.tsx`
  - Add product-language, route, and responsive-action assertions.
- `web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.tsx`
  - Relabel the current detail page as a product detail surface, hide the raw Task ID from the primary card, and rename task-oriented actions.
- `web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.test.tsx`
  - Lock down product-language copy while preserving source lineage and underlying task/workspace routes.

### Files explicitly unchanged in this phase

- `web/listingkit-ui/src/lib/canonical-products/canonical-products.ts`
- `web/listingkit-ui/src/lib/query/use-canonical-products.ts`
- `web/listingkit-ui/src/components/listingkit/workspace/workspace-screen.tsx`
- backend task/workflow/submission code

The implementation should not create a new shared label module in this phase. The affected copy is limited to three existing UI surfaces, and a new abstraction would add indirection without reuse value yet.

---

### Task 1: Reorganize the application shell around current business surfaces

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx`

**Interfaces:**
- Consumes: current route set, existing `NavItem` / `NavSection` renderer, current `MENU_ROLES`, current ZITADEL identity.
- Produces: product-oriented navigation labels with unchanged route targets and unchanged role filtering.

- [ ] **Step 1: Rewrite the primary-navigation test expectations before changing the shell**

Replace the assertions in `renders the main ListingKit workflow navigation` that currently expect `主流程`, `首页`, `新建任务`, `标准商品`, and `任务列表` with the following product-language expectations:

```tsx
expect(screen.getByText("工作")).toBeInTheDocument();
expect(screen.getByText("管理")).toBeInTheDocument();
expect(screen.getByRole("link", { name: "工作台" })).toHaveAttribute(
  "href",
  "/listing-kits/home",
);
expect(screen.getByRole("button", { name: "商品" })).toBeInTheDocument();
expect(screen.getByRole("link", { name: "执行记录" })).toHaveAttribute(
  "href",
  "/listing-kits",
);
expect(screen.getByText("商品 → 平台 → 上架")).toBeInTheDocument();
```

Then add expansion assertions for the new `商品` section:

```tsx
await user.click(screen.getByRole("button", { name: "商品" }));

expect(screen.getByRole("link", { name: "商品中心" })).toHaveAttribute(
  "href",
  "/listing-kits/canonical-products",
);
expect(screen.getByRole("link", { name: "导入商品" })).toHaveAttribute(
  "href",
  "/listing-kits/new",
);
expect(screen.getByRole("link", { name: "POD" })).toHaveAttribute(
  "href",
  "/listing-kits/sds",
);
expect(screen.getByRole("link", { name: "款式图库" })).toHaveAttribute(
  "href",
  "/listing-kits/style-gallery",
);
```

Keep the existing administrator/identity assertions in the same test.

- [ ] **Step 2: Update the active-navigation test for the current mocked `/listing-kits/sds` path**

Change the active-state assertion so the parent product section and the POD child are both represented correctly after expansion:

```tsx
const productSection = screen.getByRole("button", { name: "商品" });
expect(productSection).toHaveAttribute("aria-expanded", "true");
expect(screen.getByRole("link", { name: "POD" })).toHaveAttribute(
  "aria-current",
  "page",
);
```

Do not change the mocked pathname; `/listing-kits/sds` remains the regression case for nested active-state behavior.

- [ ] **Step 3: Run the shell test and verify the new expectations fail**

From `web/listingkit-ui` run:

```powershell
npm.cmd test -- --run src/components/listingkit/shared/listingkit-app-shell.test.tsx
```

Expected: FAIL because the shell still renders `首页`, `新建任务`, `标准商品`, `任务列表`, `主流程`, and the old tagline.

- [ ] **Step 4: Replace `PRIMARY_NAV_ITEMS` with a product-oriented current-route tree**

In `listingkit-app-shell.tsx`, use the existing `NavSection` rendering rather than introducing a new navigation component. The current business navigation should be:

```ts
const PRIMARY_NAV_ITEMS = [
  { label: "工作台", href: "/listing-kits/home", icon: Home, match: "exact" },
  {
    label: "商品",
    icon: PackageCheck,
    children: [
      {
        label: "商品中心",
        href: "/listing-kits/canonical-products",
        icon: PackageCheck,
        match: "prefix",
      },
      {
        label: "导入商品",
        href: "/listing-kits/new",
        icon: PackagePlus,
        match: "exact",
      },
      { label: "POD", href: "/listing-kits/sds", icon: Boxes, match: "prefix" },
      {
        label: "款式图库",
        href: "/listing-kits/style-gallery",
        icon: GalleryHorizontal,
        match: "prefix",
      },
    ],
  },
  {
    label: "执行记录",
    href: "/listing-kits",
    icon: ClipboardList,
    match: "exact",
  },
] as const satisfies readonly NavTreeItem[];
```

Do not add `上架中心` or `需要处理` yet because those routes do not exist in Phase 1.

- [ ] **Step 5: Rename only the top-level shell group labels**

Change:

```ts
const NAV_GROUPS = [
  { label: "主流程", items: PRIMARY_NAV_ITEMS },
  { label: "管理后台", items: ADMIN_NAV_ITEMS },
] as const;
```

To:

```ts
const NAV_GROUPS = [
  { label: "工作", items: PRIMARY_NAV_ITEMS },
  { label: "管理", items: ADMIN_NAV_ITEMS },
] as const;
```

Do not restructure the existing admin sections in this slice. Their route/role behavior is already covered and the Product Design V1 spec does not require admin-screen redesign in Phase 1.

- [ ] **Step 6: Replace the shell tagline with product language**

Find the current visible shell subtitle `源信息 -> 标准商品 -> 平台资料` and replace it with:

```tsx
<span>商品 → 平台 → 上架</span>
```

Keep the `ListingKit` brand label unchanged.

- [ ] **Step 7: Update role-filtering tests to use the new group labels without changing permission behavior**

Where shell tests assert `管理后台`, change them to `管理`. Where they assert `POD` as a top-level link, first expand `商品` and then assert `POD`.

The viewer-role test must continue to prove:

```tsx
expect(screen.getByText("管理")).toBeInTheDocument();
expect(screen.getByRole("button", { name: "业务运营" })).toBeInTheDocument();
expect(screen.getByRole("button", { name: "账号与系统" })).toBeInTheDocument();
expect(screen.queryByRole("button", { name: "调度与导入" })).not.toBeInTheDocument();
expect(screen.queryByRole("button", { name: "数据字典" })).not.toBeInTheDocument();
expect(screen.queryByRole("button", { name: "策略规则" })).not.toBeInTheDocument();
```

- [ ] **Step 8: Run shell tests, typecheck, and focused lint**

```powershell
npm.cmd test -- --run src/components/listingkit/shared/listingkit-app-shell.test.tsx
npm.cmd run typecheck
npm.cmd run lint -- src/components/listingkit/shared/listingkit-app-shell.tsx src/components/listingkit/shared/listingkit-app-shell.test.tsx
```

Expected: shell tests pass; TypeScript reports no errors; focused lint reports no errors.

- [ ] **Step 9: Commit the shell navigation slice**

```powershell
git add web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx
git diff --cached --check
git commit -m "feat: reframe ListingKit navigation around products"
```

---

### Task 2: Relabel the canonical-product list as Product Center

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/canonical/canonical-product-list-page.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/canonical/canonical-product-list-page.test.tsx`

**Interfaces:**
- Consumes: unchanged `useCanonicalProducts({ page, page_size: 30 })` query and `CanonicalProductListItem` shape.
- Produces: Product Center language while preserving pagination, row links, review badges, platform badges, and responsive actions.

- [ ] **Step 1: Add failing Product Center language assertions**

Extend `canonical-product-list-page.test.tsx` with:

```tsx
it("presents canonical products as the Product Center without execution-language copy", () => {
  render(<CanonicalProductListPage />);

  expect(screen.getByRole("heading", { name: "商品中心" })).toBeInTheDocument();
  expect(
    screen.getByText("管理 ListingKit 已整理的商品资料，并查看审核与平台准备情况。"),
  ).toBeInTheDocument();
  expect(screen.getByText("当前页 1 个商品")).toBeInTheDocument();
  expect(screen.getByText("共 1 个商品")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "打开商品" })).toHaveAttribute(
    "href",
    "/listing-kits/canonical-products/task-1",
  );
  expect(screen.queryByText(/canonical_product/i)).not.toBeInTheDocument();
  expect(screen.queryByText(/task result/i)).not.toBeInTheDocument();
});
```

Keep the existing narrow-layout test but rename the queried row action from `详情` to `打开商品`:

```tsx
expect(screen.getByRole("link", { name: "打开商品" })).toHaveClass("w-full");
```

- [ ] **Step 2: Run the focused list-page test and verify failure**

```powershell
npm.cmd test -- --run src/components/listingkit/canonical/canonical-product-list-page.test.tsx
```

Expected: FAIL because the page still renders `标准商品`, task-result copy, and the `详情` action.

- [ ] **Step 3: Replace the list-page header copy**

Change the eyebrow and H1 from `标准商品` to `商品中心` and replace the description with exactly:

```tsx
<p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
  管理 ListingKit 已整理的商品资料，并查看审核与平台准备情况。
</p>
```

Keep the refresh button and its current behavior unchanged.

- [ ] **Step 4: Replace engineering summary copy with product counts**

Keep the summary Card and `Database` icon, but render only:

```tsx
<span>当前页 {items.length} 个商品</span>
<span className="text-border">/</span>
<span>共 {total} 个商品</span>
```

Remove the visible string:

```text
来源：ListingKit task result canonical_product
```

Do not remove source data from the model; this step removes only engineering copy from the list UI.

- [ ] **Step 5: Convert list error/empty state copy to business language**

Use:

```tsx
<EmptyState
  title="商品加载失败"
  description="商品资料暂时无法加载，请稍后重试。"
  action={...existing refresh button...}
/>
```

And for the empty state:

```tsx
<EmptyState
  title="你的商品中心还是空的"
  description="导入第一个商品，ListingKit 会自动整理图片、属性、SKU 和平台所需资料。"
  action={
    <Link
      className="text-sm font-medium text-foreground underline"
      href="/listing-kits/new"
    >
      导入商品
    </Link>
  }
/>
```

The empty-state CTA must use the existing `/listing-kits/new` route rather than linking to the task list.

- [ ] **Step 6: Rename the row action only; preserve the detail route**

Change:

```tsx
详情
```

To:

```tsx
打开商品
```

Keep:

```tsx
href={`/listing-kits/canonical-products/${item.taskId}`}
```

unchanged in Phase 1.

- [ ] **Step 7: Run focused tests, typecheck, and lint**

```powershell
npm.cmd test -- --run src/components/listingkit/canonical/canonical-product-list-page.test.tsx
npm.cmd run typecheck
npm.cmd run lint -- src/components/listingkit/canonical/canonical-product-list-page.tsx src/components/listingkit/canonical/canonical-product-list-page.test.tsx
```

Expected: tests pass; TypeScript and focused lint succeed.

- [ ] **Step 8: Commit the Product Center language slice**

```powershell
git add web/listingkit-ui/src/components/listingkit/canonical/canonical-product-list-page.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-list-page.test.tsx
git diff --cached --check
git commit -m "feat: present canonical products as Product Center"
```

---

### Task 3: Reframe canonical-product detail as Product Detail

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.test.tsx`

**Interfaces:**
- Consumes: unchanged `useCanonicalProductDetail(taskId)` result including `workspaceHref`, `taskId`, source lineage, product, summary, and field traces.
- Produces: business-language Product Detail while preserving the existing workspace/status route destinations for implementation compatibility.

- [ ] **Step 1: Add failing Product Detail language assertions**

Add a new test:

```tsx
it("uses product language and keeps execution identity secondary", () => {
  render(<CanonicalProductDetailPage taskId="task-1" />);

  expect(screen.getByRole("link", { name: "返回商品中心" })).toHaveAttribute(
    "href",
    "/listing-kits/canonical-products",
  );
  expect(screen.getByText("商品详情")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "编辑商品" })).toHaveAttribute(
    "href",
    "/listing-kits/task-1/workspace?platform=shein",
  );
  expect(screen.getByRole("link", { name: "查看执行记录" })).toHaveAttribute(
    "href",
    "/listing-kits/task-1/status",
  );
  expect(screen.getByText("需确认字段")).toBeInTheDocument();
  expect(screen.getByText("已校验")).toBeInTheDocument();
  expect(screen.getByText("字段依据")).toBeInTheDocument();
  expect(screen.queryByText("task-1")).not.toBeInTheDocument();
  expect(screen.queryByText(/canonical_product/i)).not.toBeInTheDocument();
});
```

Update the existing navigation test to use the new labels:

```tsx
expect(screen.getByRole("link", { name: "查看执行记录" })).toHaveAttribute(
  "href",
  "/listing-kits/task-1/status",
);
expect(screen.getByRole("link", { name: "编辑商品" })).toHaveAttribute(
  "href",
  "/listing-kits/task-1/workspace?platform=shein",
);
```

- [ ] **Step 2: Run the detail-page test and verify failure**

```powershell
npm.cmd test -- --run src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
```

Expected: FAIL because the page still renders standard-product/task language and the raw Task ID.

- [ ] **Step 3: Replace error-state engineering copy**

Change the error state to:

```tsx
<EmptyState
  title="未找到商品"
  description="商品资料不存在，或当前暂时无法加载。"
  action={
    <Link
      href="/listing-kits/canonical-products"
      className="text-sm font-medium text-foreground underline"
    >
      返回商品中心
    </Link>
  }
/>
```

- [ ] **Step 4: Replace detail navigation and header copy**

Use:

```tsx
<Link href="/listing-kits/canonical-products" ...>
  <ArrowLeft className="mr-2 h-4 w-4" />
  返回商品中心
</Link>
```

Change the eyebrow from `标准商品详情` to `商品详情`.

Rename the actions while preserving their hrefs:

```tsx
<Button asChild>
  <Link href={detail.data.workspaceHref}>编辑商品</Link>
</Button>
<Button asChild variant="outline">
  <Link href={`/listing-kits/${detail.data.taskId}/status`}>查看执行记录</Link>
</Button>
```

- [ ] **Step 5: Hide Task ID from the primary product card**

Delete this primary-surface element:

```tsx
<div className="break-all font-mono text-xs text-muted-foreground">
  {detail.data.taskId}
</div>
```

Do not delete `taskId` from the query result or route model; it is still needed for workspace/status navigation.

- [ ] **Step 6: Rename review and trace labels**

Change the summary metric:

```tsx
<Metric label="需确认字段" value={detail.data.reviewFieldCount} />
```

Change review badges:

```tsx
{detail.data.summary.needsReview ? (
  <Badge ... variant="warning">需要确认</Badge>
) : (
  <Badge ... variant="success">已校验</Badge>
)}
```

Change the trace heading:

```tsx
<h2 className="text-base font-semibold text-foreground">字段依据</h2>
```

Change empty trace copy from `暂无字段追踪` to `暂无字段依据` and trace state labels from `需审核 / 可信` to `需要确认 / 已校验`.

- [ ] **Step 7: Preserve source-lineage behavior unchanged**

Do not alter `TaskPersistedSourceReference` or the existing source link. The existing test must continue to pass:

```tsx
expect(screen.getByText("来源 1688 · 888")).toBeInTheDocument();
expect(screen.getByRole("link", { name: "查看来源" })).toHaveAttribute(
  "href",
  "https://detail.1688.com/offer/888.html",
);
```

- [ ] **Step 8: Run detail tests, typecheck, and focused lint**

```powershell
npm.cmd test -- --run src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
npm.cmd run typecheck
npm.cmd run lint -- src/components/listingkit/canonical/canonical-product-detail-page.tsx src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
```

Expected: tests pass; source lineage is unchanged; TypeScript and lint succeed.

- [ ] **Step 9: Commit the Product Detail language slice**

```powershell
git add web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
git diff --cached --check
git commit -m "feat: reframe canonical detail as product detail"
```

---

### Task 4: Verify Phase 1 as one coherent product-language slice

**Files:**
- No new production files.
- Verify all files modified in Tasks 1–3.

**Interfaces:**
- Consumes: shell navigation, Product Center copy, Product Detail copy.
- Produces: a regression-checked Phase 1 ready for review before Phase 2 planning/execution.

- [ ] **Step 1: Run the focused regression suite together**

From `web/listingkit-ui`:

```powershell
npm.cmd test -- --run src/components/listingkit/shared/listingkit-app-shell.test.tsx src/components/listingkit/canonical/canonical-product-list-page.test.tsx src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
```

Expected: all focused tests pass.

- [ ] **Step 2: Run the full ListingKit UI test suite**

```powershell
npm.cmd test -- --maxWorkers=4
```

Expected: all UI tests pass. If an unrelated pre-existing failure exists, report it with the exact failing test and do not modify unrelated code in this phase.

- [ ] **Step 3: Run static validation**

```powershell
npm.cmd run typecheck
npm.cmd run lint
```

Expected: typecheck succeeds and lint reports no new errors introduced by this phase.

- [ ] **Step 4: Scan the three primary surfaces for prohibited Phase 1 engineering language**

Run from repository root:

```powershell
git grep -n -E "标准商品|task result canonical_product|查看原任务|进入工作台|源信息 -> 标准商品 -> 平台资料" -- web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-list-page.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.tsx
```

Expected: no matches for those old primary-UI phrases. Internal symbols such as `CanonicalProductListPage`, `taskId`, and `useCanonicalProductDetail` are intentionally allowed.

- [ ] **Step 5: Review the final diff for scope discipline**

```powershell
git diff origin/main...HEAD --stat
git diff origin/main...HEAD -- web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-list-page.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-list-page.test.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
git diff --check
git status --short
```

Expected: only the approved Phase 1 navigation/product-language files and documentation are changed; no backend/API/workflow files are touched.

- [ ] **Step 6: Commit any final test-only adjustments if required**

If Step 1–5 required test expectation adjustments that do not change scope, commit them separately:

```powershell
git add web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-list-page.test.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
git diff --cached --check
git commit -m "test: lock ListingKit product language phase one"
```

Skip this commit if no final adjustment was needed.

---

## Phase 1 Acceptance Criteria

Phase 1 is complete only when all of the following are true:

1. The current navigation says `工作台`, groups product entry points under `商品`, and calls the existing task list `执行记录`.
2. No future route is added before its product surface exists.
3. `商品中心` is the user-facing name of the current canonical-product list.
4. Product Center no longer exposes `task result canonical_product` or other internal result-source copy.
5. Product Detail says `商品详情`, `编辑商品`, `查看执行记录`, and does not show the raw Task ID in the primary product card.
6. Existing source lineage, platform workspace links, task-status links, pagination, responsive behavior, role filtering, and active navigation continue to work.
7. No backend API, Temporal workflow, database model, or canonical read-model contract changes are required.
8. Focused tests, full tests, typecheck, lint, and diff checks complete successfully or any unrelated pre-existing failure is explicitly documented.

## Follow-on Plans

Do not fold the following work into this plan. After Phase 1 is implemented and reviewed, write separate implementation plans for:

1. **Phase 2 — Workbench + Product Center V2**
   - operational homepage
   - Product Center table-first layout
   - lifecycle/review presentation models
   - filtering and bulk actions that existing APIs can support
2. **Phase 3 — Product Workspace composition**
   - three-column shell
   - canonical/platform section navigation
   - contextual AI Review rail
   - execution/revision details moved to secondary surfaces
3. **Phase 4 — Listing Center + Exception Center**
   - Listing read model
   - platform/store publication states
   - human-in-the-loop issue aggregation and recovery
4. **Phase 5 — Stable Product identity**
   - only after the UI model is validated and backend migration is justified
