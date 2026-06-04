# Shein Studio Workbench Persistence Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move Shein Studio workbench persistence to a detail-truth plus view-state model while keeping old local data readable through one compatibility adapter.

**Architecture:** The implementation keeps `itemizedBatchDetail` as the only result-state owner, introduces explicit persisted view and legacy fallback shapes, and centralizes merge precedence in workbench model helpers. Storage decode remains backward compatible, while new draft writes stop persisting compatibility-era result fields.

**Tech Stack:** TypeScript, React, Next.js, Vitest, local storage helpers, existing Shein Studio batch APIs

---

## File Map

- `web/listingkit-ui/src/lib/types/shein-studio.ts`
  Defines persisted view, grouped workspace view, legacy compatibility snapshot, and updated draft/batch types.
- `web/listingkit-ui/src/lib/shein-studio/draft-input.ts`
  Builds the persisted draft payload written by the workbench.
- `web/listingkit-ui/src/lib/shein-studio/draft-input.test.ts`
  Verifies new draft writes exclude result-state compatibility fields.
- `web/listingkit-ui/src/lib/utils/shein-studio-batches.ts`
  Decodes old and new storage shapes, preserves legacy fallback data, and feeds hydration helpers.
- `web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts`
  Covers old-shape compatibility reads and new-shape write expectations.
- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.ts`
  Centralizes merge precedence and projection helpers for draft restore and hydrated batches.
- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.test.ts`
  Verifies detail-first projection, legacy fallback behavior, and grouped workspace view restoration.
- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-hooks.ts`
  Narrows draft persistence input to view-state fields and removes result ownership from write paths.
- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.ts`
  Shrinks `SheinStudioWorkbenchDraftPatch` to view-state fields and updates hydrated batch action usage.
- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-actions.ts`
  Routes apply and hydrate flows through the centralized model helpers.
- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`
  Uses the cleaned merge flow without directly trusting persisted flat result fields.
- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`
  Covers draft restore, pre-hydration fallback display, and hydrated override behavior end-to-end.

### Task 1: Lock the persistence contract in tests

**Files:**
- Modify: `web/listingkit-ui/src/lib/shein-studio/draft-input.test.ts`
- Modify: `web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts`

- [ ] **Step 1: Add a failing draft-input test that forbids new flat result ownership**

```typescript
it("omits compatibility-era result fields from new draft writes", () => {
  const payload = buildSheinStudioDraftInput({
    prompt: "retro cherries",
    styleCount: "2",
    variationIntensity: "medium",
    productImageCount: "5",
    productImagePrompt: "hero image",
    productImagePrompts: [],
    artworkModel: "nanobanana",
    transparentBackground: false,
    sheinStoreId: "7",
    imageStrategy: "ai_generated",
    groupedImageMode: "shared_by_size",
    selectedSdsImages: [],
    renderSizeImagesWithSds: true,
    selection: {
      productId: 1,
      parentProductId: 1,
      variantId: 2,
      prototypeGroupId: 3,
      layerId: "layer-1",
      productName: "tee",
      variantLabel: "M / black",
    },
    groupedSelections: [],
    groups: [
      {
        id: "group-1",
        name: "Group 1",
        primarySelection: {
          productId: 1,
          parentProductId: 1,
          variantId: 2,
          prototypeGroupId: 3,
          layerId: "layer-1",
          productName: "tee",
          variantLabel: "M / black",
        },
        groupedSelections: [],
        sheinStoreId: "7",
        imageStrategy: "ai_generated",
        groupedImageMode: "shared_by_size",
        selectedSdsImages: [],
        renderSizeImagesWithSds: true,
        currentPrompt: "group prompt",
        promptHistory: [],
        productImageCount: "5",
        productImagePrompt: "",
        productImagePrompts: [],
        artworkModel: "nanobanana",
        transparentBackground: false,
        variationIntensity: "medium",
        designs: [{ id: "legacy-design" }],
        selectedIds: ["legacy-design"],
        createdTasks: [{ id: "task-1", taskId: "task-1", title: "legacy" }],
        updatedAt: "2026-06-04T00:00:00Z",
      },
    ],
    designs: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
    selectedIds: ["design-1"],
    createdTasks: [{ id: "task-1", taskId: "task-1", title: "legacy" }],
  });

  expect(payload).not.toHaveProperty("designs");
  expect(payload).not.toHaveProperty("selectedIds");
  expect(payload).not.toHaveProperty("createdTasks");
  expect(payload).not.toHaveProperty("generationJobs");
  expect(payload.groups?.[0]).not.toHaveProperty("designs");
  expect(payload.groups?.[0]).not.toHaveProperty("selectedIds");
  expect(payload.groups?.[0]).not.toHaveProperty("createdTasks");
});
```

- [ ] **Step 2: Run the targeted draft-input test and confirm it fails**

Run: `npm test -- --run web/listingkit-ui/src/lib/shein-studio/draft-input.test.ts`
Expected: FAIL because the payload still includes legacy result fields.

- [ ] **Step 3: Add a failing storage compatibility test for old-shape reads and new-shape writes**

```typescript
it("maps legacy stored batches into view state plus compatibility snapshot", async () => {
  upsertSheinStudioBatchDraft.mockResolvedValue({
    id: "batch-1",
    name: "Saved Batch",
    prompt: "legacy prompt",
    styleCount: "2",
    sheinStoreId: "42",
    selection,
    designs: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
    selectedIds: ["design-1"],
    createdTasks: [],
    groups: [
      {
        id: "group-1",
        name: "Group 1",
        currentPrompt: "legacy group prompt",
        promptHistory: [],
        primarySelection: selection,
        groupedSelections: [],
        sheinStoreId: "42",
        imageStrategy: "hybrid",
        groupedImageMode: "shared_by_size",
        selectedSdsImages: [],
        renderSizeImagesWithSds: true,
        productImageCount: "5",
        productImagePrompt: "",
        productImagePrompts: [],
        artworkModel: "",
        transparentBackground: false,
        variationIntensity: "medium",
        designs: [{ id: "group-design-1" }],
        selectedIds: ["group-design-1"],
        createdTasks: [],
        updatedAt: "2026-06-04T00:00:00Z",
      },
    ],
    updatedAt: "2026-06-04T00:00:00Z",
  });

  const saved = await saveSheinStudioBatch({
    prompt: "legacy prompt",
    styleCount: "2",
    sheinStoreId: "42",
    selection,
    groupedSelections: [],
    groups: [],
    designs: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
    selectedIds: ["design-1"],
    createdTasks: [],
  });

  expect(saved?.prompt).toBe("legacy prompt");
  expect(saved?.designs).toEqual([{ id: "design-1", imageUrl: "https://example.com/design.png" }]);
  expect(upsertSheinStudioBatchDraft).toHaveBeenCalledWith(
    expect.not.objectContaining({
      designs: expect.anything(),
      selectedIds: expect.anything(),
      createdTasks: expect.anything(),
      generationJobs: expect.anything(),
    }),
  );
});
```

- [ ] **Step 4: Run the storage test and confirm it fails**

Run: `npm test -- --run web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts`
Expected: FAIL because save calls still include legacy result fields and read paths do not isolate them as fallback-only data.

- [ ] **Step 5: Commit the red tests**

```bash
git add web/listingkit-ui/src/lib/shein-studio/draft-input.test.ts web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts
git commit -m "test: lock shein studio persistence contract"
```

### Task 2: Introduce explicit persisted view and compatibility snapshot types

**Files:**
- Modify: `web/listingkit-ui/src/lib/types/shein-studio.ts`
- Modify: `web/listingkit-ui/src/lib/shein-studio/draft-input.ts`

- [ ] **Step 1: Add focused persistence types in `shein-studio.ts`**

```typescript
export type SheinStudioLegacyCompatibilitySnapshot = {
  designs?: SheinStudioGeneratedDesign[];
  selectedIds?: string[];
  createdTasks?: SheinStudioCreatedTask[];
  generationJobs?: SheinStudioGenerationJob[];
};

export type SheinStudioGroupedWorkspaceView = {
  id: string;
  name: string;
  primarySelection: SDSProductVariantSelection;
  groupedSelections: GroupedSDSSelectionEligibility[];
  styleCount?: string;
  sheinStoreId: string;
  imageStrategy?: SheinStudioImageStrategy;
  groupedImageMode?: SheinStudioGroupedImageMode;
  selectedSdsImages?: SheinStudioSelectedSDSImage[];
  renderSizeImagesWithSds?: boolean;
  currentPrompt: string;
  promptHistory: SDSGroupedPromptHistoryEntry[];
  productImageCount?: string;
  productImagePrompt?: string;
  productImagePrompts?: SheinStudioProductImagePrompt[];
  artworkModel?: SheinStudioArtworkModel;
  transparentBackground?: boolean;
  variationIntensity?: SheinStudioVariationIntensity;
  updatedAt: string;
};

