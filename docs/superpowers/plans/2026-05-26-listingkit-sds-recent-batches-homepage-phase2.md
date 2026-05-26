# ListingKit SDS Recent Batches Homepage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn `/listing-kits/sds` into a batch-first entry experience with a recent-batches homepage, direct candidate-to-batch assignment, batch summary cards, and safe batch-level bulk operations.

**Architecture:** Keep the existing SDS workbench as a single-batch editor. Add a homepage dashboard layer that derives recent batch summaries from persisted `SheinStudioSavedBatch` records plus optional local-only recovery state. Reuse current batch detail/workbench loading instead of inventing a second editor model.

**Tech Stack:** Next.js App Router, React hooks/reducer state, existing studio batch/session APIs, Vitest, TypeScript

---

### Task 1: Define Recent Batch Summary and Recovery Model

**Files:**
- Modify: `web/listingkit-ui/src/lib/types/shein-studio.ts`
- Modify: `web/listingkit-ui/src/lib/utils/shein-studio-batches.ts`
- Add: `web/listingkit-ui/src/lib/shein-studio/recent-batch-summaries.ts`
- Test: `web/listingkit-ui/src/lib/shein-studio/recent-batch-summaries.test.ts`

- [ ] **Step 1: Write failing tests for recent batch summary derivation**

Add tests that prove:

```ts
it("derives batch card summary from a persisted batch", () => {
  const summaries = buildRecentBatchSummaries([
    {
      id: "batch-1",
      name: "Retro Cherries",
      prompt: "retro cherries",
      sheinStoreId: "869",
      selection: { productName: "tee", variantId: 100, ...selection },
      groupedSelections: [
        { selectionId: "sel-1", sheinStoreId: "869", selection: hoodie, eligible: true, baselineStatus: "ready", baselineReason: "" },
      ],
      designs: [{ id: "design-1" }],
      selectedIds: ["design-1"],
      createdTasks: [],
      updatedAt: "2026-05-26T10:00:00.000Z",
    },
  ]);

  expect(summaries[0]).toMatchObject({
    id: "batch-1",
    title: "Retro Cherries",
    primaryProductName: "tee",
    productCount: 2,
    promptPreview: "retro cherries",
    designCount: 1,
    createdTaskCount: 0,
  });
});

it("marks local recovery drafts separately from persisted batches", () => {
  const summaries = buildRecentBatchSummaries([], {
    draft: {
      prompt: "draft prompt",
      groups: [groupOne],
      updatedAt: "2026-05-26T11:00:00.000Z",
    },
  });

  expect(summaries[0]).toMatchObject({
    source: "local_draft",
    isRecoverableDraft: true,
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
npm test -- recent-batch-summaries.test.ts
```

Expected: FAIL because no summary builder exists yet.

- [ ] **Step 3: Add summary and recovery helper types**

Extend shared types with a lightweight homepage summary shape, for example:

```ts
export type SheinStudioRecentBatchSummary = {
  id: string;
  source: "batch" | "local_draft";
  isRecoverableDraft: boolean;
  title: string;
  primaryProductName: string;
  productCount: number;
  promptPreview: string;
  storeSummary: string;
  designCount: number;
  createdTaskCount: number;
  updatedAt: string;
};
```

- [ ] **Step 4: Implement summary derivation and precedence rules**

Implement `buildRecentBatchSummaries()` so it:

- derives homepage summaries from persisted batches
- optionally prepends a local recoverable draft summary
- prefers persisted batch records when the same batch already exists
- sorts newest-first

- [ ] **Step 5: Run tests to verify summary derivation passes**

Run:

```bash
npm test -- recent-batch-summaries.test.ts
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/listingkit-ui/src/lib/types/shein-studio.ts web/listingkit-ui/src/lib/utils/shein-studio-batches.ts web/listingkit-ui/src/lib/shein-studio/recent-batch-summaries.ts web/listingkit-ui/src/lib/shein-studio/recent-batch-summaries.test.ts
git commit -m "feat: add SDS recent batch summary model"
```

### Task 2: Render Recent Batches Homepage on `/listing-kits/sds`

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`
- Add: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx`

- [ ] **Step 1: Write failing homepage-entry tests**

Add tests proving:

