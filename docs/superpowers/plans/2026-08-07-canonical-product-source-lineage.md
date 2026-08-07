# Canonical Product Source Lineage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Carry the persisted task source reference into the canonical-product detail read model and render it on the standard-product detail page.

**Architecture:** Reuse the existing task-result `source_reference` and the existing `TaskPersistedSourceReference` card. The frontend mapper adds an optional `sourceReference` field to `CanonicalProductDetail`; `canonical.Product` and backend persistence remain unchanged.

**Tech Stack:** TypeScript, React, Vitest, Testing Library, existing ListingKit task-result types and canonical-product mapper.

## Global Constraints

- Do not add a database migration, endpoint, or persisted field to `canonical.Product`.
- Do not make source lineage editable or reconstruct it from browser-local task drafts.
- Treat blank or missing source fields as absent and keep legacy canonical-product details renderable.
- External source links must use `target="_blank"` and `rel="noreferrer"` through the existing source-reference component.

---

### Task 1: Carry source lineage through the canonical-product mapper

**Files:**
- Modify: `web/listingkit-ui/src/lib/canonical-products/canonical-products.ts`
- Modify: `web/listingkit-ui/src/lib/canonical-products/canonical-products.test.ts`

**Interfaces:**
- Consumes: `ListingKitTaskResult.source_reference?: ListingKitSourceReference`.
- Produces: `CanonicalProductDetail.sourceReference?: ListingKitSourceReference` returned by `buildCanonicalProductDetail`.

- [ ] **Step 1: Write the failing mapper tests**

Add a `source_reference` fixture to `taskResult` and assert:

```ts
expect(detail?.sourceReference).toEqual({
  key: "crawler:1688:888",
  type: "crawler",
  platform: "1688",
  id: "888",
  url: "https://detail.1688.com/offer/888.html",
});
```

Also add a legacy assertion using `buildCanonicalProductDetail({ result: { canonical_product: ... } })` that returns `sourceReference === undefined`.

- [ ] **Step 2: Run the mapper tests and verify the expected failure**

Run from `web/listingkit-ui`:

```powershell
npm.cmd test -- --run src/lib/canonical-products/canonical-products.test.ts
```

Expected: the test fails because `CanonicalProductDetail` does not expose `sourceReference` and `buildCanonicalProductDetail` does not populate it.

- [ ] **Step 3: Implement the minimal read-model projection**

Import `ListingKitSourceReference`, add `sourceReference?: ListingKitSourceReference` to `CanonicalProductDetail`, and add a defensive object copy in `buildCanonicalProductDetail`:

```ts
sourceReference: result.source_reference
  ? { ...result.source_reference }
  : undefined,
```

Keep `buildCanonicalProductListItem` unchanged because list items already have their own source projection and this task is detail-only.

- [ ] **Step 4: Run the mapper tests and typecheck**

```powershell
npm.cmd test -- --run src/lib/canonical-products/canonical-products.test.ts
npm.cmd run typecheck
```

Expected: mapper tests pass and TypeScript reports no errors.

- [ ] **Step 5: Commit the mapper change**

```powershell
git add web/listingkit-ui/src/lib/canonical-products/canonical-products.ts web/listingkit-ui/src/lib/canonical-products/canonical-products.test.ts
git diff --cached --check
git commit -m "feat: carry source lineage into canonical product details"
```

---

### Task 2: Render source lineage on the canonical-product detail page

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.test.tsx`

**Interfaces:**
- Consumes: `CanonicalProductDetail.sourceReference` from Task 1.
- Produces: a read-only source card on the standard-product detail page, using `TaskPersistedSourceReference`.

- [ ] **Step 1: Write the failing page test**

Add `sourceReference` to the mocked detail data and assert the page shows the persisted identity and safe link:

```tsx
expect(screen.getByText("来源 1688 · 888")).toBeInTheDocument();
expect(screen.getByRole("link", { name: "查看来源" })).toHaveAttribute(
  "href",
  "https://detail.1688.com/offer/888.html",
);
```

- [ ] **Step 2: Run the page test and verify the expected failure**

```powershell
npm.cmd test -- --run src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
```

Expected: the test fails because the detail page does not render a source card.

- [ ] **Step 3: Reuse the existing source-reference component**

Import `TaskPersistedSourceReference` from `@/components/listingkit/tasks/task-persisted-source-reference` and render it after the canonical detail header, before the product image/content grid:

```tsx
<TaskPersistedSourceReference source={detail.data.sourceReference} />
```

The existing component handles blank references, platform/ID formatting, and safe external-link attributes. Do not create a second source-card implementation.

- [ ] **Step 4: Run the page tests, focused lint, and typecheck**

```powershell
npm.cmd test -- --run src/components/listingkit/canonical/canonical-product-detail-page.test.tsx src/components/listingkit/tasks/task-persisted-source-reference.test.tsx
npm.cmd run typecheck
npm.cmd run lint -- src/components/listingkit/canonical/canonical-product-detail-page.tsx src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
```

Expected: all focused tests pass, typecheck succeeds, and focused lint reports no errors.

- [ ] **Step 5: Commit the page integration**

```powershell
git add web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.tsx web/listingkit-ui/src/components/listingkit/canonical/canonical-product-detail-page.test.tsx
git diff --cached --check
git commit -m "feat: show source lineage on canonical product details"
```

---

### Task 3: Run regression checks and hand off

**Files:**
- No new source files; verify the Task 1 and Task 2 changes.

**Interfaces:**
- Consumes: canonical-product mapper and detail-page source projection.
- Produces: verified branch ready for review.

- [ ] **Step 1: Run the focused canonical-product suite**

```powershell
npm.cmd test -- --run src/lib/canonical-products/canonical-products.test.ts src/components/listingkit/canonical/canonical-product-detail-page.test.tsx src/components/listingkit/tasks/task-persisted-source-reference.test.tsx
```

- [ ] **Step 2: Run the full UI validation**

```powershell
npm.cmd test -- --maxWorkers=4
npm.cmd run typecheck
npm.cmd run lint
```

Expected: all UI tests pass, typecheck has no errors, and lint has no errors. Existing unrelated lint warnings may remain and must be reported rather than changed in this slice.

- [ ] **Step 3: Review the final diff and worktree**

```powershell
git diff origin/master...HEAD --stat
git diff --check
git status --short
```

Expected: only the design/plan docs and the mapper/page test-and-source changes are present; the worktree is clean after commits.

- [ ] **Step 4: Commit or hand off**

Use `superpowers:finishing-a-development-branch` after all checks pass to choose whether to keep the branch, push a Draft PR, or merge locally.