export type SheinStudioPersistedBatchView = {
  prompt: string;
  styleCount: string;
  variationIntensity?: SheinStudioVariationIntensity;
  productImageCount?: string;
  productImagePrompt?: string;
  productImagePrompts?: SheinStudioProductImagePrompt[];
  artworkModel?: SheinStudioArtworkModel;
  transparentBackground?: boolean;
  sheinStoreId: string;
  imageStrategy?: SheinStudioImageStrategy;
  groupedImageMode?: SheinStudioGroupedImageMode;
  selectedSdsImages?: SheinStudioSelectedSDSImage[];
  renderSizeImagesWithSds?: boolean;
  selectionVariantId?: number;
  selection?: SDSProductVariantSelection;
  groupedSelections?: GroupedSDSSelectionEligibility[];
  groups?: SheinStudioGroupedWorkspaceView[];
  compatibility?: SheinStudioLegacyCompatibilitySnapshot;
  generationError?: string;
  generationJobId?: string;
  batchStatus?: string;
  draftUpdatedAt?: string;
  updatedAt: string;
};

export type SheinStudioSavedBatch = SheinStudioPersistedBatchView & {
  id: string;
  name: string;
};

export type SheinStudioDraft = SheinStudioPersistedBatchView;
```

- [ ] **Step 2: Run TypeScript on the changed type file to expose downstream breakage**

Run: `npm run typecheck`
Expected: FAIL with compile errors in draft input, workbench model, and persistence hooks that still reference removed flat fields directly.

- [ ] **Step 3: Narrow `buildSheinStudioDraftInput` to emit only view-state fields**

```typescript
export function buildSheinStudioDraftInput(
  args: BuildSheinStudioDraftInputArgs,
): SheinStudioSaveInput {
  return {
    prompt: args.prompt,
    styleCount: args.styleCount,
    variationIntensity: args.variationIntensity,
    productImageCount: args.productImageCount,
    productImagePrompt: args.productImagePrompt,
    productImagePrompts: args.productImagePrompts,
    artworkModel: args.artworkModel,
    transparentBackground: args.transparentBackground,
    sheinStoreId: args.sheinStoreId,
    imageStrategy: args.imageStrategy,
    groupedImageMode: args.groupedImageMode,
    selectedSdsImages: sanitizeSelectedSdsImages(args.selectedSdsImages),
    renderSizeImagesWithSds: args.renderSizeImagesWithSds,
    selection: sanitizeStudioSelection(args.selection),
    groupedSelections: sanitizeGroupedSelections(args.groupedSelections),
    groups: args.groups?.map((group) => ({
      id: group.id,
      name: group.name,
      primarySelection: sanitizeStudioSelection(group.primarySelection)!,
      groupedSelections: sanitizeGroupedSelections(group.groupedSelections),
      sheinStoreId: group.sheinStoreId,
      imageStrategy: group.imageStrategy,
      groupedImageMode: group.groupedImageMode,
      selectedSdsImages: sanitizeSelectedSdsImages(group.selectedSdsImages ?? []),
      renderSizeImagesWithSds: group.renderSizeImagesWithSds,
      currentPrompt: group.currentPrompt,
      promptHistory: group.promptHistory,
      productImageCount: group.productImageCount,
      productImagePrompt: group.productImagePrompt,
      productImagePrompts: group.productImagePrompts,
      artworkModel: group.artworkModel,
      transparentBackground: group.transparentBackground,
      variationIntensity: group.variationIntensity,
      updatedAt: group.updatedAt,
    })),
    updatedAt: new Date().toISOString(),
  };
}
```

- [ ] **Step 4: Re-run the two targeted tests and make them pass**

Run: `npm test -- --run web/listingkit-ui/src/lib/shein-studio/draft-input.test.ts web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts`
Expected: PASS for the new payload-shape assertions, while model and workbench tests may still fail.

- [ ] **Step 5: Commit the type and write-path narrowing**

```bash
git add web/listingkit-ui/src/lib/types/shein-studio.ts web/listingkit-ui/src/lib/shein-studio/draft-input.ts web/listingkit-ui/src/lib/shein-studio/draft-input.test.ts web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts
git commit -m "refactor: narrow shein studio persisted view state"
```

### Task 3: Add storage adapters for old-shape compatibility reads

**Files:**
- Modify: `web/listingkit-ui/src/lib/utils/shein-studio-batches.ts`
- Modify: `web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts`

- [ ] **Step 1: Write a failing adapter test for grouped legacy data**

```typescript
it("preserves legacy result fields as compatibility fallback only", async () => {
  listSheinStudioBatchDrafts.mockResolvedValue([
    {
      id: "batch-legacy",
      name: "Legacy Batch",
      prompt: "legacy prompt",
      styleCount: "3",
      sheinStoreId: "42",
      selection,
      groups: [
        {
          id: "group-1",
          name: "Legacy Group",
          currentPrompt: "legacy group prompt",
          promptHistory: [],
          primarySelection: selection,
          groupedSelections: [],
          sheinStoreId: "42",
          imageStrategy: "hybrid",
          groupedImageMode: "shared_by_size",
          selectedSdsImages: [],
          renderSizeImagesWithSds: true,
          productImageCount: "5",
          productImagePrompt: "",
          productImagePrompts: [],
          artworkModel: "",
          transparentBackground: false,
          variationIntensity: "medium",
          designs: [{ id: "group-design-1" }],
          selectedIds: ["group-design-1"],
          createdTasks: [],
          updatedAt: "2026-06-04T00:00:00Z",
        },
      ],
      designs: [{ id: "design-1" }],
      selectedIds: ["design-1"],
      createdTasks: [],
      generationJobs: [{ id: "job-1", status: "completed" }],
      updatedAt: "2026-06-04T00:00:00Z",
    },
  ]);

  const batches = await listSheinStudioBatches();

  expect(batches[0]?.compatibility).toEqual(
    expect.objectContaining({
      designs: [{ id: "design-1" }],
      selectedIds: ["design-1"],
      generationJobs: [expect.objectContaining({ id: "job-1" })],
    }),
  );
  expect(batches[0]?.groups?.[0]).not.toHaveProperty("designs");
  expect(batches[0]?.groups?.[0]).not.toHaveProperty("selectedIds");
  expect(batches[0]?.groups?.[0]).not.toHaveProperty("createdTasks");
});
```

- [ ] **Step 2: Run the storage test and confirm it fails**

Run: `npm test -- --run web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts`
Expected: FAIL because legacy flat fields are still returned directly on the persisted view shape.

- [ ] **Step 3: Implement decode helpers in `shein-studio-batches.ts`**

```typescript
function stripGroupedWorkspaceResults(
  group: SheinStudioGroupedWorkspace | SheinStudioGroupedWorkspaceView,
): SheinStudioGroupedWorkspaceView {
  return {
    id: group.id,
    name: group.name,
    primarySelection: group.primarySelection,
    groupedSelections: group.groupedSelections,
    styleCount: group.styleCount,
    sheinStoreId: group.sheinStoreId,
    imageStrategy: group.imageStrategy,
    groupedImageMode: group.groupedImageMode,
    selectedSdsImages: group.selectedSdsImages,
    renderSizeImagesWithSds: group.renderSizeImagesWithSds,
    currentPrompt: group.currentPrompt,
    promptHistory: group.promptHistory,
    productImageCount: group.productImageCount,
    productImagePrompt: group.productImagePrompt,
    productImagePrompts: group.productImagePrompts,
    artworkModel: group.artworkModel,
    transparentBackground: group.transparentBackground,
    variationIntensity: group.variationIntensity,
    updatedAt: group.updatedAt,
  };
}

