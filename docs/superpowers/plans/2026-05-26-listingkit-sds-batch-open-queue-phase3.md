# ListingKit SDS Batch Open Queue Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add homepage-level bulk actions that open selected SDS batches into the existing workbench one by one for continue-generate or create-task workflows.

**Architecture:** Keep the recent-batches dashboard as the launcher and keep the Shein Studio workbench as the only execution surface. Add a lightweight in-memory queue state to the workbench, let the homepage emit selected batch ids plus an action mode, and reuse `handleLoadBatch()` for each queue step instead of building a second batch runner.

**Tech Stack:** Next.js App Router, React hooks/reducer state, existing Shein Studio batch utilities, Vitest, TypeScript

---

### Task 1: Add Workbench Queue State Model

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.ts`
- Modify: `web/listingkit-ui/src/lib/types/shein-studio.ts`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.test.ts`

- [ ] **Step 1: Write the failing queue-state tests**

Add tests that prove:

```ts
it("stores batch queue metadata in workbench state", () => {
  const state = buildInitialSheinStudioWorkbenchState();
  const next = sheinStudioWorkbenchReducer(
    state,
    setSheinStudioWorkbenchField("batchQueueMode", "generate"),
  );

  expect(next.batchQueueMode).toBe("generate");
});

it("tracks queued batch ids and current index", () => {
  const state = buildInitialSheinStudioWorkbenchState();
  const withIds = sheinStudioWorkbenchReducer(
    state,
    setSheinStudioWorkbenchField("queuedBatchIds", ["batch-1", "batch-2"]),
  );
  const next = sheinStudioWorkbenchReducer(
    withIds,
    setSheinStudioWorkbenchField("queuedBatchIndex", 1),
  );

  expect(next.queuedBatchIds).toEqual(["batch-1", "batch-2"]);
  expect(next.queuedBatchIndex).toBe(1);
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
npm test -- shein-studio-workbench-state.test.ts
```

Expected: FAIL because queue fields do not exist yet.

- [ ] **Step 3: Add queue state fields and reducer support**

Extend the workbench state with:

```ts
type SheinStudioBatchQueueMode = "generate" | "create_tasks";

batchQueueMode: SheinStudioBatchQueueMode | null;
queuedBatchIds: string[];
queuedBatchIndex: number;
queueMessage: string;
```

Ensure the initial state includes:

```ts
batchQueueMode: null,
queuedBatchIds: [],
queuedBatchIndex: 0,
queueMessage: "",
```

- [ ] **Step 4: Run tests to verify queue state passes**

Run:

```bash
npm test -- shein-studio-workbench-state.test.ts
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.ts web/listingkit-ui/src/lib/types/shein-studio.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.test.ts
git commit -m "feat: add SDS batch queue workbench state"
```

### Task 2: Add Homepage Bulk Queue Launch Actions

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx`

- [ ] **Step 1: Write failing dashboard queue-launch tests**

Add tests that prove:

```ts
it("shows bulk queue actions when persisted batches are selected", () => {
  renderDashboardWithTwoPersistedBatches();

  fireEvent.click(screen.getByRole("checkbox", { name: "select batch-1" }));
  fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));

  expect(screen.getByRole("button", { name: "批量继续生成" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "批量创建任务" })).toBeInTheDocument();
});

