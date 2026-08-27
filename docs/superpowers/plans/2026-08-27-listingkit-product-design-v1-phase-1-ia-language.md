# ListingKit Product Design V1 Phase 1 — Information Architecture & Product Language Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reframe the existing ListingKit authenticated UI from task-oriented language to product-oriented language and reorganize the current navigation without adding dead routes or changing backend APIs.

**Architecture:** Keep all current Next.js routes and query/API contracts intact. Change only the application-shell information hierarchy and the user-facing copy of the existing canonical-product list/detail surfaces; preserve ZITADEL role filtering, current task/workspace links, and task-result-backed read models underneath.

**Tech Stack:** Next.js 16.3.0, React 19.2.4, TypeScript 6.0.3, lucide-react, existing Sidebar/Card/Button/Badge/EmptyState primitives, Vitest 4.1.4, Testing Library.

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
  - Reorganize current business navigation using the existing `NavSection` renderer.
  - Replace the shell tagline `源信息 -> 标准商品 -> 平台资料` with product-oriented copy.
- `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx`
  - Lock down new labels, grouping, route preservation, active state, and role filtering.
- `web/listingkit-ui/src/components/listingkit/canonical/canonical-product-list-page.tsx`
  - Relabel the current standard-product list as `商品中心` and remove task-result/canonical-product engineering copy from primary UI.
- `web/listingkit-ui/src/components/listingkit/canonical/canonical-product-list-page.test.tsx`
  - Add Product Center language, route, and responsive-action assertions.
- `web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.tsx`
  - Relabel the detail page as `商品详情`, hide raw Task ID from the primary card, and rename task-oriented actions.
- `web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.test.tsx`
  - Lock down product-language copy while preserving source lineage and underlying task/workspace routes.

### Files explicitly unchanged in this phase

- `web/listingkit-ui/src/lib/canonical-products/canonical-products.ts`
- `web/listingkit-ui/src/lib/query/use-canonical-products.ts`
- `web/listingkit-ui/src/components/listingkit/workspace/workspace-screen.tsx`
- backend task/workflow/submission code

Do not create a shared label/config module in Phase 1. Only three current UI surfaces need product-language changes, so a new abstraction would add indirection without meaningful reuse yet.

---

### Task 1: Reorganize the application shell around current business surfaces

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx`

**Interfaces:**
- Consumes: current route set, existing `NavItem` / `NavSection` renderer, `MENU_ROLES`, ZITADEL identity.
- Produces: product-oriented navigation labels with unchanged route targets and unchanged role filtering.

- [ ] **Step 1: Rewrite the main shell test expectations before changing production code**

The test suite mocks `usePathname()` as `/listing-kits/sds`. Because `POD` will become a child of the `商品` section, that section should be active and expanded automatically; do not click it closed.

In `renders the main ListingKit workflow navigation`, replace the old primary-navigation assertions with:

```tsx
expect(screen.getByText("ListingKit")).toBeInTheDocument();
expect(screen.getByText("商品 → 平台 → 上架")).toBeInTheDocument();

const sidebarNav = screen.getByRole("navigation", {
  name: "ListingKit 侧边栏导航",
});
expect(sidebarNav).toBeInTheDocument();
expect(screen.getByText("工作")).toBeInTheDocument();
expect(screen.getByText("管理")).toBeInTheDocument();

expect(screen.getByRole("link", { name: "工作台" })).toHaveAttribute(
  "href",
  "/listing-kits/home",
);