function decodeSavedBatch(
  input: SheinStudioSavedBatch | (SheinStudioSavedBatch & Record<string, unknown>),
): SheinStudioSavedBatch {
  const compatibility: SheinStudioLegacyCompatibilitySnapshot | undefined =
    "compatibility" in input
      ? (input.compatibility as SheinStudioLegacyCompatibilitySnapshot | undefined)
      : {
          designs: Array.isArray(input.designs) ? input.designs : undefined,
          selectedIds: Array.isArray(input.selectedIds) ? input.selectedIds : undefined,
          createdTasks: Array.isArray(input.createdTasks) ? input.createdTasks : undefined,
          generationJobs: Array.isArray(input.generationJobs) ? input.generationJobs : undefined,
        };

  return {
    id: input.id,
    name: input.name,
    prompt: input.prompt,
    styleCount: input.styleCount,
    variationIntensity: input.variationIntensity,
    productImageCount: input.productImageCount,
    productImagePrompt: input.productImagePrompt,
    productImagePrompts: input.productImagePrompts,
    artworkModel: input.artworkModel,
    transparentBackground: input.transparentBackground,
    sheinStoreId: input.sheinStoreId,
    imageStrategy: input.imageStrategy,
    groupedImageMode: input.groupedImageMode,
    selectedSdsImages: input.selectedSdsImages,
    renderSizeImagesWithSds: input.renderSizeImagesWithSds,
    selectionVariantId: input.selectionVariantId,
    selection: input.selection,
    groupedSelections: input.groupedSelections,
    groups: input.groups?.map(stripGroupedWorkspaceResults),
    compatibility,
    generationError: input.generationError,
    generationJobId: input.generationJobId,
    batchStatus: input.batchStatus,
    draftUpdatedAt: input.draftUpdatedAt,
    updatedAt: input.updatedAt,
  };
}
```

- [ ] **Step 4: Re-run the storage tests and verify adapter behavior passes**

Run: `npm test -- --run web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts`
Expected: PASS with old data routed through `compatibility` and grouped workspaces stripped to view-only fields.

- [ ] **Step 5: Commit the storage adapter**

```bash
git add web/listingkit-ui/src/lib/utils/shein-studio-batches.ts web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts
git commit -m "refactor: add shein studio persistence compatibility adapter"
```

### Task 4: Centralize detail-first merge precedence in the workbench model

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.test.ts`

