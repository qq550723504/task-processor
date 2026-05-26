# ListingKit SDS Multi-Group Prompt History Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add multi-group auto-load to `/listing-kits/sds`, so users can reopen the page, see previously saved groups immediately, and continue working with per-group current prompt and prompt history.

**Architecture:** Introduce a first-class `groups[]` workspace model at the studio draft/batch layer, add legacy normalization from the current single-group shape, and project one selected group into the existing workbench editor. Keep current generation and task-creation internals intact by editing only one active group at a time.

**Tech Stack:** Next.js App Router, React state/reducer hooks, existing studio session draft APIs, Vitest, TypeScript

---

### Task 1: Define Multi-Group Types and Normalization

**Files:**
- Modify: `web/listingkit-ui/src/lib/types/shein-studio.ts`
- Modify: `web/listingkit-ui/src/lib/shein-studio/storage-shared.ts`
- Test: `web/listingkit-ui/src/lib/shein-studio/storage-shared.test.ts`

- [ ] **Step 1: Write failing normalization tests for `groups[]` and legacy fallback**

Add tests that prove:

```ts
it("normalizes explicit multi-group drafts", () => {
  const draft = normalizeDraft({
    prompt: "legacy top-level prompt",
    groups: [
      {
        id: "group-1",
        name: "Group 1",
        currentPrompt: "prompt a",
        promptHistory: [{ prompt: "prompt old", groupedImageMode: "shared_by_size", createdAt: "2026-05-26T00:00:00Z" }],
        primarySelection: {
          variantId: 100,
          parentProductId: 1,
          productId: 1,
          prototypeGroupId: 200,
          layerId: "layer-1",
          productName: "tee",
          variantLabel: "M / black",
        },
        groupedSelections: [],
        sheinStoreId: "869",
        imageStrategy: "sds_official",
        groupedImageMode: "shared_by_size",
        selectedSdsImages: [],
        renderSizeImagesWithSds: true,
        productImageCount: "5",
        productImagePrompt: "",
        productImagePrompts: [],
        artworkModel: "",
        transparentBackground: false,
        variationIntensity: "medium",
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T00:00:00Z",
      },
    ],
    updatedAt: "2026-05-26T00:00:00Z",
  });

  expect(draft?.groups).toHaveLength(1);
  expect(draft?.groups?.[0].currentPrompt).toBe("prompt a");
});

it("synthesizes one group from legacy groupedSelections drafts", () => {
  const draft = normalizeDraft({
    prompt: "legacy prompt",
    groupedSelections: [
      {
        selectionId: "1:200:101:layer-2:101",
        selection: {
          variantId: 101,
          parentProductId: 1,
          productId: 1,
          prototypeGroupId: 200,
          layerId: "layer-2",
          productName: "hoodie",
          variantLabel: "L / white",
        },
        sheinStoreId: "869",
        baselineStatus: "ready",
        baselineReason: "",
        eligible: true,
      },
    ],
    selection: {
      variantId: 100,
      parentProductId: 1,
      productId: 1,
      prototypeGroupId: 200,
      layerId: "layer-1",
      productName: "tee",
      variantLabel: "M / black",
    },
    selectedIds: ["design-1"],
    designs: [],
    createdTasks: [],
    updatedAt: "2026-05-26T00:00:00Z",
  });

  expect(draft?.groups).toHaveLength(1);
  expect(draft?.groups?.[0].currentPrompt).toBe("legacy prompt");
  expect(draft?.groups?.[0].groupedSelections).toHaveLength(1);
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
npm test -- storage-shared.test.ts
```

Expected: FAIL because `groups` and legacy synthesis do not exist yet.

- [ ] **Step 3: Add new group and prompt-history types**

Extend `web/listingkit-ui/src/lib/types/shein-studio.ts` with explicit types:

```ts
export type SDSGroupedPromptHistoryEntry = {
  prompt: string;
  groupedImageMode: SheinStudioGroupedImageMode;
  createdAt: string;
};

export type SheinStudioGroupedWorkspace = {
  id: string;
  name: string;
  primarySelection: SDSProductVariantSelection;
  groupedSelections: GroupedSDSSelectionEligibility[];
  sheinStoreId: string;
  imageStrategy: SheinStudioImageStrategy;
  groupedImageMode: SheinStudioGroupedImageMode;
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  renderSizeImagesWithSds: boolean;
  currentPrompt: string;
  promptHistory: SDSGroupedPromptHistoryEntry[];
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  artworkModel: SheinStudioArtworkModel;
  transparentBackground: boolean;
  variationIntensity: SheinStudioVariationIntensity;
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
  updatedAt: string;
};
```

