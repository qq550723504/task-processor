# ListingKit SDS Split Homepage Route Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the SDS experience into a lightweight homepage, a dedicated new-batch selection route, and a dedicated batch editor route so `/listing-kits/sds` stops acting like a mixed dashboard-plus-selection page.

**Architecture:** Keep the current data model and editor behavior, but redistribute responsibilities across three routes. Reuse existing recent-batch summary logic on the homepage, move the SDS product browser into `/listing-kits/sds/new`, and mount the existing single-batch workbench behind `/listing-kits/sds/batches/:id`.

**Tech Stack:** Next.js App Router, React, TypeScript, existing ListingKit SDS/Shein Studio components, Vitest

---

## File Structure

### Existing files to modify

- `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\page.tsx`
  - reduce this route to homepage composition only
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein-studio\shein-studio-page-shell.tsx`
  - split page-shell responsibilities so homepage mode does not always render the product browser
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein-studio\shein-studio-workbench.tsx`
  - support mounting in dedicated batch route without relying on homepage-first composition
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein-studio\shein-studio-recent-batches-dashboard.tsx`
  - support “show only 3 by default” and dedicated open/new route navigation
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\sds\sds-product-browser.tsx`
  - move into dedicated new-batch page composition
- `D:\code\task-processor\web\listingkit-ui\src\lib\utils\shein-studio-batches.ts`
  - reuse or extend batch-route loading helpers

### New files to create

- `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\new\page.tsx`
  - dedicated new-batch route
- `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\batches\[batchId]\page.tsx`
  - dedicated batch editor route
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\sds\sds-homepage-entry.tsx`
  - focused homepage CTA block
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\sds\sds-new-batch-shell.tsx`
  - focused route-level wrapper for the product browser
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein-studio\shein-studio-batch-page-shell.tsx`
  - route-level wrapper for dedicated batch editor mounting

### Tests to modify or create

- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein-studio\shein-studio-page-shell.test.tsx`
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein-studio\shein-studio-recent-batches-dashboard.test.tsx`
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein-studio\shein-studio-workbench.test.tsx`
- `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\page.test.tsx`
- `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\new\page.test.tsx`
- `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\batches\[batchId]\page.test.tsx`

## Task 1: Add failing route-level homepage tests

**Files:**
- Modify: `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\page.test.tsx`
- Test: `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\page.test.tsx`

- [ ] **Step 1: Write the failing homepage expectations**

Add tests asserting:

- `/listing-kits/sds` shows entry CTA copy
- homepage does not render the SDS product browser by default
- homepage shows only 3 recent batches before expansion

Use test structure like:

```tsx
it("renders the SDS homepage without the product browser", async () => {
  render(<SdsPage />);

  expect(
    screen.getByRole("heading", { name: "最近批次" }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("button", { name: "新建批次并选品" }),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("heading", { name: "选择底版商品和子 SKU" }),
  ).not.toBeInTheDocument();
});
```

- [ ] **Step 2: Run the homepage test to verify it fails**

Run:

```bash
cd D:\code\task-processor\web\listingkit-ui
npm test -- src/app/listing-kits/sds/page.test.tsx
```

Expected: FAIL because the current route still renders mixed homepage-plus-product-browser content.

- [ ] **Step 3: Commit the failing test**

```bash
git add web/listingkit-ui/src/app/listing-kits/sds/page.test.tsx
git commit -m "test: define SDS homepage-only route expectations"
```

## Task 2: Add new route stubs and their failing tests

**Files:**
- Create: `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\new\page.tsx`
- Create: `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\new\page.test.tsx`
- Create: `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\batches\[batchId]\page.tsx`
- Create: `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\batches\[batchId]\page.test.tsx`

- [ ] **Step 1: Write the failing test for the new-batch route**

Use a test like:

```tsx
it("renders the dedicated new-batch selection route", async () => {
  render(<SdsNewPage />);

  expect(
    screen.getByRole("heading", { name: "选择底版商品和子 SKU" }),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("heading", { name: "最近批次" }),
  ).not.toBeInTheDocument();
});
```

- [ ] **Step 2: Write the failing test for the batch editor route**

Use a test like:

```tsx
it("renders the dedicated batch editor route", async () => {
  render(<SdsBatchPage params={{ batchId: "batch-1" }} />);

  expect(
    screen.getByText(/已载入批次/i),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("heading", { name: "最近批次" }),
  ).not.toBeInTheDocument();
});
```

- [ ] **Step 3: Run the route tests to verify they fail**

Run:

```bash
cd D:\code\task-processor\web\listingkit-ui
npm test -- src/app/listing-kits/sds/new/page.test.tsx src/app/listing-kits/sds/batches/[batchId]/page.test.tsx
```