```ts
it("shows recent batch cards before any explicit product reselection", async () => {
  listSheinStudioBatches.mockResolvedValue([batchOne, batchTwo]);

  render(<SheinStudioWorkbench activeStep="generate" />);

  expect(await screen.findByText("最近批次")).toBeInTheDocument();
  expect(screen.getByText("Retro Cherries")).toBeInTheDocument();
  expect(screen.getByText("2 款商品")).toBeInTheDocument();
});

it("loads the selected batch into the editor when clicking a card", async () => {
  listSheinStudioBatches.mockResolvedValue([batchOne, batchTwo]);

  render(<SheinStudioWorkbench activeStep="generate" />);

  fireEvent.click(await screen.findByRole("button", { name: /Retro Cherries/ }));
  expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
npm test -- shein-studio-workbench.test.tsx shein-studio-recent-batches-dashboard.test.tsx
```

Expected: FAIL because the recent batch dashboard does not exist yet.

- [ ] **Step 3: Add a dedicated recent-batches dashboard component**

Render:

- section title `最近批次`
- empty state when none exist
- newest-first batch cards
- buttons for:
  - `继续编辑`
  - `新建批次`

- [ ] **Step 4: Wire workbench homepage selection into existing batch loader**

When a batch card is chosen:

- reuse current `applyBatch`
- hydrate workbench state from that batch
- do not require the user to reselect the original product first

Keep the workbench editor itself single-batch.

- [ ] **Step 5: Run homepage-entry tests to verify they pass**

Run:

```bash
npm test -- shein-studio-workbench.test.tsx shein-studio-recent-batches-dashboard.test.tsx
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx
git commit -m "feat: add SDS recent batches homepage"
```

### Task 3: Support Candidate Pool Add-to-Batch

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/sds/sds-grouped-candidates-panel.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/sds/sds-product-browser.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/sds/sds-grouped-candidates-panel.test.tsx`

- [ ] **Step 1: Write failing add-to-batch tests**

Add tests that prove:

```ts
it("allows adding a ready candidate into the current batch", async () => {
  fireEvent.click(screen.getByRole("button", { name: "加入当前批次" }));
  expect(saveSheinStudioDraftWithOptions).toHaveBeenCalled();
});