- [ ] **Step 4: Implement `groups[]` normalization and legacy migration**

Update `web/listingkit-ui/src/lib/shein-studio/storage-shared.ts` to:

- normalize `groups[]` when present
- synthesize one group from legacy top-level `selection`, `groupedSelections`, `prompt`, `selectedIds`, `designs`, `createdTasks`
- preserve existing single-group draft shape for backward reads

Add focused helpers such as:

```ts
function normalizePromptHistory(value: unknown): SDSGroupedPromptHistoryEntry[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .filter((item): item is SDSGroupedPromptHistoryEntry => {
      return (
        !!item &&
        typeof item === "object" &&
        typeof item.prompt === "string" &&
        typeof item.createdAt === "string" &&
        (item.groupedImageMode === "shared_by_size" || item.groupedImageMode === "per_product")
      );
    })
    .slice(0, 5);
}
```

- [ ] **Step 5: Run tests to verify normalization passes**

Run:

```bash
npm test -- storage-shared.test.ts
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/listingkit-ui/src/lib/types/shein-studio.ts web/listingkit-ui/src/lib/shein-studio/storage-shared.ts web/listingkit-ui/src/lib/shein-studio/storage-shared.test.ts
git commit -m "feat: add SDS multi-group storage model"
```

### Task 2: Persist Groups Through Draft and Batch APIs

**Files:**
- Modify: `web/listingkit-ui/src/lib/shein-studio/draft-input.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio-sessions.ts`
- Modify: `web/listingkit-ui/src/lib/utils/shein-studio-batches.ts`
- Test: `web/listingkit-ui/src/lib/shein-studio/draft-input.test.ts`
- Test: `web/listingkit-ui/src/lib/api/shein-studio.test.ts`
- Test: `web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts`

- [ ] **Step 1: Write failing persistence tests for `groups[]`**

Add tests that verify:

```ts
expect(payload.groups).toEqual([
  expect.objectContaining({
    id: "group-1",
    currentPrompt: "prompt a",
    promptHistory: [
      expect.objectContaining({ prompt: "prompt old" }),
    ],
  }),
]);
```

and:

```ts
expect(requestBody.grouped_selections).toBeUndefined();
expect(requestBody.groups).toHaveLength(1);
```

for both draft save and batch save paths.

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
npm test -- draft-input.test.ts shein-studio.test.ts shein-studio-batches.test.ts
```

Expected: FAIL because the API payloads do not yet include `groups`.

- [ ] **Step 3: Extend save input and request mappers**

Update `web/listingkit-ui/src/lib/utils/shein-studio-batches.ts` and `web/listingkit-ui/src/lib/shein-studio/draft-input.ts` so `SheinStudioSaveInput` carries:

```ts
groups?: SheinStudioGroupedWorkspace[];
```

and draft builders populate it.

- [ ] **Step 4: Extend session API payload mapping**

Update `web/listingkit-ui/src/lib/api/shein-studio-sessions.ts` to send and read:

```ts
groups: patch.groups?.map(groupToPayload)
```

using a dedicated mapper:

```ts
function groupToPayload(group: SheinStudioGroupedWorkspace) {
  return {
    id: group.id,
    name: group.name,
    current_prompt: group.currentPrompt,
    prompt_history: group.promptHistory.map((entry) => ({
      prompt: entry.prompt,
      grouped_image_mode: entry.groupedImageMode,
      created_at: entry.createdAt,
    })),
    primary_selection: selectionToPayload(group.primarySelection),
    grouped_selections: group.groupedSelections.map(groupedSelectionToPayload),
    shein_store_id: group.sheinStoreId,
    image_strategy: group.imageStrategy,
    grouped_image_mode: group.groupedImageMode,
    selected_sds_images: group.selectedSdsImages,
    render_size_images_with_sds: group.renderSizeImagesWithSds,
    product_image_count: group.productImageCount,
    product_image_prompt: group.productImagePrompt,
    product_image_prompts: group.productImagePrompts,
    artwork_model: group.artworkModel,
    transparent_background: group.transparentBackground,
    variation_intensity: group.variationIntensity,
    designs: group.designs,
    approved_design_ids: group.selectedIds,
    created_tasks: group.createdTasks,
    updated_at: group.updatedAt,
  };
}
```

- [ ] **Step 5: Run persistence tests to verify they pass**

Run:

```bash
npm test -- draft-input.test.ts shein-studio.test.ts shein-studio-batches.test.ts
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/listingkit-ui/src/lib/shein-studio/draft-input.ts web/listingkit-ui/src/lib/api/shein-studio-sessions.ts web/listingkit-ui/src/lib/utils/shein-studio-batches.ts web/listingkit-ui/src/lib/shein-studio/draft-input.test.ts web/listingkit-ui/src/lib/api/shein-studio.test.ts web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts
git commit -m "feat: persist SDS grouped workspaces"
```

### Task 3: Add Active Group State to the Workbench

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-hooks.ts`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.test.ts`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.test.ts`