Expected: FAIL because the routes do not exist or do not yet render the dedicated layouts.

- [ ] **Step 4: Commit the failing route tests**

```bash
git add web/listingkit-ui/src/app/listing-kits/sds/new/page.test.tsx web/listingkit-ui/src/app/listing-kits/sds/batches/[batchId]/page.test.tsx
git commit -m "test: define dedicated SDS new and batch route expectations"
```

## Task 3: Implement the lightweight SDS homepage route

**Files:**
- Modify: `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\page.tsx`
- Create: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\sds\sds-homepage-entry.tsx`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein-studio\shein-studio-page-shell.tsx`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein-studio\shein-studio-recent-batches-dashboard.tsx`

- [ ] **Step 1: Create the homepage entry component**

Implement a focused CTA component like:

```tsx
type SdsHomepageEntryProps = {
  onContinueRecent: () => void;
  onCreateNew: () => void;
  recommendedRiskLabel?: string | null;
};

export function SdsHomepageEntry({
  onContinueRecent,
  onCreateNew,
  recommendedRiskLabel,
}: SdsHomepageEntryProps) {
  return (
    <section>
      <p>POD</p>
      <h1>从 POD 商品生成上架资料</h1>
      <p>先继续最近批次，或新建一个批次再开始选品。</p>
      <button onClick={onContinueRecent}>
        {recommendedRiskLabel
          ? `继续最近批次（优先处理 ${recommendedRiskLabel}）`
          : "继续最近批次"}
      </button>
      <button onClick={onCreateNew}>新建批次并选品</button>
    </section>
  );
}
```

- [ ] **Step 2: Limit homepage recent-batch cards to 3 by default**

Update the dashboard to derive:

```tsx
const visibleBatches = showAllBatches ? summaries : summaries.slice(0, 3);
```

and add a `查看全部批次` control when `summaries.length > 3`.

- [ ] **Step 3: Update `/listing-kits/sds` to render homepage-only content**

Refactor the SDS page so it renders:

- page intro / CTA entry
- recent batch summary area
- no SDS product browser by default

The homepage should not mount the full product browser section.

- [ ] **Step 4: Run homepage tests**

Run:

```bash
cd D:\code\task-processor\web\listingkit-ui
npm test -- src/app/listing-kits/sds/page.test.tsx src/components/listingkit/shein-studio/shein-studio-page-shell.test.tsx src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit the homepage split**

```bash
git add web/listingkit-ui/src/app/listing-kits/sds/page.tsx web/listingkit-ui/src/components/listingkit/sds/sds-homepage-entry.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-page-shell.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.tsx
git commit -m "feat: split SDS homepage from product selection"
```

## Task 4: Implement the dedicated `/listing-kits/sds/new` route

**Files:**
- Create: `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\new\page.tsx`
- Create: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\sds\sds-new-batch-shell.tsx`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\sds\sds-product-browser.tsx`

- [ ] **Step 1: Create the focused new-batch shell**

Implement a wrapper like:

```tsx
export function SdsNewBatchShell() {
  return (
    <section>
      <p>第 1 步 · 新建批次</p>
      <h1>选择底版商品和子 SKU</h1>
      <p>完成商品选择后，再进入专门的批次工作台继续生成和审核。</p>
      <SdsProductBrowser />
    </section>
  );
}
```

- [ ] **Step 2: Create the `/listing-kits/sds/new` page**

Use:

```tsx
export default function Page() {
  return <SdsNewBatchShell />;
}
```

and ensure it does not render homepage recent-batch dashboard content.

- [ ] **Step 3: Run the new-route test**

Run:

```bash
cd D:\code\task-processor\web\listingkit-ui
npm test -- src/app/listing-kits/sds/new/page.test.tsx
```

Expected: PASS

- [ ] **Step 4: Commit the new route**

```bash
git add web/listingkit-ui/src/app/listing-kits/sds/new/page.tsx web/listingkit-ui/src/components/listingkit/sds/sds-new-batch-shell.tsx
git commit -m "feat: add dedicated SDS new-batch route"
```

## Task 5: Implement the dedicated `/listing-kits/sds/batches/:id` route

**Files:**
- Create: `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\batches\[batchId]\page.tsx`
- Create: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein-studio\shein-studio-batch-page-shell.tsx`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein-studio\shein-studio-workbench.tsx`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\lib\utils\shein-studio-batches.ts`

- [ ] **Step 1: Create a route-level batch page shell**

Wrap the existing workbench like:

```tsx
type SheinStudioBatchPageShellProps = {
  batchId: string;
};

