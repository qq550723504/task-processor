# Studio Generation Panel Controller Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the deterministic `SheinStudioGenerationPanel` props projection out of `SheinStudioWorkbench` and into the existing generation controller without changing behavior.

**Architecture:** Export a named panel props contract, add a pure controller builder with grouped action/form/status inputs, and make the Workbench render the builder result. The builder owns only deterministic projection; Workbench continues to own state, refs, effects, persistence, API calls, and orchestration.

**Tech Stack:** Next.js 16, React 19, TypeScript 6, Vitest 4, Testing Library.

## Global Constraints

- Preserve every current callback, label, notice, request, persistence action, and rendered result.
- Do not change reducer or state ownership.
- Do not introduce React Context, a feature store, or a state-machine library.
- Do not change API modules or split `shein-studio-batch-drafts.ts`.
- Do not change UI structure, styling, copy, or accessibility behavior.
- Do not change async-job, persistence, batch-run, task-creation, or retry sequencing.
- Do not move unrelated Workbench sections.
- Follow red-green-refactor: no production-code step starts until its named test has failed for the expected reason.

---

### Task 1: Define the panel contract and generation-mode projection

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-panel.tsx:34-138`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-controller.ts`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-controller.test.tsx`

**Interfaces:**
- Consumes: existing `SheinStudioGenerationActions`, `SheinStudioGenerationFormModel`, `SheinStudioGenerationStatusModel`, `SheinStudioBatchDetail`, `SDSProductVariantSelection`, and `GroupedSDSSelectionEligibility`.
- Produces:

```ts
export type SheinStudioGenerationPanelProps = {
  actions: SheinStudioGenerationActions;
  form: SheinStudioGenerationFormModel;
  status: SheinStudioGenerationStatusModel;
};

export type SheinStudioGenerationPanelProjectionInput = {
  actions: SheinStudioGenerationPanelActionProjection;
  form: SheinStudioGenerationFormModel;
  status: SheinStudioGenerationPanelStatusProjection;
};

export function buildSheinStudioGenerationPanelProps(
  input: SheinStudioGenerationPanelProjectionInput,
): SheinStudioGenerationPanelProps;
```

- [ ] **Step 1: Install the frontend dependencies in the isolated worktree**

Run from `web/listingkit-ui`:

```powershell
npm.cmd ci
```

Expected: exit code `0` and a clean lockfile/worktree after installation.

- [ ] **Step 2: Write the failing normal and retry-mode projection tests**

Add imports for the desired builder and types, then add this focused helper and test to `shein-studio-generation-controller.test.tsx`:

```ts
import type {
  SheinStudioGenerationFormModel,
  SheinStudioGenerationPanelProps,
} from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import {
  buildSheinStudioGenerationPanelProps,
  type SheinStudioGenerationPanelProjectionInput,
} from "@/components/listingkit/shein-studio/shein-studio-generation-controller";

function buildGenerationPanelProjectionInput(
  overrides: Partial<SheinStudioGenerationPanelProjectionInput> = {},
): SheinStudioGenerationPanelProjectionInput {
  const generate = vi.fn();
  return {
    actions: {
      generate,
      retryFailedItem: vi.fn().mockResolvedValue(undefined),
      retryFailedItems: vi.fn().mockResolvedValue(undefined),
    } as unknown as SheinStudioGenerationPanelProjectionInput["actions"],
    form: {} as SheinStudioGenerationFormModel,
    status: {
      activeSelection: selection,
      createdTasks: [],
      creatingError: "",
      creatingMessage: "",
      currentStoreLabel: "Store 869",
      generationError: "",
      groupedSelections: [],
      hasRetryableFailedItems: false,
      initialBatchId: undefined,
      isCreatingTasks: false,
      isGenerating: false,
      itemizedBatchDetail: undefined,
      retryableFailedItemCount: 0,
      retryingFailedItemId: "",
      savedBatches: [],
      saveMessage: "",
      selectedStyleCount: 2,
      storeRequiredMessage: "",
      subscriptionBlockedMessage: "",
    },
    ...overrides,
  };
}