- [ ] **Step 1: Write failing reducer tests for active-group projection**

Add tests proving:

```ts
expect(next.groups).toHaveLength(2);
expect(next.activeGroupId).toBe("group-2");
expect(next.prompt).toBe("group 2 prompt");
expect(next.groupedSelections).toEqual(groupTwo.groupedSelections);
```

and:

```ts
const next = reducer(state, selectGroup("group-1"));
expect(next.prompt).toBe("group 1 prompt");
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
npm test -- shein-studio-workbench-state.test.ts shein-studio-workbench-model.test.ts
```

Expected: FAIL because there is no multi-group workbench state yet.

- [ ] **Step 3: Extend workbench state with groups and active group id**

Update `shein-studio-workbench-state.ts` with:

```ts
groups: SheinStudioGroupedWorkspace[];
activeGroupId: string;
```

and add reducer actions to:

- apply groups from draft/batch
- select active group
- sync current editor fields back into the active group

- [ ] **Step 4: Add projection helpers between group and editor fields**

Add helper functions in `shein-studio-workbench-model.ts` similar to:

```ts
export function projectGroupToWorkbench(group: SheinStudioGroupedWorkspace) {
  return {
    prompt: group.currentPrompt,
    sheinStoreId: group.sheinStoreId,
    imageStrategy: group.imageStrategy,
    groupedImageMode: group.groupedImageMode,
    selectedSdsImages: group.selectedSdsImages,
    renderSizeImagesWithSds: group.renderSizeImagesWithSds,
    groupedSelections: group.groupedSelections,
    designs: group.designs,
    selectedIds: group.selectedIds,
    createdTasks: group.createdTasks,
  };
}
```

- [ ] **Step 5: Run reducer/model tests to verify they pass**

Run:

```bash
npm test -- shein-studio-workbench-state.test.ts shein-studio-workbench-model.test.ts
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-hooks.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.test.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.test.ts
git commit -m "feat: add active SDS grouped workspace state"
```