const productSection = screen.getByRole("button", { name: "商品" });
expect(productSection).toHaveAttribute("aria-expanded", "true");
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
expect(screen.getByRole("link", { name: "POD" })).toHaveAttribute(
  "aria-current",
  "page",
);
expect(screen.getByRole("link", { name: "款式图库" })).toHaveAttribute(
  "href",
  "/listing-kits/style-gallery",
);
expect(screen.getByRole("link", { name: "执行记录" })).toHaveAttribute(
  "href",
  "/listing-kits",
);
```

Keep the current account, tenant, sidebar layout, and administrator-section assertions in the same test.

- [ ] **Step 2: Update other shell tests for renamed groups without changing permission expectations**

Replace assertions for `管理后台` with `管理`. In tests that expect `POD`, continue using the existing mocked `/listing-kits/sds` pathname so the parent `商品` section is expanded automatically.

The viewer-role test must still contain these assertions:

```tsx
expect(screen.getByRole("button", { name: "商品" })).toBeInTheDocument();
expect(screen.getByRole("link", { name: "POD" })).toBeInTheDocument();
expect(screen.getByText("管理")).toBeInTheDocument();
expect(screen.getByRole("button", { name: "业务运营" })).toBeInTheDocument();
expect(screen.getByRole("button", { name: "账号与系统" })).toBeInTheDocument();
expect(screen.queryByRole("button", { name: "调度与导入" })).not.toBeInTheDocument();
expect(screen.queryByRole("button", { name: "数据字典" })).not.toBeInTheDocument();
expect(screen.queryByRole("button", { name: "策略规则" })).not.toBeInTheDocument();
```

- [ ] **Step 3: Run the shell test and verify the new expectations fail**

From `web/listingkit-ui` run:

```powershell
npm.cmd test -- --run src/components/listingkit/shared/listingkit-app-shell.test.tsx
```

Expected: FAIL because the shell still renders `首页`, `新建任务`, `标准商品`, `任务列表`, `主流程`, `管理后台`, and the old tagline.

- [ ] **Step 4: Replace `PRIMARY_NAV_ITEMS` with a product-oriented current-route tree**

Use the existing `NavSection` rendering and change the constant to:

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

Do not add `上架中心` or `需要处理` because those product surfaces are not implemented in Phase 1.

- [ ] **Step 5: Rename only the top-level shell groups**

Replace:

```ts
const NAV_GROUPS = [
  { label: "主流程", items: PRIMARY_NAV_ITEMS },
  { label: "管理后台", items: ADMIN_NAV_ITEMS },
] as const satisfies readonly { label: string; items: readonly NavTreeItem[] }[];
```

With:

```ts
const NAV_GROUPS = [
  { label: "工作", items: PRIMARY_NAV_ITEMS },
  { label: "管理", items: ADMIN_NAV_ITEMS },
] as const satisfies readonly { label: string; items: readonly NavTreeItem[] }[];
```

Do not reorganize the existing admin sections in this slice.

- [ ] **Step 6: Replace the visible shell tagline**

Replace the current string:

```text
源信息 -> 标准商品 -> 平台资料
```

With:

```text
商品 → 平台 → 上架
```

Keep `ListingKit` unchanged.

- [ ] **Step 7: Run shell tests, typecheck, and focused lint**

```powershell
npm.cmd test -- --run src/components/listingkit/shared/listingkit-app-shell.test.tsx
npm.cmd run typecheck
npm.cmd run lint -- src/components/listingkit/shared/listingkit-app-shell.tsx src/components/listingkit/shared/listingkit-app-shell.test.tsx
```

Expected: shell tests pass; TypeScript reports no errors; focused lint reports no errors.

- [ ] **Step 8: Commit the shell navigation slice**

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

- [ ] **Step 1: Make the existing list fixture report its total and add failing Product Center assertions**

Inside the existing mocked query result, add:

```ts
data: {
  items: [
    {
      taskId: "task-1",
      title: "Canvas Tote",
      brand: "Studio",
      categoryPath: ["Bags"],
      imageUrl: "https://example.com/main.jpg",
      platformLabels: ["shein"],
      needsReview: false,
      imageCount: 3,
      variantCount: 2,
      completedAt: "2026-05-01T00:00:00Z",
      createdAt: "2026-04-30T00:00:00Z",
    },
  ],
  total: 1,
},
```

Add this test:

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

Update the existing responsive assertion to:

```tsx
expect(screen.getByRole("link", { name: "打开商品" })).toHaveClass("w-full");
```

- [ ] **Step 2: Run the list-page test and verify failure**

```powershell
npm.cmd test -- --run src/components/listingkit/canonical/canonical-product-list-page.test.tsx
```

Expected: FAIL because the page still renders `标准商品`, task-result copy, and the `详情` action.

- [ ] **Step 3: Replace the list header with Product Center copy**

Use exactly:

```tsx
<p className="text-[11px] font-semibold uppercase tracking-[0.26em] text-teal-700">
  商品中心
</p>
<h1 className="mt-3 text-3xl font-semibold tracking-tight text-foreground">
  商品中心
</h1>
<p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
  管理 ListingKit 已整理的商品资料，并查看审核与平台准备情况。
</p>
```

Keep the current refresh button unchanged.

- [ ] **Step 4: Replace engineering summary copy with product counts**

Keep the `Card`, `Database` icon, separators, and layout. The text content inside the summary should be:

```tsx
<Database className="h-4 w-4 text-teal-700" />
<span>当前页 {items.length} 个商品</span>
<span className="text-border">/</span>
<span>共 {total} 个商品</span>
```

Remove `来源：ListingKit task result canonical_product` from the rendered page.

- [ ] **Step 5: Convert list loading/error/empty language to business language**

Keep the current loading spinner unchanged.

Use this error state:

```tsx
<EmptyState
  title="商品加载失败"
  description="商品资料暂时无法加载，请稍后重试。"
  action={
    <Button variant="secondary" onClick={() => products.refetch()}>
      <RefreshCw className="mr-2 h-4 w-4" />
      刷新
    </Button>
  }