describe("buildSheinStudioGenerationPanelProps", () => {
  it("projects the normal generation panel contract", () => {
    const input = buildGenerationPanelProjectionInput();

    const result: SheinStudioGenerationPanelProps =
      buildSheinStudioGenerationPanelProps(input);

    expect(result.actions.onGenerate).toBe(input.actions.generate);
    expect(result.form).toBe(input.form);
    expect(result.status).toMatchObject({
      batchProductCount: 1,
      batchStoreLabel: "Store 869",
      createTaskButtonLabel: "生成 SHEIN 资料",
      failedBatchItems: [],
      generateButtonLabel: "生成款式图",
      generationNotice: "",
      isRetryingFailedItems: false,
      selectedStyleCount: 2,
      selectionReady: true,
      showSavedBatches: true,
    });
  });

  it("projects retry mode as the generation action", () => {
    const retryFailedItems = vi.fn().mockResolvedValue(undefined);
    const base = buildGenerationPanelProjectionInput();
    const input = buildGenerationPanelProjectionInput({
      actions: {
        ...base.actions,
        retryFailedItems,
      },
      status: {
        ...base.status,
        hasRetryableFailedItems: true,
        retryableFailedItemCount: 1,
      },
    });

    const result = buildSheinStudioGenerationPanelProps(input);
    result.actions.onGenerate();

    expect(retryFailedItems).toHaveBeenCalledTimes(1);
    expect(result.status).toMatchObject({
      failedBatchItems: [],
      generateButtonLabel: "重试失败批次",
      generationNotice:
        "当前批次有 1 个失败项。点击“重试失败批次”只会重试失败部分，不会重复生成已成功内容。",
      isRetryingFailedItems: true,
    });
  });
});
```

- [ ] **Step 3: Run the focused test and verify RED**

Run from `web/listingkit-ui`:

```powershell
npm.cmd test -- src/components/listingkit/shein-studio/shein-studio-generation-controller.test.tsx
```

Expected: FAIL because `buildSheinStudioGenerationPanelProps` and its input type do not exist.

- [ ] **Step 4: Export the named panel props contract**

In `shein-studio-generation-panel.tsx`, add:

```ts
export type SheinStudioGenerationPanelProps = {
  actions: SheinStudioGenerationActions;
  form: SheinStudioGenerationFormModel;
  status: SheinStudioGenerationStatusModel;
};
```

Change the component signature to:

```ts
export function SheinStudioGenerationPanel({
  actions,
  form,
  status,
}: SheinStudioGenerationPanelProps) {
```

- [ ] **Step 5: Add the projection input types**

In `shein-studio-generation-controller.ts`, import the panel models and props,
`SheinStudioBatchDetail`, `SDSProductVariantSelection`,
`GroupedSDSSelectionEligibility`, and `countSelectionsWithPrimary`. Add:

```ts
type ProjectedGenerationStatusKey =
  | "batchProductCount"
  | "batchStoreLabel"
  | "createTaskButtonLabel"
  | "failedBatchItems"
  | "failedTasks"
  | "generateButtonLabel"
  | "generationNotice"
  | "isRetryingFailedItems"
  | "rejectedTasks"
  | "reusedTasks"
  | "selectionReady"
  | "showSavedBatches"
  | "statusGroups";

export type SheinStudioGenerationPanelActionProjection = Omit<
  SheinStudioGenerationActions,
  "onGenerate" | "onRetryFailedItem"
> & {
  generate: SheinStudioGenerationActions["onGenerate"];
  retryFailedItem: (itemId: string) => Promise<void>;
  retryFailedItems: () => Promise<void>;
};

export type SheinStudioGenerationPanelStatusProjection = Omit<
  SheinStudioGenerationStatusModel,
  ProjectedGenerationStatusKey
> & {
  activeSelection?: SDSProductVariantSelection;
  currentStoreLabel: string;
  groupedSelections: GroupedSDSSelectionEligibility[];
  hasRetryableFailedItems: boolean;
  initialBatchId?: string;
  itemizedBatchDetail?: SheinStudioBatchDetail | null;
  retryableFailedItemCount: number;
};

export type SheinStudioGenerationPanelProjectionInput = {
  actions: SheinStudioGenerationPanelActionProjection;
  form: SheinStudioGenerationFormModel;
  status: SheinStudioGenerationPanelStatusProjection;
};
```

- [ ] **Step 6: Implement the normal and retry-mode projection without item filtering**

Add the builder with action, label, notice, and mode behavior. Leave
`failedBatchItems` empty until Task 2:

```ts
export function buildSheinStudioGenerationPanelProps({
  actions,
  form,
  status,
}: SheinStudioGenerationPanelProjectionInput): SheinStudioGenerationPanelProps {
  const {
    generate,
    retryFailedItem,
    retryFailedItems,
    ...panelActions
  } = actions;
  const {
    activeSelection,
    currentStoreLabel,
    groupedSelections,
    hasRetryableFailedItems,
    initialBatchId,
    itemizedBatchDetail,
    retryableFailedItemCount,
    ...panelStatus
  } = status;
  const batchProductCount = countSelectionsWithPrimary(
    activeSelection,
    groupedSelections,
  );

  return {
    actions: {
      ...panelActions,
      onGenerate: hasRetryableFailedItems
        ? () => {
            void retryFailedItems();
          }
        : generate,
      onRetryFailedItem: (itemId) => {
        void retryFailedItem(itemId);
      },
    },
    form,
    status: {
      ...panelStatus,
      batchProductCount,
      batchStoreLabel: currentStoreLabel || "未设置",
      createTaskButtonLabel:
        groupedSelections.length > 0
          ? `为 ${batchProductCount} 款商品生成 SHEIN 资料`
          : "生成 SHEIN 资料",
      failedBatchItems: [],
      failedTasks: itemizedBatchDetail?.failedTasks ?? [],
      generateButtonLabel: hasRetryableFailedItems
        ? "重试失败批次"
        : "生成款式图",
      generationNotice: hasRetryableFailedItems
        ? `当前批次有 ${retryableFailedItemCount} 个失败项。点击“重试失败批次”只会重试失败部分，不会重复生成已成功内容。`
        : "",
      isRetryingFailedItems: hasRetryableFailedItems,
      rejectedTasks: itemizedBatchDetail?.rejectedTasks ?? [],
      reusedTasks: itemizedBatchDetail?.reusedTasks ?? [],
      selectionReady: Boolean(activeSelection?.variantId),
      showSavedBatches: !initialBatchId,
      statusGroups: itemizedBatchDetail?.statusGroups,
    },
  };
}
```

- [ ] **Step 7: Run the focused test and verify GREEN**

Run:

```powershell
npm.cmd test -- src/components/listingkit/shein-studio/shein-studio-generation-controller.test.tsx
```

Expected: PASS with no warnings, including the normal and retry-mode tests.

- [ ] **Step 8: Commit Task 1**

```powershell
git add -- `
  web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-panel.tsx `
  web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-controller.ts `
  web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-controller.test.tsx
git diff --cached --check
git commit -m "refactor: define studio generation panel projection"
```

---

### Task 2: Add failed-item projection

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-controller.ts`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-controller.test.tsx`

**Interfaces:**
- Consumes: `buildSheinStudioGenerationPanelProps` and `SheinStudioGenerationPanelProjectionInput` from Task 1.
- Produces: the same builder with the existing failed-item filtering behavior.

- [ ] **Step 1: Write the failing failed-item projection test**

Add inside the existing builder describe block:

```ts
it("projects only failed entries as retryable batch items", () => {
  const failedItem = {
    id: "item-failed",
    batchId: "batch-1",
    targetGroupKey: "size:1000x1000",
    targetGroupLabel: "黑色 M",
    status: "failed" as const,
    selectionCount: 1,
    lastError: "upstream timeout",
    createdAt: "2026-08-08T00:00:00.000Z",
    updatedAt: "2026-08-08T00:01:00.000Z",
  };
  const readyItem = {
    ...failedItem,
    id: "item-ready",
    status: "review_ready" as const,
  };
  const input = buildGenerationPanelProjectionInput({
    status: {
      ...buildGenerationPanelProjectionInput().status,
      hasRetryableFailedItems: true,
      retryableFailedItemCount: 1,
      itemizedBatchDetail: {
        batch: {} as never,
        items: [
          { item: failedItem, designs: [] },
          { item: readyItem, designs: [] },
        ],
      },
    },
  });

  const result = buildSheinStudioGenerationPanelProps(input);

  expect(result.status.failedBatchItems).toEqual([failedItem]);
});
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
npm.cmd test -- src/components/listingkit/shein-studio/shein-studio-generation-controller.test.tsx
```

Expected: FAIL because the builder still returns an empty `failedBatchItems` array.

- [ ] **Step 3: Implement failed-item filtering**

After calculating `batchProductCount`, add:

```ts
const failedBatchItems = hasRetryableFailedItems
  ? (itemizedBatchDetail?.items
      .filter((entry) => entry.item.status === "failed")
      .map((entry) => entry.item) ?? [])
  : [];
```

Replace the literal `failedBatchItems: []` with the projected value:

```ts
status: {
  ...panelStatus,
  batchProductCount,
  batchStoreLabel: currentStoreLabel || "未设置",
  createTaskButtonLabel:
    groupedSelections.length > 0
      ? `为 ${batchProductCount} 款商品生成 SHEIN 资料`
      : "生成 SHEIN 资料",
  failedBatchItems,
  failedTasks: itemizedBatchDetail?.failedTasks ?? [],
  rejectedTasks: itemizedBatchDetail?.rejectedTasks ?? [],
  reusedTasks: itemizedBatchDetail?.reusedTasks ?? [],
  selectionReady: Boolean(activeSelection?.variantId),
  showSavedBatches: !initialBatchId,
  statusGroups: itemizedBatchDetail?.statusGroups,
},
```

- [ ] **Step 4: Run the focused test and verify GREEN**

Run:

```powershell
npm.cmd test -- src/components/listingkit/shein-studio/shein-studio-generation-controller.test.tsx
```

Expected: PASS, including the normal, retry-mode, and failed-item projection tests.

- [ ] **Step 5: Commit Task 2**

```powershell
git add -- `
  web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-controller.ts `
  web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-controller.test.tsx
git diff --cached --check
git commit -m "refactor: project studio generation retry state"
```

---

### Task 3: Route Workbench composition through the controller

**Files:**
- Create: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-boundary.test.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx:19-32,1380-1400,1694-1801`

**Interfaces:**
- Consumes: `buildSheinStudioGenerationPanelProps` from Tasks 1-2.
- Produces: a Workbench that renders `<SheinStudioGenerationPanel {...generationPanelProps} />` and contains no inline panel contract.

- [ ] **Step 1: Write the failing structural boundary test**

Create `shein-studio-generation-boundary.test.ts`:

```ts
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

describe("SheinStudioWorkbench generation boundary", () => {
  it("renders the controller-projected GenerationPanel contract", () => {
    const source = readFileSync(
      join(
        process.cwd(),
        "src/components/listingkit/shein-studio/shein-studio-workbench.tsx",
      ),
      "utf8",
    );

    expect(source).toContain("buildSheinStudioGenerationPanelProps({");
    expect(source).toContain(
      "<SheinStudioGenerationPanel {...generationPanelProps} />",
    );
    expect(source).not.toMatch(
      /<SheinStudioGenerationPanel\s+actions=\{\{/,
    );
  });
});
```

- [ ] **Step 2: Run the boundary test and verify RED**

Run:

```powershell
npm.cmd test -- src/components/listingkit/shein-studio/shein-studio-generation-boundary.test.ts
```

Expected: FAIL because the Workbench still renders three inline object literals and does not call the builder.

- [ ] **Step 3: Import the builder and build the complete projection**

Add `buildSheinStudioGenerationPanelProps` to the existing generation-controller import. After `projectItemizedTaskRecoveryState(...)` and before JSX composition, add:

```ts
const generationPanelProps = buildSheinStudioGenerationPanelProps({
  actions: {
    analyzeReferenceStyle,
    generate: handleGenerate,
    onCreateTasks: handleCreateTasks,
    onDeleteBatch: handleDeleteBatch,
    onLoadBatch: handleLoadBatch,
    onRestorePrompt: handlePromptChange,
    onSaveBatch: handleSaveBatch,
    retryFailedItem: handleRetryFailedItem,
    retryFailedItems: handleRetryFailedItems,
    setArtworkGenerationMode,
    setArtworkModel,
    setGroupedImageMode,
    setHotStyleReferenceBrief,
    setHotStyleReferenceImageUrls: handleHotStyleReferenceImageUrlsChange,
    setHotStyleReferencePrompt,
    setImageStrategy,
    setProductImageCount,
    setProductImagePrompt,
    setProductImagePrompts,
    setPrompt: handlePromptChange,
    setPromptMode,
    setRenderSizeImagesWithSds,
    setSelectedSdsImages: (value) => {
      hasCustomizedSdsSelectionRef.current = true;
      setSelectedSdsImages(value);
    },
    setStyleCount,
    setTransparentBackground,
    setTransparentBackgroundMode,
    setVariationIntensity,
    uploadHotStyleReferenceImages,
  },
  form: {
    artworkGenerationMode,
    artworkModel,
    availableSdsImages,
    groupedImageMode,
    hotStyleReferenceBrief,
    hotStyleReferenceImageUrls,
    hotStyleReferencePrompt,
    imageStrategy,
    productImageCount,
    productImagePrompt,
    productImagePrompts,
    prompt,
    promptHistory: activeGroupPromptHistory,
    promptInputRef,
    promptMode,
    renderSizeImagesWithSds,
    selectedSdsImages,
    styleCount,
    transparentBackground,
    transparentBackgroundMode,
    variationIntensity,
  },
  status: {
    activeSelection,
    createdTasks,
    creatingError,
    creatingMessage,
    currentStoreLabel,
    generationError,
    groupedSelections,
    hasRetryableFailedItems,
    initialBatchId,
    isCreatingTasks,
    isGenerating: effectiveIsGenerating,
    itemizedBatchDetail,
    retryableFailedItemCount,
    retryingFailedItemId,
    savedBatches,
    saveMessage,
    selectedStyleCount: selectedIds.length,
    storeRequiredMessage,
    subscriptionBlockedMessage,
  },
});
```

- [ ] **Step 4: Replace the inline JSX contract**

Replace the current `actions={{...}}`, `form={{...}}`, and `status={{...}}` block with:

```tsx
<SheinStudioGenerationPanel {...generationPanelProps} />
```

Remove the Workbench import of `countSelectionsWithPrimary` if no other call remains. Keep `buildGroupedSDSSelectionID` imported.

- [ ] **Step 5: Run boundary and focused behavior tests**

Run:

```powershell
npm.cmd test -- `
  src/components/listingkit/shein-studio/shein-studio-generation-boundary.test.ts `
  src/components/listingkit/shein-studio/shein-studio-generation-controller.test.tsx `
  src/components/listingkit/shein-studio/shein-studio-generation-panel.test.tsx `
  src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx
```

Expected: PASS with no warnings or changed snapshots.

- [ ] **Step 6: Run typecheck and lint for the integrated boundary**

Run:

```powershell
npm.cmd run typecheck
npm.cmd run lint
```

Expected: both commands exit `0`.

- [ ] **Step 7: Commit Task 3**

```powershell
git add -- `
  web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-boundary.test.ts `
  web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx
git diff --cached --check
git commit -m "refactor: route studio generation panel through controller"
```

---

### Task 4: Verify the complete frontend change

**Files:**
- Verify only; no new production files.

**Interfaces:**
- Consumes: the completed controller projection and Workbench integration.
- Produces: current-baseline evidence for the complete frontend gate.

- [ ] **Step 1: Run the full frontend test suite**

Run from `web/listingkit-ui`:

```powershell
npm.cmd test
```

Expected: all Vitest files and tests pass.

- [ ] **Step 2: Run the production frontend build**

Run:

```powershell
npm.cmd run build
```

Expected: Next.js production build exits `0`.

- [ ] **Step 3: Re-run static gates after the build**

Run:

```powershell
npm.cmd run lint
npm.cmd run typecheck
```

Expected: both commands exit `0`.

- [ ] **Step 4: Inspect the final scope**

Run from the worktree root:

```powershell
git status --short
git diff master...HEAD --stat
git diff master...HEAD --check
git diff master...HEAD -- `
  web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-panel.tsx `
  web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-controller.ts `
  web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-controller.test.tsx `
  web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-boundary.test.ts `
  web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx
```

Expected:

- worktree clean;
- no files outside the approved spec, plan, panel contract, controller, tests, and Workbench integration;
- no whitespace errors;
- no API, persistence, UI-copy, or styling changes.