- [ ] **Step 1: Add failing model tests for the new merge precedence**

```typescript
it("uses compatibility fallback only before hydrated detail exists", () => {
  const state = mergeSheinStudioDraftState({
    draft: {
      prompt: "saved prompt",
      styleCount: "2",
      variationIntensity: "medium",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "869",
      imageStrategy: "hybrid",
      groups: [],
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      compatibility: {
        designs: [{ id: "design-1", imageUrl: "https://example.com/a.png" }],
        selectedIds: ["design-1"],
        createdTasks: [],
        generationJobs: [{ id: "job-1", status: "completed" }],
      },
      updatedAt: "2026-05-10T00:00:00.000Z",
    },
  });

  expect(state.designs.map((item) => item.id)).toEqual(["design-1"]);
  expect(state.selectedIds).toEqual(["design-1"]);
  expect(state.generationJobs).toEqual([expect.objectContaining({ id: "job-1" })]);
});

it("drops compatibility fallback once hydrated detail is present", () => {
  const projection = projectHydratedBatchToWorkbench({
    savedBatch: {
      id: "batch-1",
      name: "Saved Batch",
      prompt: "legacy prompt",
      styleCount: "9",
      sheinStoreId: "42",
      compatibility: {
        designs: [{ id: "legacy-design" }],
        selectedIds: ["legacy-design"],
        createdTasks: [],
      },
      updatedAt: "2026-06-01T10:00:00Z",
    },
    detail: buildHydratedBatchDetailFixture(),
  });

  expect(projection.designs.map((item) => item.id)).toEqual(["design-1"]);
  expect(projection.selectedIds).toEqual(["design-1"]);
  expect(projection.prompt).toBe("itemized prompt");
});
```

- [ ] **Step 2: Run the model test file and confirm it fails**

Run: `npm test -- --run web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.test.ts`
Expected: FAIL because merge helpers still read result fields directly from draft and group shapes.

- [ ] **Step 3: Implement centralized helpers and route existing projections through them**

