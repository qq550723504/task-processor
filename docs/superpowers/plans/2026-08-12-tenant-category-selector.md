# Tenant Category Selector Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the import task page's numeric category ID input with a selector backed by the current tenant's enabled categories while preserving an explicit empty choice.

**Architecture:** Reuse the existing `GET /admin/categories` API and `ListingCategory` contract. The import page will build readable parent/child labels from the flat tenant-scoped category list, submit the selected category's existing numeric `id`, and submit no category when the empty option is selected. No backend route or schema changes are needed for this UI-only change.

**Tech Stack:** Next.js/React, TanStack Query, existing ListingKit API client, Vitest Testing Library.

## Global Constraints

- Only enabled tenant categories (`status=1`) are selectable.
- The empty option remains available and must leave `categoryId` undefined.
- Do not use SHEIN official category APIs or introduce a second category model.
- Preserve unrelated working-tree changes.
- Do not commit, push, deploy, or activate production changes without explicit authorization.

### Task 1: Add tenant category loading and selector behavior

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/admin/import-task-admin-page.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/admin/import-task-admin-page.test.tsx`

**Interfaces:**
- Consumes: `getListingCategories(query)` and `ListingCategory` from `@/lib/api/admin-categories`.
- Produces: An import form field labeled `分类` whose blank value maps to `categoryId: undefined` and whose selected value maps to the tenant category `id`.

- [ ] **Step 1: Write the failing selector test**

Mock `getListingCategories` with an enabled parent category `10` and child category `22`, then select the child option by value `22`, upload one product ID, submit, and assert the batch-create call contains `categoryId: 22`.

- [ ] **Step 2: Run the focused test and verify RED**

Run from `web/listingkit-ui`:

```powershell
npm.cmd test -- --run src/components/listingkit/admin/import-task-admin-page.test.tsx
```

Expected: the selector test fails because the page currently renders a numeric `类目 ID` input and does not load tenant categories.

- [ ] **Step 3: Implement the selector**

Add a TanStack Query for `getListingCategories({ status: "1" })`. Build labels from the flat list using `parentId`, with a cycle/missing-parent guard; render `未指定分类` plus enabled category options through the existing `ImportTaskSelect`. Map the blank option to `undefined`, map selected values with `Number`, include category-query errors in `visibleError`, and disable the selector while categories load.

- [ ] **Step 4: Run focused frontend tests and verify GREEN**

Run:

```powershell
npm.cmd run typecheck
npm.cmd test -- --run src/components/listingkit/admin/import-task-admin-page.test.tsx src/lib/api/admin-import-tasks.test.ts
```

Expected: typecheck succeeds and all selected test files pass, including the existing empty-category import test.

### Task 2: Verify API contract and regression boundaries

**Files:**
- Test: `web/listingkit-ui/src/components/listingkit/admin/import-task-admin-page.test.tsx`

**Interfaces:**
- Consumes: Existing category list parser and import batch API spy.
- Produces: Regression coverage that the selector uses the tenant category API and does not alter the import request contract.

- [ ] **Step 1: Assert the existing category API query**

In the page test, assert that the `getListingCategories` spy is called with `{ status: "1" }`. The existing `admin-categories` API test already covers serialization of the status query, so do not add a new endpoint or duplicate that API test.

- [ ] **Step 2: Run the complete affected verification**

Run from the repository root:

```powershell
go test ./internal/listingadmin ./internal/listingruntime/local ./internal/listingkit/... ./internal/listingcontrol ./internal/app/task ./internal/app/consumer ./internal/app/runtime/listing ./internal/processor -count=1
```

Run from `web/listingkit-ui`:

```powershell
npm.cmd run typecheck
npm.cmd test -- --run src/components/listingkit/admin/import-task-admin-page.test.tsx src/lib/api/admin-import-tasks.test.ts src/lib/api/admin-categories.test.ts
```

Also run `git diff --check` and confirm the existing tenant store-statistics route changes remain present and no unrelated files are modified.

- [ ] **Step 3: Handoff**

Report the files changed and verification results. Keep the change local until explicit deployment authorization is provided.