export function SheinStudioBatchPageShell({
  batchId,
}: SheinStudioBatchPageShellProps) {
  return <SheinStudioWorkbench initialBatchId={batchId} />;
}
```

- [ ] **Step 2: Create the batch route page**

Use:

```tsx
export default function Page({
  params,
}: {
  params: { batchId: string };
}) {
  return <SheinStudioBatchPageShell batchId={params.batchId} />;
}
```

- [ ] **Step 3: Make workbench support direct batch-route entry**

Extend the workbench input contract to accept an initial batch id and call existing load logic on mount.

Minimal pattern:

```tsx
type SheinStudioWorkbenchProps = {
  initialBatchId?: string;
};
```

and on mount:

```tsx
useEffect(() => {
  if (!initialBatchId) return;
  void loadBatchById(initialBatchId);
}, [initialBatchId, loadBatchById]);
```

- [ ] **Step 4: Run the batch-route tests**

Run:

```bash
cd D:\code\task-processor\web\listingkit-ui
npm test -- src/app/listing-kits/sds/batches/[batchId]/page.test.tsx src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit the batch route**

```bash
git add web/listingkit-ui/src/app/listing-kits/sds/batches/[batchId]/page.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-page-shell.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx web/listingkit-ui/src/lib/utils/shein-studio-batches.ts
git commit -m "feat: mount SDS workbench on dedicated batch route"
```

## Task 6: Rewire homepage actions to navigate by route, not by partial expansion

**Files:**
- Modify: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein-studio\shein-studio-page-shell.tsx`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein-studio\shein-studio-recent-batches-dashboard.tsx`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\lib\shein-studio\section-highlight.ts`

- [ ] **Step 1: Replace homepage create action with route navigation**

Change the start CTA so:

```tsx
router.push("/listing-kits/sds/new");
```

instead of expanding the in-page product browser.

- [ ] **Step 2: Replace continue/open actions with batch-route navigation**

Batch card actions should navigate using:

```tsx
router.push(`/listing-kits/sds/batches/${batchId}`);
```

Queue launch entry may still carry mode state, but should resolve around the dedicated batch route rather than homepage partial expansion.

- [ ] **Step 3: Run navigation-focused tests**

Run:

```bash
cd D:\code\task-processor\web\listingkit-ui
npm test -- src/components/listingkit/shein-studio/shein-studio-page-shell.test.tsx src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx
```

Expected: PASS

- [ ] **Step 4: Commit route-based navigation cleanup**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-page-shell.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.tsx web/listingkit-ui/src/lib/shein-studio/section-highlight.ts
git commit -m "feat: route SDS homepage actions to dedicated pages"
```

## Task 7: Full verification pass

**Files:**
- Test: `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\page.test.tsx`
- Test: `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\new\page.test.tsx`
- Test: `D:\code\task-processor\web\listingkit-ui\src\app\listing-kits\sds\batches\[batchId]\page.test.tsx`
- Test: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein-studio\shein-studio-page-shell.test.tsx`
- Test: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein-studio\shein-studio-recent-batches-dashboard.test.tsx`
- Test: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein-studio\shein-studio-workbench.test.tsx`

- [ ] **Step 1: Run the targeted SDS route suite**

Run:

```bash
cd D:\code\task-processor\web\listingkit-ui
npm test -- src/app/listing-kits/sds/page.test.tsx src/app/listing-kits/sds/new/page.test.tsx src/app/listing-kits/sds/batches/[batchId]/page.test.tsx src/components/listingkit/shein-studio/shein-studio-page-shell.test.tsx src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx
```

Expected: PASS

- [ ] **Step 2: Run type-check**

Run:

```bash
cd D:\code\task-processor\web\listingkit-ui
npx tsc --noEmit
```

Expected: PASS

- [ ] **Step 3: Commit verification-safe cleanups if needed**

```bash
git add -A
git commit -m "test: verify SDS split homepage routes"
```

## Spec Coverage Check

Covered requirements:

- split `/listing-kits/sds` into homepage role
- add dedicated `/listing-kits/sds/new`
- add dedicated `/listing-kits/sds/batches/:id`
- show only 3 recent batches by default
- move full product browser off homepage
- preserve batch-editor behavior via reuse

Deferred but intentionally out of phase 1:

- deeper Phase 2 simplification of expanded recent-batch dashboard interactions
- removal of all legacy compatibility rendering paths

## Placeholder Scan

No `TODO`, `TBD`, or “implement later” placeholders remain. All steps include concrete files, commands, and code shapes.

## Type Consistency Check

Planned symbols stay consistent across tasks:

- `SdsNewBatchShell`
- `SheinStudioBatchPageShell`
- `initialBatchId`
- `/listing-kits/sds/new`
- `/listing-kits/sds/batches/:id`