```typescript
function projectCompatibilityFallback(
  compatibility?: SheinStudioLegacyCompatibilitySnapshot,
) {
  return {
    designs: compatibility?.designs ?? [],
    selectedIds: compatibility?.selectedIds ?? [],
    createdTasks: compatibility?.createdTasks ?? [],
    generationJobs: compatibility?.generationJobs ?? [],
  };
}

export function mergePersistedViewWithCompatibilityFallback({
  draft,
  galleryDesign,
  galleryPrompt,
}: {
  draft?: SheinStudioDraft | null;
  galleryDesign?: SheinStudioGeneratedDesign | null;
  galleryPrompt?: string | null;
}) {
  const compatibility = projectCompatibilityFallback(draft?.compatibility);
  const designs =
    galleryDesign && !compatibility.designs.some((design) => design.id === galleryDesign.id)
      ? [...compatibility.designs, galleryDesign]
      : compatibility.designs;
  const selectedIds =
    galleryDesign && !compatibility.selectedIds.includes(galleryDesign.id)
      ? [...compatibility.selectedIds, galleryDesign.id]
      : compatibility.selectedIds;

  return {
    prompt: draft?.prompt || galleryPrompt || "",
    selection: draft?.selection,
    styleCount: draft?.styleCount ?? "1",
    variationIntensity: draft?.variationIntensity ?? DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
    productImageCount: draft?.productImageCount ?? DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
    productImagePrompt: draft?.productImagePrompt ?? "",
    productImagePrompts: draft?.productImagePrompts ?? [],
    artworkModel: draft?.artworkModel ?? DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
    transparentBackground: draft?.transparentBackground ?? false,
    sheinStoreId: draft?.sheinStoreId || DEFAULT_SHEIN_STORE_ID,
    imageStrategy: draft?.imageStrategy ?? DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
    groupedImageMode: draft?.groupedImageMode ?? DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
    selectedSdsImages: draft?.selectedSdsImages ?? [],
    groups: draft?.groups ?? [],
    groupedSelections: draft?.groupedSelections ?? [],
    renderSizeImagesWithSds: draft?.renderSizeImagesWithSds ?? true,
    designs,
    selectedIds,
    createdTasks: compatibility.createdTasks,
    generationJobs: compatibility.generationJobs,
  };
}
```

- [ ] **Step 4: Re-run the model tests and make them pass**

Run: `npm test -- --run web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.test.ts`
Expected: PASS with detail-first projection and fallback-only compatibility behavior.

- [ ] **Step 5: Commit the model merge cleanup**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.test.ts
git commit -m "refactor: centralize shein studio hydration precedence"
```

### Task 5: Narrow workbench draft persistence and page wiring

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-hooks.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-actions.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`

- [ ] **Step 1: Add a failing end-to-end workbench test for pre-hydration fallback and hydrated override**

```typescript
it("shows compatibility fallback first, then swaps to hydrated detail results", async () => {
  loadSheinStudioDraft.mockResolvedValue({
    prompt: "draft prompt",
    styleCount: "2",
    sheinStoreId: "42",
    groupedSelections: [],
    groups: [],
    compatibility: {
      designs: [{ id: "legacy-design", imageUrl: "https://example.com/legacy.png" }],
      selectedIds: ["legacy-design"],
      createdTasks: [],
    },
    updatedAt: "2026-06-04T00:00:00Z",
  });
  getSheinStudioHydratedBatch.mockResolvedValue({
    savedBatch: {
      id: "batch-1",
      name: "Saved Batch",
      prompt: "draft prompt",
      styleCount: "2",
      sheinStoreId: "42",
      groupedSelections: [],
      groups: [],
      compatibility: {
        designs: [{ id: "legacy-design", imageUrl: "https://example.com/legacy.png" }],
        selectedIds: ["legacy-design"],
      },
      updatedAt: "2026-06-04T00:00:00Z",
    },
    detail: buildHydratedBatchDetailFixture(),
  });

  render(<SheinStudioWorkbench />);

  expect(await screen.findByAltText(/legacy-design/i)).toBeInTheDocument();
  expect(await screen.findByAltText(/design-1/i)).toBeInTheDocument();
  expect(screen.queryByAltText(/legacy-design/i)).not.toBeInTheDocument();
});
```

- [ ] **Step 2: Run the workbench test and confirm it fails**

Run: `npm test -- --run web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`
Expected: FAIL because draft persistence and hydration still share flat result ownership.

- [ ] **Step 3: Narrow persistence contracts in hooks and state**