it("allows sending a ready candidate into another recent batch", async () => {
  fireEvent.click(screen.getByRole("button", { name: "加入其他批次" }));
  fireEvent.click(screen.getByRole("button", { name: /Retro Cherries/ }));
  expect(saveSheinStudioBatch).toHaveBeenCalledWith(
    expect.objectContaining({
      id: "batch-1",
    }),
  );
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
npm test -- sds-grouped-candidates-panel.test.tsx
```

Expected: FAIL because candidate-to-batch actions do not exist yet.

- [ ] **Step 3: Add batch-target actions to the candidate panel**

For ready candidates, support:

- `加入当前批次`
- `加入其他批次`
- `新建批次并加入`

Keep non-ready candidates blocked with existing readiness messaging.

- [ ] **Step 4: Implement persistence updates for target batches**

When adding to another batch:

- load/update that batch’s grouped selections
- preserve existing prompt/design/task state
- update `updatedAt`

Prefer reusing current batch save utilities instead of creating a one-off API.

- [ ] **Step 5: Run add-to-batch tests to verify they pass**

Run:

```bash
npm test -- sds-grouped-candidates-panel.test.tsx shein-studio-workbench.test.tsx
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/sds/sds-grouped-candidates-panel.tsx web/listingkit-ui/src/components/listingkit/sds/sds-product-browser.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx web/listingkit-ui/src/components/listingkit/sds/sds-grouped-candidates-panel.test.tsx
git commit -m "feat: support SDS candidate add-to-batch flow"
```

### Task 4: Add Batch Card Management Actions

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.tsx`
- Modify: `web/listingkit-ui/src/lib/utils/shein-studio-batches.ts`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx`

- [ ] **Step 1: Write failing batch-management tests**

Add tests that prove:

```ts
it("renames a batch from the homepage", async () => {
  fireEvent.click(screen.getByRole("button", { name: /rename batch-1/i }));
  fireEvent.change(screen.getByLabelText("批次名称"), { target: { value: "New Name" } });
  fireEvent.click(screen.getByRole("button", { name: "保存名称" }));
  expect(saveSheinStudioBatch).toHaveBeenCalledWith(
    expect.objectContaining({ id: "batch-1", name: "New Name" }),
  );
});

it("deletes a batch from the homepage", async () => {
  fireEvent.click(screen.getByRole("button", { name: /delete batch-1/i }));
  expect(deleteSheinStudioBatch).toHaveBeenCalledWith("batch-1");
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
npm test -- shein-studio-recent-batches-dashboard.test.tsx
```

Expected: FAIL because batch-card actions are not implemented yet.

- [ ] **Step 3: Add single-batch homepage actions**

Support:

- rename
- duplicate
- delete

Duplicate should:

- clone batch content
- assign a new id through existing save path
- update title and timestamp

- [ ] **Step 4: Run homepage management tests to verify they pass**

Run:

```bash
npm test -- shein-studio-recent-batches-dashboard.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.tsx web/listingkit-ui/src/lib/utils/shein-studio-batches.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx
git commit -m "feat: add recent batch management actions"
```

### Task 5: Add Safe Batch-Level Bulk Store Operations

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.tsx`
- Modify: `web/listingkit-ui/src/lib/shein-studio/recent-batch-summaries.ts`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx`

- [ ] **Step 1: Write failing bulk-operation tests**

Add tests that prove:

```ts
it("supports multi-selecting recent batch cards", async () => {
  fireEvent.click(screen.getByRole("checkbox", { name: /select batch-1/i }));
  fireEvent.click(screen.getByRole("checkbox", { name: /select batch-2/i }));
  expect(screen.getByText("已选择 2 个批次")).toBeInTheDocument();
});

it("bulk updates selected batches to follow one explicit store", async () => {
  fireEvent.click(screen.getByRole("button", { name: "批量改店铺" }));
  fireEvent.change(screen.getByLabelText("目标店铺"), { target: { value: "869" } });
  fireEvent.click(screen.getByRole("button", { name: "应用到已选批次" }));
  expect(saveSheinStudioBatch).toHaveBeenCalledTimes(2);
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
npm test -- shein-studio-recent-batches-dashboard.test.tsx
```

Expected: FAIL because multi-select and bulk actions do not exist yet.

- [ ] **Step 3: Add batch multi-select and bulk store toolbar**

Support:

- selecting batch cards
- clearing selection
- bulk set to follow current store
- bulk set to one explicit store

Keep bulk actions metadata-safe only in this phase.

- [ ] **Step 4: Run bulk-operation tests to verify they pass**

Run:

```bash
npm test -- shein-studio-recent-batches-dashboard.test.tsx shein-studio-workbench.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.tsx web/listingkit-ui/src/lib/shein-studio/recent-batch-summaries.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx
git commit -m "feat: add batch-level bulk store operations"
```

### Task 6: Final Regression and Compatibility Verification

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`
- Modify: `web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio.test.ts`

- [ ] **Step 1: Add regression coverage for legacy and coexistence cases**

Cover:

- legacy grouped draft still recoverable
- persisted recent batches take precedence over local-only recovery
- opening a batch does not drop prompt history
- candidate add-to-batch does not corrupt grouped selections

- [ ] **Step 2: Run focused regression suite**

Run:

```bash
npm test -- shein-studio.test.ts shein-studio-batches.test.ts shein-studio-workbench.test.tsx shein-studio-recent-batches-dashboard.test.tsx
```

Expected: PASS

- [ ] **Step 3: Run full typecheck**

Run:

```bash
npx tsc --noEmit --ignoreDeprecations 5.0
```

Expected: PASS

- [ ] **Step 4: Run final SDS homepage verification suite**

Run:

```bash
npm test -- grouped-sds-create.test.ts draft-input.test.ts shein-studio-workbench-state.test.ts shein-studio-generation-panel.test.tsx shein-studio-grouped-selection-panel.test.tsx shein-studio-workbench.test.tsx shein-studio-recent-batches-dashboard.test.tsx sds-grouped-candidates-panel.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts web/listingkit-ui/src/lib/api/shein-studio.test.ts
git commit -m "test: cover SDS recent batch homepage compatibility"
```

## Spec Coverage Check

- recent batch homepage: covered by Tasks 1-2
- continue editing existing independent batches: covered by Task 2
- candidate pool add-to-batch flow: covered by Task 3
- batch rename / duplicate / delete: covered by Task 4
- batch-level bulk store operations: covered by Task 5
- legacy compatibility and precedence rules: covered by Task 6

## Placeholder Scan

- No `TODO` or `TBD` placeholders remain
- Every task includes exact files, commands, and expected outcomes
- Bulk actions are intentionally scoped to safe metadata operations in this phase

## Type Consistency Check

- `SheinStudioRecentBatchSummary` is the homepage summary shape throughout
- persisted `SheinStudioSavedBatch` remains the durable batch unit
- local grouped recovery is treated as a distinct recoverable draft source, not a peer batch type