### Task 4: Auto-Load Recent Groups on `/listing-kits/sds`

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-workspace.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`

- [ ] **Step 1: Write failing page-entry restoration tests**

Add tests to prove:

```ts
it("loads saved groups on page entry without requiring reselecting the original product", async () => {
  mockedLoadDraft.mockResolvedValue({
    prompt: "legacy top-level",
    groups: [
      { id: "group-1", name: "Group 1", currentPrompt: "prompt a", ...groupOne },
      { id: "group-2", name: "Group 2", currentPrompt: "prompt b", ...groupTwo },
    ],
  });

  render(<SheinStudioWorkbench />);

  expect(await screen.findByText("Group 2")).toBeInTheDocument();
  expect(screen.getByDisplayValue("prompt b")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
npm test -- shein-studio-workbench.test.tsx
```

Expected: FAIL because the workbench still requires an active selection-backed draft load.

- [ ] **Step 3: Update workspace loader to prefer saved groups**

Modify `shein-studio-workbench-workspace.ts` so page entry:

- loads draft data even when there is no current active selection, using cached session detail or a “last session” helper if needed
- if `groups[]` exist, selects the most recently updated group
- hydrates editor state from the active group

If no multi-group data exists, continue existing single-selection behavior.

- [ ] **Step 4: Update workbench rendering to show recent groups before active selection exists**

In `shein-studio-workbench.tsx`, render a `Recent Groups` area above the editor. For this phase it can be a simple list of buttons showing:

- group name
- primary product
- updated time

Selecting a group should project it into the current editor state.

- [ ] **Step 5: Run page restoration tests to verify they pass**

Run:

```bash
npm test -- shein-studio-workbench.test.tsx
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-workspace.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx
git commit -m "feat: auto-load recent SDS grouped workspaces"
```

### Task 5: Add Per-Group Prompt History

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-form-sections.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-actions.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-panel.test.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`

- [ ] **Step 1: Write failing prompt-history tests**

Add tests that prove:

```ts
it("appends the active prompt to the group's history when generating", async () => {
  // generate with current prompt
  expect(updatedGroup.promptHistory[0]).toEqual(
    expect.objectContaining({ prompt: "new prompt", groupedImageMode: "shared_by_size" }),
  );
});

it("restores a historic prompt into the current prompt field", async () => {
  fireEvent.click(screen.getByText("restore-prompt-old"));
  expect(screen.getByDisplayValue("prompt old")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
npm test -- shein-studio-generation-panel.test.tsx shein-studio-workbench.test.tsx
```

Expected: FAIL because there is no prompt-history UI or append logic yet.

- [ ] **Step 3: Append prompt history on generation**

In `shein-studio-workbench-actions.ts`, before dispatching generation, update the active group:

```ts
const historyEntry = {
  prompt: prompt.trim(),
  groupedImageMode,
  createdAt: new Date().toISOString(),
};
```

Rules:

- only append if `prompt.trim()` is non-empty
- do not append if it matches the newest existing history entry
- keep only the latest 5 entries

- [ ] **Step 4: Render prompt-history UI for the active group**

In `shein-studio-generation-form-sections.tsx`, add a compact section:

```tsx
{promptHistory.length > 0 ? (
  <div>
    <div className="text-xs font-medium">最近使用过的提示词</div>
    {promptHistory.map((entry) => (
      <button key={entry.createdAt} onClick={() => onRestorePrompt(entry.prompt)} type="button">
        {entry.prompt}
      </button>
    ))}
  </div>
) : null}
```

Use existing visual language from the studio form; do not introduce a modal in phase 1.

- [ ] **Step 5: Run prompt-history tests to verify they pass**

Run:

```bash
npm test -- shein-studio-generation-panel.test.tsx shein-studio-workbench.test.tsx
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-form-sections.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-actions.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-panel.test.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx
git commit -m "feat: add per-group prompt history"
```

### Task 6: End-to-End Verification and Legacy Regression Coverage

**Files:**
- Modify: `web/listingkit-ui/src/lib/api/shein-studio.test.ts`
- Modify: `web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-detail.test.tsx`

- [ ] **Step 1: Add regression tests for legacy draft compatibility**

Add tests that verify a legacy draft without `groups[]` still:

- loads into one synthesized group
- preserves grouped selections
- preserves selected ids and created tasks

- [ ] **Step 2: Run focused regression suite**

Run:

```bash
npm test -- shein-studio.test.ts shein-studio-batches.test.ts shein-studio-batch-detail.test.tsx shein-studio-workbench.test.tsx
```

Expected: PASS

- [ ] **Step 3: Run full typecheck**

Run:

```bash
npm run typecheck
```

Expected: PASS

- [ ] **Step 4: Run final grouped SDS verification suite**

Run:

```bash
npm test -- grouped-sds-create.test.ts draft-input.test.ts shein-studio-workbench-state.test.ts shein-studio-generation-panel.test.tsx shein-studio-grouped-selection-panel.test.tsx shein-studio-workbench.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/lib/api/shein-studio.test.ts web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-detail.test.tsx
git commit -m "test: cover SDS multi-group legacy compatibility"
```

## Spec Coverage Check

- Multi-group model: covered by Tasks 1-4
- Auto-load recent groups on `/listing-kits/sds`: covered by Task 4
- Per-group current prompt: covered by Tasks 3-5
- Per-group prompt history: covered by Task 5
- Legacy single-group migration: covered by Tasks 1, 2, and 6
- Keep existing generation/task fan-out behavior: preserved by Tasks 3-6 without changing task-creation contract

## Placeholder Scan

- No `TODO` or `TBD` placeholders remain
- Every task includes exact files, commands, and expected outcomes
- Each coding task includes concrete structures or snippets rather than generic directions

## Type Consistency Check

- `SheinStudioGroupedWorkspace` is the single new persisted group type across tasks
- `SDSGroupedPromptHistoryEntry` is used consistently for group-local prompt history
- `currentPrompt` is the editable per-group prompt, while `promptHistory` is append-only generation history
