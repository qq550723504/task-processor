# Canonical Product Actions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add status and platform-workspace actions to the canonical-product detail page.

**Architecture:** Reuse `buildTaskWorkspaceHref` in the existing canonical-product mapper to produce a deterministic workspace URL from task-result metadata. The page renders a status link and a primary workspace link with the existing `Button asChild` and `Link` patterns.

**Tech Stack:** TypeScript, React, Next.js `Link`, Vitest, Testing Library.

## Global Constraints

- Do not add a backend endpoint, database field, or task mutation.
- Do not duplicate platform-selection logic in the page; `buildTaskWorkspaceHref` remains authoritative.
- Preserve the base workspace route for tasks without a platform.
- Keep source lineage rendering read-only and unchanged.

---

### Task 1: Add workspace navigation to the canonical-product read model

**Files:**
- Modify: `web/listingkit-ui/src/lib/canonical-products/canonical-products.ts`
- Modify: `web/listingkit-ui/src/lib/canonical-products/canonical-products.test.ts`

**Interfaces:**
- Consumes: `ListingKitTaskResult.task_id`, `result.platforms`, and `shein_workflow_status`.
- Produces: `CanonicalProductDetail.workspaceHref: string` from `buildCanonicalProductDetail`.

- [ ] **Step 1: Write the failing mapper tests**

Add assertions for the existing SHEIN fixture:

```ts
expect(detail?.workspaceHref).toBe(
  "/listing-kits/task-1/workspace?platform=shein",
);
```

Add a legacy no-platform fixture and assert:

```ts
expect(detail?.workspaceHref).toBe("/listing-kits/task-legacy/workspace");
```

- [ ] **Step 2: Run the mapper tests and verify the expected failure**

From `web/listingkit-ui` run:

```powershell
npm.cmd test -- --run src/lib/canonical-products/canonical-products.test.ts
```

Expected: the tests fail because `CanonicalProductDetail` has no `workspaceHref` and the mapper does not populate it.

- [ ] **Step 3: Implement the minimal projection**

Import `buildTaskWorkspaceHref`, add `workspaceHref: string` to `CanonicalProductDetail`, and populate it in `buildCanonicalProductDetail`:

```ts
workspaceHref: buildTaskWorkspaceHref({
  task_id: summary.taskId,
  platforms: result.result?.platforms,
  shein_workflow_status: result.shein_workflow_status,
}),
```

Do not change `buildTaskWorkspaceHref` or add a second routing helper.

- [ ] **Step 4: Run mapper tests and typecheck**

```powershell
npm.cmd test -- --run src/lib/canonical-products/canonical-products.test.ts src/lib/listingkit/task-workspace-href.test.ts
npm.cmd run typecheck
```

Expected: mapper and routing tests pass and TypeScript reports no errors.

- [ ] **Step 5: Commit the read-model change**

```powershell
git add web/listingkit-ui/src/lib/canonical-products/canonical-products.ts web/listingkit-ui/src/lib/canonical-products/canonical-products.test.ts
git diff --cached --check
git commit -m "feat: derive workspace action for canonical products"
```

---

### Task 2: Add status and workspace actions to the detail page

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.test.tsx`

**Interfaces:**
- Consumes: `CanonicalProductDetail.workspaceHref` from Task 1 and `taskId`.
- Produces: `查看原任务` link to `/listing-kits/:taskId/status` and `进入工作台` link to `workspaceHref`.

- [ ] **Step 1: Write the failing page test**

Add assertions to the existing canonical detail page test:

```tsx
expect(screen.getByRole("link", { name: "查看原任务" })).toHaveAttribute(
  "href",
  "/listing-kits/task-1/status",
);
expect(screen.getByRole("link", { name: "进入工作台" })).toHaveAttribute(
  "href",
  "/listing-kits/task-1/workspace?platform=shein",
);
```

- [ ] **Step 2: Run the page test and verify the expected failure**

```powershell
npm.cmd test -- --run src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
```

Expected: the test fails because the detail page does not render either action.

- [ ] **Step 3: Implement the action group**

Import `Button`, then render this action group in the canonical detail header after the brand/category line:

```tsx
<div className="mt-4 flex flex-wrap gap-3">
  <Button asChild>
    <Link href={detail.data.workspaceHref}>进入工作台</Link>
  </Button>
  <Button asChild variant="outline">
    <Link href={`/listing-kits/${detail.data.taskId}/status`}>查看原任务</Link>
  </Button>
</div>
```

Keep the workspace action primary and the status action secondary. Do not add click handlers or client-side routing state.

- [ ] **Step 4: Run page tests, focused lint, and typecheck**

```powershell
npm.cmd test -- --run src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
npm.cmd run typecheck
npm.cmd run lint -- src/components/listingkit/canonical/canonical-product-detail-page.tsx src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
```

Expected: tests pass, typecheck succeeds, and focused lint reports no errors.

- [ ] **Step 5: Commit the page actions**

```powershell
git add web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
git diff --cached --check
git commit -m "feat: add canonical product navigation actions"
```

---

### Task 3: Run regression checks and hand off

**Files:**
- No new files; verify the Task 1 and Task 2 changes.

**Interfaces:**
- Consumes: canonical-product workspace projection and page action links.
- Produces: verified branch ready for review.

- [ ] **Step 1: Run focused navigation coverage**

```powershell
npm.cmd test -- --run src/lib/canonical-products/canonical-products.test.ts src/lib/listingkit/task-workspace-href.test.ts src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
```

- [ ] **Step 2: Run full UI validation**

```powershell
npm.cmd test -- --maxWorkers=4
npm.cmd run typecheck
npm.cmd run lint
```

Expected: all UI tests pass, typecheck has no errors, and lint has no errors. Existing unrelated lint warnings may remain and must be reported rather than changed in this slice.

- [ ] **Step 3: Review final diff and worktree**

```powershell
git diff origin/master...HEAD --stat
git diff --check
git status --short
```

Expected: only the design/plan docs and the mapper/page source-and-test changes are present; the worktree is clean after commits.

- [ ] **Step 4: Commit or hand off**

Use `superpowers:finishing-a-development-branch` after all checks pass to choose whether to keep the branch, push a Draft PR, or merge locally.