```typescript
type WorkbenchDraftState = {
  activeSelection?: SDSProductVariantSelection;
  artworkModel: SheinStudioArtworkModel;
  groups: SheinStudioGroupedWorkspaceView[];
  imageStrategy: SheinStudioImageStrategy;
  groupedImageMode: SheinStudioGroupedImageMode;
  isCreatingTasks: boolean;
  isGenerating: boolean;
  isLoadingWorkspace: boolean;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  prompt: string;
  regeneratingId: string;
  renderSizeImagesWithSds: boolean;
  groupedSelections: GroupedSDSSelectionEligibility[];
  persistedUpdatedAt: string;
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  setDraftWarning: (value: string | ((current: string) => string)) => void;
  setPersistedUpdatedAt: (value: string) => void;
  sheinStoreId: string;
  styleCount: string;
  transparentBackground: boolean;
  variationIntensity: SheinStudioVariationIntensity;
};

export type SheinStudioWorkbenchDraftPatch = Partial<Pick<
  SheinStudioWorkbenchState,
  | "prompt"
  | "selection"
  | "styleCount"
  | "variationIntensity"
  | "productImageCount"
  | "productImagePrompt"
  | "productImagePrompts"
  | "artworkModel"
  | "transparentBackground"
  | "sheinStoreId"
  | "imageStrategy"
  | "groupedImageMode"
  | "selectedSdsImages"
  | "groups"
  | "activeGroupId"
  | "groupedSelections"
  | "renderSizeImagesWithSds"
  | "persistedUpdatedAt"
  | "galleryRatioCheck"
>>;
```

- [ ] **Step 4: Route apply and hydration flows through the centralized model helpers**

```typescript
const restored = mergePersistedViewWithCompatibilityFallback({
  draft,
  galleryDesign,
  galleryPrompt,
});

dispatch({
  type: "restore-draft",
  draft: restored,
});

const hydrated = projectHydratedBatchToWorkbench({
  savedBatch,
  detail,
});

dispatch(applySheinStudioWorkbenchHydratedBatch({ savedBatch, detail }));
```

- [ ] **Step 5: Run focused tests plus typecheck**

Run: `npm test -- --run web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.test.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts web/listingkit-ui/src/lib/shein-studio/draft-input.test.ts`
Expected: PASS

Run: `npm run typecheck`
Expected: PASS

- [ ] **Step 6: Commit the workbench wiring cleanup**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-hooks.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-actions.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx
git commit -m "refactor: clean shein studio workbench persistence ownership"
```

### Task 6: Run the full regression slice and update the spec if needed

**Files:**
- Modify: `docs/superpowers/specs/2026-06-04-shein-studio-workbench-persistence-cleanup-design.md`
  Only if implementation discovers a real contract adjustment.

- [ ] **Step 1: Run the full Shein Studio regression slice**

Run: `npm test -- --run web/listingkit-ui/src/lib/shein-studio/draft-input.test.ts web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.test.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx`
Expected: PASS

- [ ] **Step 2: Run lint or typecheck if the touched files introduce new warnings**

Run: `npm run typecheck`
Expected: PASS

Run: `npm run lint -- web/listingkit-ui/src/components/listingkit/shein-studio web/listingkit-ui/src/lib/shein-studio web/listingkit-ui/src/lib/utils/shein-studio-batches.ts web/listingkit-ui/src/lib/types/shein-studio.ts`
Expected: PASS or only pre-existing unrelated warnings.

- [ ] **Step 3: Update the design spec only if the implementation uncovered a real contract change**

```markdown
## Implementation Follow-up

- grouped workspace restore continues to omit result-state ownership
- `generationJobs` remains fallback-only until a dedicated transient UI model is introduced
```

- [ ] **Step 4: Commit the verification pass**

```bash
git add docs/superpowers/specs/2026-06-04-shein-studio-workbench-persistence-cleanup-design.md
git commit -m "docs: note shein studio persistence cleanup follow-up"
```

## Self-Review Notes

- Spec coverage check:
  - ownership split is covered by Tasks 2, 3, and 4
  - grouped workspace cleanup is covered by Tasks 1, 2, and 3
  - hydration precedence is covered by Tasks 4 and 5
  - testing and recent batch regression coverage is covered by Tasks 5 and 6
- Placeholder scan:
  - no `TBD`, `TODO`, or deferred-only steps remain
  - each task includes concrete files, commands, and code snippets
- Type consistency:
  - the plan consistently uses `SheinStudioPersistedBatchView`, `SheinStudioLegacyCompatibilitySnapshot`, `SheinStudioGroupedWorkspaceView`, and `mergePersistedViewWithCompatibilityFallback`
