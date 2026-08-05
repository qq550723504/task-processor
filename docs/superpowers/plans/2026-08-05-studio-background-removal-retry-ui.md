# Studio Background Removal Retry UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the Studio background-removal retry button hydrate the active batch detail before retrying instead of silently doing nothing when detail state is missing.

**Architecture:** Keep retry orchestration in `shein-studio-workbench.tsx`, reuse the existing `getSheinStudioHydratedBatch` and `handleLoadHydratedBatch` paths, and preserve the existing background-removal retry API. Add focused controller tests for the missing-detail path and failure cleanup.

**Tech Stack:** React, TypeScript, Vitest, Testing Library, existing Studio batch API helpers.

## Global Constraints

- Do not change the backend retry endpoint or authorization behavior.
- Do not bypass design ID validation or change image-processing behavior.
- Keep unrelated working-tree changes untouched.

---

### Task 1: Add a failing regression test for missing batch detail

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-task-creation-controller.test.ts`
- Read: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`

**Interfaces:**
- The test must exercise the existing retry action contract with an active batch ID and no itemized detail, and assert that hydration is attempted before the retry operation.

- [ ] **Step 1: Write the failing test**

Add a test fixture for a failed removal design with `itemizedBatchDetail: null`, invoke the retry action through the workbench/controller test seam, and assert that `getSheinStudioHydratedBatch` is called before `retrySheinStudioBatchBackgroundRemoval`.

- [ ] **Step 2: Run the focused test and verify the expected failure**

Run from `web/listingkit-ui`:

```powershell
npm test -- --run src/components/listingkit/shein-studio/shein-studio-task-creation-controller.test.ts
```

Expected result: the new test fails because the current handler returns when `itemizedBatchDetail` is missing and never hydrates or retries.

### Task 2: Implement hydration before background-removal retry

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`

**Interfaces:**
- Consume the existing `getSheinStudioHydratedBatch` loader and `handleLoadHydratedBatch` state applicator.
- Produce the same `retrySheinStudioBatchBackgroundRemoval(activeBatchId, [designId], { tenantId })` call used by the current path.

- [ ] **Step 1: Remove the silent missing-detail return**

Guard only on a missing `activeBatchId`. Select the current tenant from the hydrated detail when available, otherwise from `currentActiveBatch`.

- [ ] **Step 2: Hydrate missing detail**

When `itemizedBatchDetail` is null, call `getSheinStudioHydratedBatch(activeBatchId, tenantId)` through the existing helper signature, apply the returned hydrated batch with `handleLoadHydratedBatch`, and use its detail for the retry request. If no detail is returned, throw a user-visible error through the existing workbench error path.

- [ ] **Step 3: Preserve retry state cleanup**

Keep the `finally` block clearing `retryingBackgroundRemovalId`, including hydration failures and API failures.

- [ ] **Step 4: Run the focused regression test**

Run the command from Task 1. Expected result: PASS.

### Task 3: Verify the Studio frontend change

**Files:**
- No additional files.

- [ ] **Step 1: Run the related Studio tests**

Run:

```powershell
npm test -- --run src/components/listingkit/shein-studio
```

- [ ] **Step 2: Run the frontend type-check/build validation**

Run the repository's existing frontend validation command from `web/listingkit-ui/package.json`, and confirm it completes successfully.

- [ ] **Step 3: Inspect the final diff**

Run `git diff --check` and confirm only the retry controller, its regression test, and the new design/plan documents changed; leave the pre-existing untracked implementation plan untouched.