it("emits selected persisted batch ids for continue-generate mode", () => {
  const onOpenBatchQueue = vi.fn();
  renderDashboard({ onOpenBatchQueue });

  fireEvent.click(screen.getByRole("checkbox", { name: "select batch-1" }));
  fireEvent.click(screen.getByRole("button", { name: "批量继续生成" }));

  expect(onOpenBatchQueue).toHaveBeenCalledWith({
    batchIds: ["batch-1"],
    mode: "generate",
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
npm test -- shein-studio-recent-batches-dashboard.test.tsx
```

Expected: FAIL because queue-launch actions do not exist yet.

- [ ] **Step 3: Add bulk queue action callbacks to the dashboard**

Add a prop:

```ts
onOpenBatchQueue?: (input: {
  batchIds: string[];
  mode: "generate" | "create_tasks";
}) => void;
```

Render the buttons only when one or more persisted `batch` summaries are selected:

```tsx
<Button onClick={() => onOpenBatchQueue?.({ batchIds, mode: "generate" })}>
  批量继续生成
</Button>
<Button onClick={() => onOpenBatchQueue?.({ batchIds, mode: "create_tasks" })}>
  批量创建任务
</Button>
```

Filter out `local_draft` rows before emitting ids.

- [ ] **Step 4: Run tests to verify dashboard queue launch passes**

Run:

```bash
npm test -- shein-studio-recent-batches-dashboard.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx
git commit -m "feat: add homepage batch queue launch actions"
```

### Task 3: Add Queue Banner and Sequential Batch Loading

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`
- Add: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-queue-banner.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`

- [ ] **Step 1: Write failing queue-banner tests**

Add tests that prove:

```ts
it("starts queue mode from homepage selection and loads the first batch", async () => {
  listSheinStudioBatches.mockResolvedValue([batchOne, batchTwo]);
  render(<SheinStudioWorkbench activeStep="generate" />);

  fireEvent.click(await screen.findByRole("checkbox", { name: "select batch-1" }));
  fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
  fireEvent.click(screen.getByRole("button", { name: "批量继续生成" }));

  expect(await screen.findByText("批量继续生成")).toBeInTheDocument();
  expect(screen.getByText("第 1 / 2 个批次")).toBeInTheDocument();
  expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument();
});

it("moves to the next batch when clicking next", async () => {
  listSheinStudioBatches.mockResolvedValue([batchOne, batchTwo]);
  render(<SheinStudioWorkbench activeStep="generate" />);
  startQueue();

  fireEvent.click(await screen.findByRole("button", { name: "下一批次" }));
  expect(await screen.findByDisplayValue("second prompt")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
npm test -- shein-studio-workbench.test.tsx
```

Expected: FAIL because queue mode and banner do not exist yet.

- [ ] **Step 3: Add a dedicated queue banner component**

Create a small presentational component that renders:

```tsx
export function SheinStudioBatchQueueBanner({
  currentBatchName,
  currentIndex,
  mode,
  total,
  onExit,
  onNext,
  onSkip,
}: {
  currentBatchName: string;
  currentIndex: number;
  mode: "generate" | "create_tasks";
  total: number;
  onExit: () => void;
  onNext: () => void;
  onSkip: () => void;
}) {
  return (
    <section>
      <div>{mode === "generate" ? "批量继续生成" : "批量创建任务"}</div>
      <div>{`第 ${currentIndex + 1} / ${total} 个批次`}</div>
      <div>{currentBatchName}</div>
      <button onClick={onNext} type="button">下一批次</button>
      <button onClick={onSkip} type="button">跳过</button>
      <button onClick={onExit} type="button">退出批量处理</button>
    </section>
  );
}
```

- [ ] **Step 4: Wire homepage queue selection into the existing batch loader**

In `SheinStudioWorkbench`:

- add `handleOpenBatchQueue({ batchIds, mode })`
- validate ids against `savedBatches`
- set queue fields in state
- load the first batch through `handleLoadBatch(batch)`
- set step:
  - `generate` mode -> `setEffectiveStep("generate")`
  - `create_tasks` mode -> `setEffectiveStep(batch.createdTasks.length > 0 ? "tasks" : batch.designs.length > 0 ? "review" : "generate")`

- [ ] **Step 5: Implement queue navigation handlers**

Add:

```ts
function clearBatchQueue() {
  workbenchController.setField("batchQueueMode", null);
  workbenchController.setField("queuedBatchIds", []);
  workbenchController.setField("queuedBatchIndex", 0);
  workbenchController.setField("queueMessage", "");
}
```

and:

```ts
function loadQueuedBatch(index: number) {
  const batchId = queuedBatchIds[index];
  const batch = savedBatches.find((item) => item.id === batchId);
  if (!batch) {
    return false;
  }
  handleLoadBatch(batch);
  workbenchController.setField("queuedBatchIndex", index);
  return true;
}
```

Use these for `下一批次`, `跳过`, and queue completion.

- [ ] **Step 6: Run tests to verify queue loading passes**

Run:

```bash
npm test -- shein-studio-workbench.test.tsx
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-queue-banner.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx
git commit -m "feat: add sequential SDS batch queue mode"
```

### Task 4: Handle Missing Batch IDs and Queue Exit Safety

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`

- [ ] **Step 1: Write failing queue-safety tests**

Add tests that prove:

```ts
it("skips missing batch ids and continues to the next available batch", async () => {
  listSheinStudioBatches.mockResolvedValue([batchTwo]);
  render(<SheinStudioWorkbench activeStep="generate" />);

  openQueueWithIds(["missing-batch", "batch-2"], "generate");
  expect(await screen.findByDisplayValue("second prompt")).toBeInTheDocument();
});

it("clears queue mode when exiting", async () => {
  listSheinStudioBatches.mockResolvedValue([batchOne, batchTwo]);
  render(<SheinStudioWorkbench activeStep="generate" />);
  startQueue();

  fireEvent.click(await screen.findByRole("button", { name: "退出批量处理" }));
  expect(screen.queryByText("批量继续生成")).not.toBeInTheDocument();
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
npm test -- shein-studio-workbench.test.tsx
```

Expected: FAIL because missing-id skip and explicit exit are not implemented yet.

- [ ] **Step 3: Add safe queue fallback behavior**

When loading a queued batch:

- if batch is missing, move to the next id
- if no ids remain, clear queue and set:

```ts
workbenchController.setField(
  "queueMessage",
  "已完成这批批次的顺序处理。",
);
```

On explicit exit:

- clear queue state
- leave the currently loaded batch untouched

- [ ] **Step 4: Run tests to verify queue safety passes**

Run:

```bash
npm test -- shein-studio-workbench.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx
git commit -m "feat: harden SDS batch queue navigation"
```

### Task 5: Final Regression and Type Verification

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`

- [ ] **Step 1: Add regression coverage for coexistence with existing homepage actions**

Add tests covering:

- queue launch still coexists with rename / duplicate / delete controls
- local draft summaries do not participate in queue selection
- bulk store update toolbar still works when queue actions are present

- [ ] **Step 2: Run focused regression suite**

Run:

```bash
npm test -- shein-studio-recent-batches-dashboard.test.tsx shein-studio-workbench.test.tsx shein-studio-workbench-state.test.ts
```

Expected: PASS

- [ ] **Step 3: Run full typecheck**

Run:

```bash
npx tsc --noEmit --ignoreDeprecations 5.0
```

Expected: PASS

- [ ] **Step 4: Run final recent-batches suite**

Run:

```bash
npm test -- shein-studio.test.ts shein-studio-batches.test.ts shein-studio-workbench-state.test.ts shein-studio-workbench.test.tsx shein-studio-recent-batches-dashboard.test.tsx sds-grouped-candidates-panel.test.tsx grouped-sds-create.test.ts draft-input.test.ts shein-studio-generation-panel.test.tsx shein-studio-grouped-selection-panel.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx
git commit -m "test: cover SDS batch queue homepage flow"
```

## Spec Coverage Check

- homepage bulk queue launcher: covered by Tasks 2-3
- queue banner and sequential loading: covered by Task 3
- continue-generate and create-task queue modes: covered by Task 3
- skip / next / exit controls: covered by Task 4
- safe missing-batch handling: covered by Task 4
- coexistence with existing homepage actions: covered by Task 5

## Placeholder Scan

- No `TODO` or `TBD` placeholders remain
- Every task includes exact files, commands, and expected outcomes
- Queue execution is intentionally limited to sequential batch opening in this phase

## Type Consistency Check

- `batchQueueMode` uses only `"generate" | "create_tasks"`
- `queuedBatchIds` stores persisted batch ids only
- homepage emits queue requests through `onOpenBatchQueue`
- workbench remains the only place that calls `handleLoadBatch()`