/>
```

Use this empty state:

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

- [ ] **Step 6: Rename the row action while preserving its route**

Keep:

```tsx
href={`/listing-kits/canonical-products/${item.taskId}`}
```

Change the visible label from `详情` to:

```text
打开商品
```

- [ ] **Step 7: Run list tests, typecheck, and focused lint**

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
- Produces: business-language Product Detail while preserving existing workspace/status route destinations.

- [ ] **Step 1: Add failing Product Detail language assertions**

Add:

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

Change the existing `links back to the task status and platform workspace` assertions to:

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

- [ ] **Step 2: Run the detail test and verify failure**

```powershell
npm.cmd test -- --run src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
```

Expected: FAIL because the page still renders standard-product/task language and the raw Task ID.

- [ ] **Step 3: Replace the detail error state**

Use exactly:

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

- [ ] **Step 4: Replace navigation and header language**

Use this back link:

```tsx
<Link
  href="/listing-kits/canonical-products"
  className="inline-flex w-fit items-center text-sm font-medium text-muted-foreground hover:text-foreground"
>
  <ArrowLeft className="mr-2 h-4 w-4" />
  返回商品中心
</Link>
```

Change the eyebrow from `标准商品详情` to `商品详情`.

Keep the existing route destinations and change only visible action labels:

```tsx
<div className="mt-4 flex flex-wrap gap-3">
  <Button asChild>
    <Link href={detail.data.workspaceHref}>编辑商品</Link>
  </Button>
  <Button asChild variant="outline">
    <Link href={`/listing-kits/${detail.data.taskId}/status`}>查看执行记录</Link>
  </Button>
</div>
```

- [ ] **Step 5: Hide raw Task ID from the primary product card**

Delete this rendered element:

```tsx
<div className="break-all font-mono text-xs text-muted-foreground">
  {detail.data.taskId}
</div>
```

Do not delete `taskId` from the query result or route model because workspace/status navigation still depends on it.

- [ ] **Step 6: Rename review and evidence labels**

Use:

```tsx
<Metric label="需确认字段" value={detail.data.reviewFieldCount} />
```

Replace the review-badge block with:

```tsx
{detail.data.summary.needsReview ? (
  <Badge className="gap-1 rounded-full" variant="warning">
    <ShieldAlert className="mr-1 h-3.5 w-3.5" />
    需要确认
  </Badge>
) : (
  <Badge className="gap-1 rounded-full" variant="success">
    <CheckCircle2 className="mr-1 h-3.5 w-3.5" />
    已校验
  </Badge>
)}
```

Change the field-trace heading to:

```tsx
<h2 className="text-base font-semibold text-foreground">字段依据</h2>
```

Change the empty trace message to `暂无字段依据` and the per-field state labels to `需要确认` and `已校验`.

- [ ] **Step 7: Preserve source-lineage behavior unchanged**

The existing test must continue to pass exactly:

```tsx
expect(screen.getByText("来源 1688 · 888")).toBeInTheDocument();
expect(screen.getByRole("link", { name: "查看来源" })).toHaveAttribute(
  "href",
  "https://detail.1688.com/offer/888.html",
);
```

Do not modify `TaskPersistedSourceReference` in this task.

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
- Produces: regression-checked Phase 1 ready for review before Phase 2 planning.

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

Expected: all UI tests pass. If an unrelated pre-existing failure exists, report the exact failing test and do not modify unrelated code in this phase.

- [ ] **Step 3: Run static validation**

```powershell
npm.cmd run typecheck
npm.cmd run lint
```

Expected: typecheck succeeds and lint reports no new errors introduced by this phase.

- [ ] **Step 4: Scan the three primary surfaces for old engineering-facing phrases**

Run from repository root:

```powershell
git grep -n -E "标准商品|task result canonical_product|查看原任务|进入工作台|源信息 -> 标准商品 -> 平台资料" -- web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-list-page.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.tsx
```

Expected: no matches. Internal symbols such as `CanonicalProductListPage`, `taskId`, and `useCanonicalProductDetail` are intentionally allowed.

- [ ] **Step 5: Review final diff and scope discipline**

```powershell
git diff origin/main...HEAD --stat
git diff origin/main...HEAD -- web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-list-page.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-list-page.test.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
git diff --check
git status --short
```

Expected: only the approved Phase 1 navigation/product-language files and documentation are changed; no backend/API/workflow files are touched.

- [ ] **Step 6: Commit final test-only corrections only if verification required them**

If verification required a legitimate in-scope test correction, stage the three Phase 1 test files that actually changed, run `git diff --cached --check`, and commit with:

```powershell
git commit -m "test: lock ListingKit product language phase one"
```

If verification required no correction, do not create an empty commit.

---

## Phase 1 Acceptance Criteria

Phase 1 is complete only when all of the following are true:

1. The navigation says `工作台`, groups current product entry points under `商品`, and calls the existing task list `执行记录`.
2. No future route is added before its product surface exists.
3. `商品中心` is the user-facing name of the current canonical-product list.
4. Product Center no longer exposes `task result canonical_product` or equivalent internal result-source copy.
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
